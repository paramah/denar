package usecase_test

import (
	"errors"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var errAudit = errors.New("audit store failed")

func (f *family) invite(by membership.Role, email string, role membership.Role) (usecase.NewInvitation, error) {
	return usecase.NewCreateInvitation(f.deps, usecase.InvitationConfig{TTL: 7 * 24 * time.Hour}).
		Do(ctx, human(f.byRole(by)), "h1", usecase.CreateInvitationInput{Email: email, Role: role})
}

func TestCreateInvitation(t *testing.T) {
	f := newFamily(t)
	inv, err := f.invite(membership.RoleAdmin, " Grace@Example.com ", membership.RoleMember)
	noErr(t, err)
	if inv.Token == "" || inv.Invitation.Email != "grace@example.com" || inv.Invitation.TokenHash != "" ||
		inv.State != membership.InvitationPending || !inv.Invitation.ExpiresAt.Equal(f.Clock.Now().Add(7*24*time.Hour)) {
		t.Errorf("invitation = %+v", inv)
	}
	stored, err := f.Invitations.ByTokenHash(ctx, usecase.HashToken(inv.Token))
	noErr(t, err)
	if stored.ID != inv.Invitation.ID {
		t.Error("stored by another hash")
	}
	if a := f.actions(); len(a) != 1 || a[0] != "invitation.created" {
		t.Errorf("audit = %v", a)
	}

	_, err = f.invite(membership.RoleAdmin, "x@example.com", membership.RoleOwner)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = f.invite(membership.RoleOwner, "x@example.com", membership.RoleOwner)
	noErr(t, err)
	_, err = f.invite(membership.RoleMember, "x@example.com", membership.RoleViewer)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = f.invite(membership.RoleOwner, "not an email", "chief")
	wantErr(t, err, domain.ErrInvalid)

	list, err := usecase.NewListInvitations(f.deps).Do(ctx, human(f.admin), "h1")
	noErr(t, err)
	if len(list) != 2 || list[0].Invitation.TokenHash != "" {
		t.Errorf("list = %+v", list)
	}
	_, err = usecase.NewListInvitations(f.deps).Do(ctx, human(f.member), "h1")
	wantForbidden(t, err, domain.ForbiddenRole)
	b := f.addBot("b1", "h1", f.owner, bot.Scopes...)
	_, err = usecase.NewCreateInvitation(f.deps, usecase.InvitationConfig{TTL: time.Hour}).
		Do(ctx, asBot(b), "h1", usecase.CreateInvitationInput{Email: "x@example.com", Role: membership.RoleViewer})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
}

func TestAcceptInvitation(t *testing.T) {
	f := newFamily(t)
	inv, err := f.invite(membership.RoleOwner, "outsider@example.com", membership.RoleViewer)
	noErr(t, err)

	preview, err := usecase.NewPreviewInvitation(f.deps).Do(ctx, inv.Token)
	noErr(t, err)
	if preview.HouseholdName != "Dom h1" || preview.Role != membership.RoleViewer || !preview.AccountExists {
		t.Errorf("preview = %+v", preview)
	}

	got, err := usecase.NewAcceptInvitation(f.deps).Do(ctx, human(f.outsider), inv.Token)
	noErr(t, err)
	if got.Household.ID != "h1" || got.Role != membership.RoleViewer {
		t.Errorf("accepted = %+v", got)
	}
	m, err := f.Memberships.Get(ctx, "h1", "outsider")
	noErr(t, err)
	if m.Role != membership.RoleViewer {
		t.Errorf("role = %s", m.Role)
	}
	// The link works once.
	_, err = usecase.NewAcceptInvitation(f.deps).Do(ctx, human(f.member), inv.Token)
	wantConflict(t, err, domain.ConflictInvitationUsed)
	_, err = usecase.NewPreviewInvitation(f.deps).Do(ctx, inv.Token)
	wantConflict(t, err, domain.ConflictInvitationUsed)

	again, err := f.invite(membership.RoleOwner, "member@example.com", membership.RoleViewer)
	noErr(t, err)
	_, err = usecase.NewAcceptInvitation(f.deps).Do(ctx, human(f.member), again.Token)
	wantConflict(t, err, domain.ConflictAlreadyMember)

	_, err = usecase.NewAcceptInvitation(f.deps).Do(ctx, human(f.member), "unknown")
	wantErr(t, err, domain.ErrNotFound)
	_, err = usecase.NewPreviewInvitation(f.deps).Do(ctx, "")
	wantErr(t, err, domain.ErrNotFound)
}

func TestInvitationExpiryAndRevocation(t *testing.T) {
	f := newFamily(t)
	expiring, err := f.invite(membership.RoleOwner, "a@example.com", membership.RoleMember)
	noErr(t, err)
	revoked, err := f.invite(membership.RoleOwner, "b@example.com", membership.RoleMember)
	noErr(t, err)

	revoke := usecase.NewRevokeInvitation(f.deps)
	wantForbidden(t, revoke.Do(ctx, human(f.member), "h1", revoked.Invitation.ID), domain.ForbiddenRole)
	noErr(t, revoke.Do(ctx, human(f.admin), "h1", revoked.Invitation.ID))
	noErr(t, revoke.Do(ctx, human(f.admin), "h1", revoked.Invitation.ID))
	wantErr(t, revoke.Do(ctx, human(f.outsider), "h2", revoked.Invitation.ID), domain.ErrNotFound)
	_, err = usecase.NewPreviewInvitation(f.deps).Do(ctx, revoked.Token)
	wantErr(t, err, domain.ErrNotFound)

	f.Clock.Advance(8 * 24 * time.Hour)
	_, err = usecase.NewAcceptInvitation(f.deps).Do(ctx, human(f.outsider), expiring.Token)
	wantConflict(t, err, domain.ConflictInvitationExpired)
}

func TestRegisterWithInvitation(t *testing.T) {
	f := newFamily(t)
	cfg := usecase.InvitationConfig{TTL: time.Hour, SessionTTL: ttl}
	inv, err := f.invite(membership.RoleOwner, "grace@example.com", membership.RoleMember)
	noErr(t, err)
	reg := usecase.NewRegisterWithInvitation(f.deps, cfg)

	_, err = reg.Do(ctx, usecase.RegisterWithInvitationInput{Token: inv.Token, Name: "", Password: "short"})
	wantErr(t, err, domain.ErrInvalid)
	_, err = reg.Do(ctx, usecase.RegisterWithInvitationInput{Token: "nope", Name: "Grace", Password: "a long password!"})
	wantErr(t, err, domain.ErrNotFound)

	got, err := reg.Do(ctx, usecase.RegisterWithInvitationInput{Token: inv.Token, Name: " Grace ", Password: "a long password!"})
	noErr(t, err)
	if got.User.Email != "grace@example.com" || got.User.Name != "Grace" || got.Token == "" || got.Household.Role != membership.RoleMember {
		t.Errorf("registered = %+v", got)
	}
	u, err := f.Users.ByEmail(ctx, "grace@example.com")
	noErr(t, err)
	if !f.Hasher.Verify("a long password!", u.PasswordHash) || u.InstanceAdmin {
		t.Errorf("user = %+v", u)
	}
	if _, err := f.Sessions.ByTokenHash(ctx, usecase.HashToken(got.Token)); err != nil {
		t.Error("no session")
	}
	a := f.actions()
	if a[len(a)-1] != "membership.joined" || f.Audit.All()[len(a)-1].Actor.ID != u.ID {
		t.Errorf("audit = %v", a)
	}

	// An email with an account must sign in instead.
	taken, err := f.invite(membership.RoleOwner, "member@example.com", membership.RoleViewer)
	noErr(t, err)
	_, err = reg.Do(ctx, usecase.RegisterWithInvitationInput{Token: taken.Token, Name: "M", Password: "a long password!"})
	wantConflict(t, err, domain.ConflictDuplicateEmail)
	if inv, _ := f.Invitations.ByID(ctx, taken.Invitation.ID); !inv.AcceptedAt.IsZero() {
		t.Error("failed registration used the invitation")
	}
}
