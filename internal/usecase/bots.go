package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/user"
)

// MaxBotsPerHousehold bounds the bots of one household.
const MaxBotsPerHousehold = 100

// botTokenPrefixLength is how much of a token is kept in the clear for
// people to recognise it: the prefix and four characters of the secret.
const botTokenPrefixLength = len(BotTokenPrefix) + 4

// BotView is a bot as people see it: without its token hash, with what it
// may do now.
type BotView struct {
	Bot bot.Bot
	// CreatorRole is the current role of the creator; "" when the creator
	// is no longer a member.
	CreatorRole membership.Role
	// EffectiveScopes are the scopes the creator's role permits (FR-032).
	EffectiveScopes []bot.Scope
}

func botView(b bot.Bot, creatorRole membership.Role) BotView {
	b.TokenHash = ""
	return BotView{Bot: b, CreatorRole: creatorRole, EffectiveScopes: b.EffectiveScopes(creatorRole)}
}

// BotWithToken is a bot and its token in the clear, given out once.
type BotWithToken struct {
	BotView
	Token string
}

// newBotToken makes a token: dnr_ and 32 random bytes in base64url (FR-030).
func newBotToken(tokens TokenGenerator) (token, hash, prefix string, err error) {
	secret, err := tokens.NewToken()
	if err != nil {
		return "", "", "", err
	}
	token = BotTokenPrefix + secret
	return token, hashToken(token), token[:min(len(token), botTokenPrefixLength)], nil
}

// mayManageBot reports whether a person with the scope may change the bot:
// owners and admins every bot, others the ones they created (D-010).
func mayManageBot(s HouseholdScope, p Principal, b bot.Bot) error {
	if s.Role.CanManage() || b.CreatedBy == p.User.ID {
		return nil
	}
	return domain.ForbiddenError{Reason: domain.ForbiddenRole}
}

type CreateBotInput struct {
	Name      string
	Scopes    []string
	WalletIDs []string
	ExpiresAt time.Time // zero for a token that does not expire
}

// checkBotInput validates the input of a new or changed bot.
func checkBotInput(ctx context.Context, d Deps, householdID, creatorID domain.ID, name string, scopes, wallets []string,
	expiresAt time.Time) (bot.Bot, error) {
	b := bot.Bot{Name: strings.TrimSpace(name), ExpiresAt: expiresAt.UTC().Truncate(time.Microsecond)}
	var errs []error
	if err := b.Validate(d.Clock.Now()); err != nil {
		errs = append(errs, err)
	}
	var err error
	if b.Scopes, err = bot.ParseScopes("scopes", scopes); err != nil {
		errs = append(errs, err)
	}
	if b.WalletIDs, err = bot.ParseWalletIDs("wallet_ids", wallets); err != nil {
		errs = append(errs, err)
	} else if d.Wallets != nil && len(b.WalletIDs) > 0 {
		if err := d.Wallets.ValidateWalletIDs(ctx, householdID, creatorID, b.WalletIDs); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return bot.Bot{}, errors.Join(errs...)
	}
	return b, nil
}

// insertBot stores a new bot with a fresh token and records it. Call it in
// a transaction.
func insertBot(ctx context.Context, d Deps, b bot.Bot, actor domain.Actor) (string, error) {
	existing, err := d.Bots.ListByHousehold(ctx, b.HouseholdID)
	if err != nil {
		return "", err
	}
	if len(existing) >= MaxBotsPerHousehold {
		return "", domain.ValidationError{Field: "name", Message: "the household already has the 100 bots it may have"}
	}
	token, hash, prefix, err := newBotToken(d.Tokens)
	if err != nil {
		return "", fmt.Errorf("make the bot token: %w", err)
	}
	b.TokenHash, b.TokenPrefix = hash, prefix
	if err := d.Bots.Create(ctx, b); err != nil {
		return "", err
	}
	changes := audit.Changes{}.Set("name", b.Name).Set("scopes", joinScopes(b.Scopes)).
		Set("wallet_ids", joinIDs(b.WalletIDs)).Set("created_by", string(b.CreatedBy))
	if !b.ExpiresAt.IsZero() {
		changes.Set("expires_at", b.ExpiresAt.Format(time.RFC3339))
	}
	if err := d.record(ctx, b.HouseholdID, actor, "bot.created", audit.ResourceBot, b.ID, changes); err != nil {
		return "", err
	}
	return token, nil
}

func joinScopes(scopes []bot.Scope) string {
	s := make([]string, len(scopes))
	for i, sc := range scopes {
		s[i] = string(sc)
	}
	return strings.Join(s, " ")
}

func joinIDs(ids []domain.ID) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = string(id)
	}
	return strings.Join(s, " ")
}

func formatExpiry(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// CreateBot adds a bot for the signed-in member, who becomes its creator.
// Owners, admins and members may; viewers and bots may not (D-010, FR-033).
// The token is returned in the clear this once.
type CreateBot struct{ d Deps }

func NewCreateBot(d Deps) *CreateBot { return &CreateBot{d: d} }

func (uc *CreateBot) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateBotInput) (BotWithToken, error) {
	if err := requireHuman(p); err != nil {
		return BotWithToken{}, err
	}
	var out BotWithToken
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleMember); err != nil {
			return err
		}
		b, err := checkBotInput(ctx, uc.d, householdID, p.User.ID, in.Name, in.Scopes, in.WalletIDs, in.ExpiresAt)
		if err != nil {
			return err
		}
		b.ID, b.HouseholdID, b.CreatedBy, b.CreatedAt = uc.d.IDs.NewID(), householdID, p.User.ID, uc.d.Clock.Now()
		token, err := insertBot(ctx, uc.d, b, p.Actor())
		if err != nil {
			return err
		}
		stored, err := uc.d.Bots.ByID(ctx, b.ID)
		if err != nil {
			return err
		}
		out = BotWithToken{BotView: botView(stored, s.Role), Token: token}
		return nil
	})
	return out, err
}

// ListBots returns the bots of a household: every bot to owners and
// admins, their own to the others.
type ListBots struct{ d Deps }

func NewListBots(d Deps) *ListBots { return &ListBots{d: d} }

func (uc *ListBots) Do(ctx context.Context, p Principal, householdID domain.ID) ([]BotView, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return nil, err
	}
	all, err := uc.d.Bots.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	members, err := uc.d.Memberships.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := []BotView{}
	for _, b := range all {
		if mayManageBot(s, p, b) != nil {
			continue
		}
		m, _ := findMember(members, b.CreatedBy)
		out = append(out, botView(b, m.Role))
	}
	return out, nil
}

// manageableBot finds a bot of the household the person may manage. A bot
// of another household is not found; one the person may not manage is
// forbidden.
func manageableBot(ctx context.Context, d Deps, p Principal, householdID, botID domain.ID) (HouseholdScope, bot.Bot, membership.Role, error) {
	s, err := d.access().household(ctx, p, householdID)
	if err != nil {
		return HouseholdScope{}, bot.Bot{}, "", err
	}
	b, err := d.Bots.ByID(ctx, botID)
	if err != nil || b.HouseholdID != householdID {
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return HouseholdScope{}, bot.Bot{}, "", err
		}
		return HouseholdScope{}, bot.Bot{}, "", domain.ErrNotFound
	}
	if err := mayManageBot(s, p, b); err != nil {
		return HouseholdScope{}, bot.Bot{}, "", err
	}
	creator, err := d.Memberships.Get(ctx, householdID, b.CreatedBy)
	if _, err := ignoreNotFound(err); err != nil {
		return HouseholdScope{}, bot.Bot{}, "", err
	}
	return s, b, creator.Role, nil
}

// GetBot returns one bot to a person who may manage it.
type GetBot struct{ d Deps }

func NewGetBot(d Deps) *GetBot { return &GetBot{d: d} }

func (uc *GetBot) Do(ctx context.Context, p Principal, householdID, botID domain.ID) (BotView, error) {
	if err := requireHuman(p); err != nil {
		return BotView{}, err
	}
	_, b, role, err := manageableBot(ctx, uc.d, p, householdID, botID)
	if err != nil {
		return BotView{}, err
	}
	return botView(b, role), nil
}

type UpdateBotInput struct {
	Name      domain.Optional[string]
	Scopes    domain.Optional[[]string]
	WalletIDs domain.Optional[[]string]
	// ExpiresAt set to the zero time removes the expiry.
	ExpiresAt domain.Optional[time.Time]
	Disabled  domain.Optional[bool]
}

// UpdateBot changes a bot; the same people who may see it.
type UpdateBot struct{ d Deps }

func NewUpdateBot(d Deps) *UpdateBot { return &UpdateBot{d: d} }

func (uc *UpdateBot) Do(ctx context.Context, p Principal, householdID, botID domain.ID, in UpdateBotInput) (BotView, error) {
	if err := requireHuman(p); err != nil {
		return BotView{}, err
	}
	var view BotView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		_, old, role, err := manageableBot(ctx, uc.d, p, householdID, botID)
		if err != nil {
			return err
		}
		name, scopes, wallets, expires := old.Name, scopeStrings(old.Scopes), idStrings(old.WalletIDs), old.ExpiresAt
		if in.Name.Set {
			name = in.Name.Value
		}
		if in.Scopes.Set {
			scopes = in.Scopes.Value
		}
		if in.WalletIDs.Set {
			wallets = in.WalletIDs.Value
		}
		if in.ExpiresAt.Set {
			expires = in.ExpiresAt.Value
		}
		// An unchanged expiry in the past does not block other changes.
		checkExpiry := expires
		if !in.ExpiresAt.Set {
			checkExpiry = time.Time{}
		}
		checked, err := checkBotInput(ctx, uc.d, householdID, old.CreatedBy, name, scopes, wallets, checkExpiry)
		if err != nil {
			return err
		}
		b := old
		b.Name, b.Scopes, b.WalletIDs = checked.Name, checked.Scopes, checked.WalletIDs
		if in.ExpiresAt.Set {
			b.ExpiresAt = checked.ExpiresAt
		}
		if in.Disabled.Set {
			b.Disabled = in.Disabled.Value
		}
		changes := audit.Changes{}.Diff("name", old.Name, b.Name).
			Diff("scopes", joinScopes(old.Scopes), joinScopes(b.Scopes)).
			Diff("wallet_ids", joinIDs(old.WalletIDs), joinIDs(b.WalletIDs)).
			Diff("expires_at", formatExpiry(old.ExpiresAt), formatExpiry(b.ExpiresAt)).
			Diff("disabled", old.Disabled, b.Disabled)
		if len(changes) > 0 {
			if err := uc.d.Bots.Update(ctx, b); err != nil {
				return err
			}
			if err := uc.d.record(ctx, householdID, p.Actor(), "bot.updated", audit.ResourceBot, b.ID, changes); err != nil {
				return err
			}
		}
		view = botView(b, role)
		return nil
	})
	return view, err
}

func scopeStrings(scopes []bot.Scope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

func idStrings(ids []domain.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// RotateBotToken replaces the token of a bot; the old one stops working at
// once (US-005). The new token is returned in the clear this once.
type RotateBotToken struct{ d Deps }

func NewRotateBotToken(d Deps) *RotateBotToken { return &RotateBotToken{d: d} }

func (uc *RotateBotToken) Do(ctx context.Context, p Principal, householdID, botID domain.ID) (BotWithToken, error) {
	if err := requireHuman(p); err != nil {
		return BotWithToken{}, err
	}
	var out BotWithToken
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		_, b, role, err := manageableBot(ctx, uc.d, p, householdID, botID)
		if err != nil {
			return err
		}
		token, hash, prefix, err := newBotToken(uc.d.Tokens)
		if err != nil {
			return fmt.Errorf("make the bot token: %w", err)
		}
		oldPrefix := b.TokenPrefix
		b.TokenHash, b.TokenPrefix = hash, prefix
		if err := uc.d.Bots.Update(ctx, b); err != nil {
			return err
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "bot.token_rotated", audit.ResourceBot, b.ID,
			audit.Changes{}.Diff("token_prefix", oldPrefix, prefix)); err != nil {
			return err
		}
		out = BotWithToken{BotView: botView(b, role), Token: token}
		return nil
	})
	return out, err
}

// DeleteBot removes a bot; its token stops working at once.
type DeleteBot struct{ d Deps }

func NewDeleteBot(d Deps) *DeleteBot { return &DeleteBot{d: d} }

func (uc *DeleteBot) Do(ctx context.Context, p Principal, householdID, botID domain.ID) error {
	if err := requireHuman(p); err != nil {
		return err
	}
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		_, b, _, err := manageableBot(ctx, uc.d, p, householdID, botID)
		if err != nil {
			return err
		}
		if err := uc.d.Bots.Delete(ctx, b.ID); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "bot.deleted", audit.ResourceBot, b.ID,
			audit.Changes{}.Diff("name", b.Name, nil))
	})
}

type CreateBotCLIInput struct {
	HouseholdID domain.ID
	// CreatorEmail names the member the bot acts for; empty means the
	// household's oldest owner.
	CreatorEmail string
	CreateBotInput
}

// CreateBotCLI creates a bot from the command line of the instance
// administrator (FR-036), with the system as the actor. The creator's role
// limits the bot as for any other.
type CreateBotCLI struct{ d Deps }

func NewCreateBotCLI(d Deps) *CreateBotCLI { return &CreateBotCLI{d: d} }

func (uc *CreateBotCLI) Do(ctx context.Context, in CreateBotCLIInput) (BotWithToken, error) {
	var out BotWithToken
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.Households.ByID(ctx, in.HouseholdID); err != nil {
			return fmt.Errorf("household %s: %w", in.HouseholdID, plainNotFound(err))
		}
		members, err := uc.d.Memberships.ListByHousehold(ctx, in.HouseholdID)
		if err != nil {
			return err
		}
		var creator membership.Membership
		if in.CreatorEmail != "" {
			u, err := uc.d.Users.ByEmail(ctx, user.NormalizeEmail(in.CreatorEmail))
			if err != nil {
				return fmt.Errorf("user %s: %w", in.CreatorEmail, plainNotFound(err))
			}
			m, ok := findMember(members, u.ID)
			if !ok {
				return domain.ValidationError{Field: "as", Message: "must be a member of the household"}
			}
			creator = m
		} else {
			i := slices.IndexFunc(members, func(m membership.Membership) bool { return m.Role == membership.RoleOwner })
			if i < 0 {
				return domain.ValidationError{Field: "as", Message: "the household has no owner to create the bot for"}
			}
			creator = members[i]
		}
		if !creator.Role.AtLeast(membership.RoleMember) {
			return domain.ValidationError{Field: "as", Message: "a viewer cannot create bots"}
		}
		b, err := checkBotInput(ctx, uc.d, in.HouseholdID, creator.UserID, in.Name, in.Scopes, in.WalletIDs, in.ExpiresAt)
		if err != nil {
			return err
		}
		b.ID, b.HouseholdID, b.CreatedBy, b.CreatedAt = uc.d.IDs.NewID(), in.HouseholdID, creator.UserID, uc.d.Clock.Now()
		token, err := insertBot(ctx, uc.d, b, SystemActor)
		if err != nil {
			return err
		}
		stored, err := uc.d.Bots.ByID(ctx, b.ID)
		if err != nil {
			return err
		}
		out = BotWithToken{BotView: botView(stored, creator.Role), Token: token}
		return nil
	})
	return out, err
}
