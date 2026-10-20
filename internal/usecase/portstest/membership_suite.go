package portstest

import (
	"context"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

func memberIDs(ms []membership.Membership, household bool) []domain.ID {
	out := make([]domain.ID, len(ms))
	for i, m := range ms {
		if household {
			out[i] = m.HouseholdID
		} else {
			out[i] = m.UserID
		}
	}
	return out
}

// RunMembershipSuite checks a MembershipRepository. It needs Users and
// Households.
func RunMembershipSuite(t *testing.T, factory Factory) {
	ctx := context.Background()
	member := func(env Env, hid, uid domain.ID, role membership.Role, offset time.Duration) membership.Membership {
		return membership.Membership{HouseholdID: hid, UserID: uid, Role: role, CreatedAt: now(env).Add(offset)}
	}

	t.Run("Create, Get and the lists", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2", "u3")
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u2")))
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u1", membership.RoleOwner, 0)))
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u3", membership.RoleViewer, time.Second)))
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u2", membership.RoleMember, 2*time.Second)))
		noError(t, env.Memberships.Create(ctx, member(env, "h2", "u2", membership.RoleOwner, -time.Second)))

		got, err := env.Memberships.Get(ctx, "h1", "u3")
		noError(t, err)
		if got.Role != membership.RoleViewer || got.HouseholdID != "h1" || got.UserID != "u3" || !got.CreatedAt.Equal(now(env).Add(time.Second)) {
			t.Errorf("Get = %+v", got)
		}
		_, err = env.Memberships.Get(ctx, "h2", "u1")
		wantError(t, err, domain.ErrNotFound)

		list, err := env.Memberships.ListByHousehold(ctx, "h1")
		noError(t, err)
		sameOrder(t, "ListByHousehold", memberIDs(list, false), []domain.ID{"u1", "u3", "u2"})
		mine, err := env.Memberships.ListByUser(ctx, "u2")
		noError(t, err)
		sameOrder(t, "ListByUser", memberIDs(mine, true), []domain.ID{"h2", "h1"})
		none, err := env.Memberships.ListByUser(ctx, "nobody")
		noError(t, err)
		if len(none) != 0 {
			t.Errorf("ListByUser(nobody) = %v", none)
		}
	})

	t.Run("Create rejects a second membership", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u1", membership.RoleOwner, 0)))
		wantConflict(t, env.Memberships.Create(ctx, member(env, "h1", "u1", membership.RoleViewer, 0)), domain.ConflictAlreadyMember)
	})

	t.Run("UpdateRole and Delete", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u1", membership.RoleOwner, 0)))
		noError(t, env.Memberships.Create(ctx, member(env, "h1", "u2", membership.RoleMember, 0)))

		noError(t, env.Memberships.UpdateRole(ctx, "h1", "u2", membership.RoleAdmin))
		got, err := env.Memberships.Get(ctx, "h1", "u2")
		noError(t, err)
		if got.Role != membership.RoleAdmin {
			t.Errorf("role = %s", got.Role)
		}
		wantError(t, env.Memberships.UpdateRole(ctx, "h1", "nobody", membership.RoleAdmin), domain.ErrNotFound)

		noError(t, env.Memberships.Delete(ctx, "h1", "u2"))
		_, err = env.Memberships.Get(ctx, "h1", "u2")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.Memberships.Delete(ctx, "h1", "u2"), domain.ErrNotFound)
		_, err = env.Memberships.Get(ctx, "h1", "u1")
		noError(t, err)
	})
}
