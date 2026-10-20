package portstest

import (
	"context"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

func newInvitation(env Env, id, householdID, createdBy domain.ID) membership.Invitation {
	return membership.Invitation{
		ID: id, HouseholdID: householdID, Email: string(id) + "@example.com", Role: membership.RoleMember,
		TokenHash: "hash-" + string(id), CreatedBy: createdBy, CreatedAt: now(env), ExpiresAt: now(env).Add(7 * 24 * time.Hour),
	}
}

func wantInvitation(t *testing.T, got, want membership.Invitation) {
	t.Helper()
	for _, pair := range [][2]time.Time{{got.CreatedAt, want.CreatedAt}, {got.ExpiresAt, want.ExpiresAt},
		{got.AcceptedAt, want.AcceptedAt}, {got.RevokedAt, want.RevokedAt}} {
		if !pair[0].Equal(pair[1]) {
			t.Errorf("time = %v, want %v", pair[0], pair[1])
		}
	}
	got.CreatedAt, got.ExpiresAt, got.AcceptedAt, got.RevokedAt = want.CreatedAt, want.ExpiresAt, want.AcceptedAt, want.RevokedAt
	if got != want {
		t.Errorf("invitation = %+v, want %+v", got, want)
	}
}

// RunInvitationSuite checks an InvitationRepository. It needs Users and
// Households.
func RunInvitationSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back by id and token hash", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		inv := newInvitation(env, "i1", "h1", "u1")
		inv.Role = membership.RoleViewer
		noError(t, env.Invitations.Create(ctx, inv))
		got, err := env.Invitations.ByID(ctx, "i1")
		noError(t, err)
		wantInvitation(t, got, inv)
		got, err = env.Invitations.ByTokenHash(ctx, "hash-i1")
		noError(t, err)
		wantInvitation(t, got, inv)
		_, err = env.Invitations.ByTokenHash(ctx, "")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Invitations.ByID(ctx, "nope")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Create rejects a taken token hash", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Invitations.Create(ctx, newInvitation(env, "i1", "h1", "u1")))
		clash := newInvitation(env, "i2", "h1", "u1")
		clash.TokenHash = "hash-i1"
		wantError(t, env.Invitations.Create(ctx, clash), domain.ErrConflict)
	})

	t.Run("Update stores acceptance and revocation only", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		inv := newInvitation(env, "i1", "h1", "u1")
		noError(t, env.Invitations.Create(ctx, inv))
		changed := inv
		changed.AcceptedAt, changed.AcceptedBy, changed.RevokedAt = now(env).Add(time.Minute), "u2", now(env).Add(2*time.Minute)
		changed.Email, changed.Role = "other@example.com", membership.RoleOwner
		noError(t, env.Invitations.Update(ctx, changed))
		got, err := env.Invitations.ByID(ctx, "i1")
		noError(t, err)
		want := inv
		want.AcceptedAt, want.AcceptedBy, want.RevokedAt = changed.AcceptedAt, "u2", changed.RevokedAt
		wantInvitation(t, got, want)
		wantError(t, env.Invitations.Update(ctx, newInvitation(env, "nope", "h1", "u1")), domain.ErrNotFound)
	})

	t.Run("ListByHousehold is newest first; PendingByEmail keeps usable ones", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		mk := func(id, hid domain.ID, offset time.Duration, email string) membership.Invitation {
			i := newInvitation(env, id, hid, "u1")
			i.CreatedAt, i.Email = now(env).Add(offset), email
			return i
		}
		pending := mk("i1", "h1", 0, "x@example.com")
		accepted := mk("i2", "h1", time.Second, "x@example.com")
		accepted.AcceptedAt, accepted.AcceptedBy = now(env), "u2"
		revoked := mk("i3", "h1", 2*time.Second, "x@example.com")
		revoked.RevokedAt = now(env)
		expired := mk("i4", "h2", 3*time.Second, "x@example.com")
		expired.ExpiresAt = now(env)
		otherEmail := mk("i5", "h2", 4*time.Second, "y@example.com")
		elsewhere := mk("i6", "h2", 5*time.Second, "x@example.com")
		for _, i := range []membership.Invitation{pending, accepted, revoked, expired, otherEmail, elsewhere} {
			noError(t, env.Invitations.Create(ctx, i))
		}
		list, err := env.Invitations.ListByHousehold(ctx, "h1")
		noError(t, err)
		ids := []domain.ID{}
		for _, i := range list {
			ids = append(ids, i.ID)
		}
		sameOrder(t, "ListByHousehold", ids, []domain.ID{"i3", "i2", "i1"})

		got, err := env.Invitations.PendingByEmail(ctx, "x@example.com", now(env))
		noError(t, err)
		ids = ids[:0]
		for _, i := range got {
			ids = append(ids, i.ID)
		}
		sameSet(t, "PendingByEmail", ids, []domain.ID{"i1", "i6"})
	})
}
