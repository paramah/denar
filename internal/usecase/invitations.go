package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/user"
)

// InvitationConfig holds the settings of invitations.
type InvitationConfig struct {
	// TTL is how long a new invitation works (DENAR_INVITE_TTL).
	TTL time.Duration
	// SessionTTL is the session length of a person who registers with an
	// invitation.
	SessionTTL time.Duration
	// BaseURL starts the invitation link sent by e-mail (stage 005).
	BaseURL string
}

// InvitationView is an invitation without its token hash, with its state.
type InvitationView struct {
	Invitation membership.Invitation
	State      membership.InvitationState
}

func invitationView(i membership.Invitation, now time.Time) InvitationView {
	i.TokenHash = ""
	return InvitationView{Invitation: i, State: i.State(now)}
}

// NewInvitation is a created invitation and its token in the clear, given
// out once to be put into the link.
type NewInvitation struct {
	InvitationView
	Token string
	// EmailSent: the link went out by e-mail; EmailError says why not
	// when sending was asked for and failed.
	EmailSent  bool
	EmailError string
}

type CreateInvitationInput struct {
	Email string
	Role  membership.Role
	// SendEmail asks to e-mail the link (FR-570); it needs SMTP.
	SendEmail bool
}

// CreateInvitation invites an email with a role; owners and admins only,
// and only an owner invites owners (FR-023).
type CreateInvitation struct {
	d   Deps
	cfg InvitationConfig
}

func NewCreateInvitation(d Deps, cfg InvitationConfig) *CreateInvitation {
	return &CreateInvitation{d: d, cfg: cfg}
}

func (uc *CreateInvitation) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateInvitationInput) (NewInvitation, error) {
	if err := requireHuman(p); err != nil {
		return NewInvitation{}, err
	}
	email := user.NormalizeEmail(in.Email)
	if err := errors.Join(user.ValidateEmail("email", email), membership.ValidateRole("role", in.Role)); err != nil {
		return NewInvitation{}, err
	}
	if in.SendEmail && uc.d.Mailer == nil {
		return NewInvitation{}, domain.ConflictError{Reason: domain.ConflictSMTPNotConfigured}
	}
	var householdName, inviter string
	token, err := uc.d.Tokens.NewToken()
	if err != nil {
		return NewInvitation{}, fmt.Errorf("make the invitation token: %w", err)
	}
	now := uc.d.Clock.Now()
	inv := membership.Invitation{
		ID: uc.d.IDs.NewID(), HouseholdID: householdID, Email: email, Role: in.Role,
		TokenHash: hashToken(token), CreatedBy: p.User.ID, CreatedAt: now, ExpiresAt: now.Add(uc.cfg.TTL),
	}
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleAdmin); err != nil {
			return err
		}
		if err := membership.CheckInvite(s.Role, in.Role); err != nil {
			return err
		}
		if err := uc.d.Invitations.Create(ctx, inv); err != nil {
			return err
		}
		householdName, inviter = s.Household.Name, cmpOr(p.User.Name, p.User.Email)
		return uc.d.record(ctx, householdID, p.Actor(), "invitation.created", audit.ResourceInvitation, inv.ID,
			audit.Changes{}.Set("email", email).Set("role", string(in.Role)).Set("expires_at", inv.ExpiresAt.Format(time.RFC3339)))
	})
	if err != nil {
		return NewInvitation{}, err
	}
	out := NewInvitation{InvitationView: invitationView(inv, now), Token: token}
	if in.SendEmail {
		// After the commit: a failure leaves the invitation, whose link the
		// answer still carries (AD-513).
		link := uc.cfg.BaseURL + "/invite#" + token
		body := inviter + " zaprasza Cię do gospodarstwa „" + householdName + "” w Denarze.\n\n" +
			"Otwórz link, aby dołączyć (ważny do " + inv.ExpiresAt.Format("02.01.2006 15:04") + " UTC):\n" + link + "\n"
		if err := uc.d.Mailer.SendMail(ctx, email, "Zaproszenie do gospodarstwa "+householdName, body); err != nil {
			uc.d.log().WarnContext(ctx, "sending the invitation e-mail failed", "invitation_id", string(inv.ID), "error", err.Error())
			out.EmailError = errorText(err)
		} else {
			out.EmailSent = true
		}
	}
	return out, nil
}

// ListInvitations returns the invitations of a household, newest first;
// owners and admins only.
type ListInvitations struct{ d Deps }

func NewListInvitations(d Deps) *ListInvitations { return &ListInvitations{d: d} }

func (uc *ListInvitations) Do(ctx context.Context, p Principal, householdID domain.ID) ([]InvitationView, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return nil, err
	}
	if err := requireRole(s, membership.RoleAdmin); err != nil {
		return nil, err
	}
	all, err := uc.d.Invitations.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	now := uc.d.Clock.Now()
	out := make([]InvitationView, len(all))
	for i, inv := range all {
		out[i] = invitationView(inv, now)
	}
	return out, nil
}

// RevokeInvitation withdraws an invitation that has not been accepted;
// owners and admins only. Revoking twice is fine.
type RevokeInvitation struct{ d Deps }

func NewRevokeInvitation(d Deps) *RevokeInvitation { return &RevokeInvitation{d: d} }

func (uc *RevokeInvitation) Do(ctx context.Context, p Principal, householdID, invitationID domain.ID) error {
	if err := requireHuman(p); err != nil {
		return err
	}
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleAdmin); err != nil {
			return err
		}
		inv, err := uc.d.Invitations.ByID(ctx, invitationID)
		if err != nil || inv.HouseholdID != householdID {
			return domain.ErrNotFound
		}
		switch inv.State(uc.d.Clock.Now()) {
		case membership.InvitationAccepted:
			return domain.ConflictError{Reason: domain.ConflictInvitationUsed}
		case membership.InvitationRevoked:
			return nil
		}
		inv.RevokedAt = uc.d.Clock.Now()
		if err := uc.d.Invitations.Update(ctx, inv); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "invitation.revoked", audit.ResourceInvitation, inv.ID, nil)
	})
}

// InvitationPreview is what the holder of a link may see before accepting.
type InvitationPreview struct {
	HouseholdName string
	Email         string
	Role          membership.Role
	ExpiresAt     time.Time
	// AccountExists tells whether the invited email has an account, so
	// the page offers to sign in rather than to register.
	AccountExists bool
}

// PreviewInvitation shows a usable invitation to whoever holds its token.
// Unknown and revoked tokens give domain.ErrNotFound; used and expired
// ones a conflict.
type PreviewInvitation struct{ d Deps }

func NewPreviewInvitation(d Deps) *PreviewInvitation { return &PreviewInvitation{d: d} }

func (uc *PreviewInvitation) Do(ctx context.Context, token string) (InvitationPreview, error) {
	inv, h, err := usableInvitation(ctx, uc.d, token)
	if err != nil {
		return InvitationPreview{}, err
	}
	_, err = uc.d.Users.ByEmail(ctx, inv.Email)
	exists, err := ignoreNotFound(err)
	if err != nil {
		return InvitationPreview{}, err
	}
	return InvitationPreview{HouseholdName: h.Name, Email: inv.Email, Role: inv.Role, ExpiresAt: inv.ExpiresAt, AccountExists: exists}, nil
}

// usableInvitation finds the invitation of a token and its household.
func usableInvitation(ctx context.Context, d Deps, token string) (membership.Invitation, household.Household, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return membership.Invitation{}, household.Household{}, domain.ErrNotFound
	}
	inv, err := d.Invitations.ByTokenHash(ctx, hashToken(token))
	if err != nil {
		return membership.Invitation{}, household.Household{}, plainNotFound(err)
	}
	if err := inv.CheckUsable(d.Clock.Now()); err != nil {
		return membership.Invitation{}, household.Household{}, err
	}
	h, err := d.Households.ByID(ctx, inv.HouseholdID)
	if err != nil {
		return membership.Invitation{}, household.Household{}, plainNotFound(err)
	}
	return inv, h, nil
}

// accept makes the user a member with the invitation's role and uses the
// invitation up. Call it in a transaction.
func accept(ctx context.Context, d Deps, inv membership.Invitation, u user.User) error {
	_, err := d.Memberships.Get(ctx, inv.HouseholdID, u.ID)
	if member, err := ignoreNotFound(err); err != nil {
		return err
	} else if member {
		return domain.ConflictError{Reason: domain.ConflictAlreadyMember}
	}
	now := d.Clock.Now()
	if err := d.Memberships.Create(ctx, membership.Membership{HouseholdID: inv.HouseholdID, UserID: u.ID, Role: inv.Role, CreatedAt: now}); err != nil {
		return err
	}
	inv.AcceptedAt, inv.AcceptedBy = now, u.ID
	if err := d.Invitations.Update(ctx, inv); err != nil {
		return err
	}
	actor := domain.Actor{Kind: domain.ActorUser, ID: u.ID}
	return d.record(ctx, inv.HouseholdID, actor, "membership.joined", audit.ResourceMembership, u.ID,
		audit.Changes{}.Set("role", string(inv.Role)).Set("invitation_id", string(inv.ID)))
}

// AcceptInvitation lets a signed-in person accept an invitation. The
// email of the account need not match the invited one: the one-time link
// is the secret (decision AD-006).
type AcceptInvitation struct{ d Deps }

func NewAcceptInvitation(d Deps) *AcceptInvitation { return &AcceptInvitation{d: d} }

func (uc *AcceptInvitation) Do(ctx context.Context, p Principal, token string) (HouseholdView, error) {
	if err := requireHuman(p); err != nil {
		return HouseholdView{}, err
	}
	var view HouseholdView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		inv, h, err := usableInvitation(ctx, uc.d, token)
		if err != nil {
			return err
		}
		if err := accept(ctx, uc.d, inv, p.User); err != nil {
			return err
		}
		view = HouseholdView{Household: h, Role: inv.Role}
		return nil
	})
	return view, err
}

type RegisterWithInvitationInput struct {
	Token    string
	Name     string
	Password string
}

// Registered is a new account, its first household and its session.
type Registered struct {
	Login
	Household HouseholdView
}

// RegisterWithInvitation creates a local account for the invited email,
// accepts the invitation and signs the person in. An email that has an
// account already gives a conflict: that person signs in and accepts.
type RegisterWithInvitation struct {
	d        Deps
	sessions sessionStarter
}

func NewRegisterWithInvitation(d Deps, cfg InvitationConfig) *RegisterWithInvitation {
	return &RegisterWithInvitation{d: d, sessions: sessionStarter{sessions: d.Sessions, tokens: d.Tokens, clock: d.Clock, ttl: cfg.SessionTTL}}
}

func (uc *RegisterWithInvitation) Do(ctx context.Context, in RegisterWithInvitationInput) (Registered, error) {
	name := strings.TrimSpace(in.Name)
	if err := errors.Join(user.ValidateName("name", name), user.ValidatePassword("password", in.Password)); err != nil {
		return Registered{}, err
	}
	// Check the invitation before the slow hash, so a wrong token costs
	// nothing; and again inside the transaction.
	if _, _, err := usableInvitation(ctx, uc.d, in.Token); err != nil {
		return Registered{}, err
	}
	hash, err := uc.d.Hasher.Hash(in.Password)
	if err != nil {
		return Registered{}, fmt.Errorf("hash the password: %w", err)
	}
	var out Registered
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		inv, h, err := usableInvitation(ctx, uc.d, in.Token)
		if err != nil {
			return err
		}
		u := user.User{
			ID: uc.d.IDs.NewID(), Email: inv.Email, Name: name, PasswordHash: hash, Active: true, CreatedAt: uc.d.Clock.Now(),
		}
		if err := uc.d.Users.Create(ctx, u); err != nil {
			return err
		}
		if err := accept(ctx, uc.d, inv, u); err != nil {
			return err
		}
		login, err := uc.sessions.start(ctx, u, "")
		if err != nil {
			return err
		}
		out = Registered{Login: login, Household: HouseholdView{Household: h, Role: inv.Role}}
		return nil
	})
	return out, err
}
