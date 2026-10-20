package usecase

import (
	"context"
	"errors"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

// HouseholdScope is what a principal may do in one household, as the
// store has it at the moment of the request.
type HouseholdScope struct {
	Household household.Household
	// Role is the person's own role; for a bot, the current role of its
	// creator, "" when the creator is no longer a member.
	Role membership.Role
	// Bot is set when the principal is a bot.
	Bot *bot.Bot
	// Scopes are the bot's effective scopes: its scopes the creator's role
	// permits. Nil for a person.
	Scopes []bot.Scope
	// WalletIDs limits a bot to these wallets; empty means every wallet
	// the creator sees. Nil for a person.
	WalletIDs []domain.ID
}

// IsBot reports whether the scope is a bot's.
func (s HouseholdScope) IsBot() bool { return s.Bot != nil }

// ActorUserID is the person whose permissions apply: the user, or the
// creator of the bot.
func (s HouseholdScope) ActorUserID(p Principal) domain.ID {
	if s.IsBot() {
		return s.Bot.CreatedBy
	}
	return p.User.ID
}

// WalletAllowed reports whether the bot's wallet list lets it reach the
// wallet. It does not check that the creator sees the wallet: stage 002
// filters by the creator's visible wallets first and asks this second.
// A person is never limited here.
func (s HouseholdScope) WalletAllowed(walletID domain.ID) bool {
	if !s.IsBot() || len(s.WalletIDs) == 0 {
		return true
	}
	return slices.Contains(s.WalletIDs, walletID)
}

// access decides which households a principal sees and with what rights.
// A principal who cannot see a household gets domain.ErrNotFound, exactly
// as if it did not exist.
type access struct {
	households  HouseholdRepository
	memberships MembershipRepository
}

// household returns the scope of the principal in the household. A person
// needs a membership; a bot sees its own household only. The role of a
// bot's creator is read here on every call, so a creator who loses their
// role or membership narrows the bot at once (FR-032).
func (a access) household(ctx context.Context, p Principal, householdID domain.ID) (HouseholdScope, error) {
	if p.IsBot() && p.Bot.HouseholdID != householdID {
		return HouseholdScope{}, domain.ErrNotFound
	}
	h, err := a.households.ByID(ctx, householdID)
	if err != nil {
		return HouseholdScope{}, plainNotFound(err)
	}
	if !p.IsBot() {
		m, err := a.memberships.Get(ctx, householdID, p.User.ID)
		if err != nil {
			return HouseholdScope{}, plainNotFound(err)
		}
		return HouseholdScope{Household: h, Role: m.Role}, nil
	}
	b := p.Bot
	scope := HouseholdScope{Household: h, Bot: &b, WalletIDs: slices.Clone(b.WalletIDs)}
	m, err := a.memberships.Get(ctx, householdID, b.CreatedBy)
	if found, err := ignoreNotFound(err); err != nil {
		return HouseholdScope{}, err
	} else if found {
		scope.Role = m.Role
	}
	scope.Scopes = b.EffectiveScopes(scope.Role)
	return scope, nil
}

// plainNotFound strips whatever a repository wrapped around
// domain.ErrNotFound, so that a missing and a hidden resource give the
// very same error.
func plainNotFound(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func plainInvalid(err error) bool { return errors.Is(err, domain.ErrInvalid) }

// ignoreNotFound turns domain.ErrNotFound into found == false.
func ignoreNotFound(err error) (found bool, _ error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	return false, err
}

var errHumanOnly = domain.ForbiddenError{Reason: domain.ForbiddenHumanOnly}

// requireHuman refuses bots. Every administrative use case (members,
// invitations, bots, settings, audit) starts with it, before it looks at
// anything (FR-033).
func requireHuman(p Principal) error {
	if p.IsBot() {
		return errHumanOnly
	}
	return nil
}

// requireRole refuses a scope whose role is below min. Bots never pass:
// a role check guards administration, which bots may not do.
func requireRole(s HouseholdScope, min membership.Role) error {
	if s.IsBot() {
		return errHumanOnly
	}
	if !s.Role.AtLeast(min) {
		return domain.ForbiddenError{Reason: domain.ForbiddenRole}
	}
	return nil
}

// requireScope checks the permission for an operation on household data.
// A person needs a role that permits the scope (a viewer only reads). A bot
// needs the scope among its effective scopes, which already leaves out what
// its creator's current role does not permit.
func requireScope(s HouseholdScope, scope bot.Scope) error {
	if s.IsBot() {
		if !slices.Contains(s.Scopes, scope) {
			return domain.ForbiddenError{Reason: domain.ForbiddenScope}
		}
		return nil
	}
	if !bot.RolePermits(s.Role, scope) {
		return domain.ForbiddenError{Reason: domain.ForbiddenRole}
	}
	return nil
}
