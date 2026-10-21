package portstest

import (
	"context"
	"errors"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/user"
)

func userIDs(users []user.User) []domain.ID {
	ids := make([]domain.ID, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}

func wantUser(t *testing.T, got, want user.User) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	got.CreatedAt = want.CreatedAt
	if got != want {
		t.Errorf("user = %+v, want %+v", got, want)
	}
}

// newUser returns a local member account; id also makes the email unique.
func newUser(env Env, id domain.ID) user.User {
	return user.User{
		ID:        id,
		Email:     string(id) + "@example.com",
		Name:      "User " + string(id),
		Active:    true,
		CreatedAt: now(env),
	}
}

// RunUserSuite checks a UserRepository and the transaction it takes part in.
func RunUserSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back by id, email and OIDC subject", func(t *testing.T) {
		env := factory(t)
		oidc := newUser(env, "u1")
		oidc.OIDCIssuer, oidc.OIDCSubject = "https://idp.test", "sub-1"
		local := newUser(env, "u2")
		local.InstanceAdmin, local.PasswordHash, local.Active = true, "encoded-hash", false
		noError(t, env.Users.Create(ctx, oidc))
		noError(t, env.Users.Create(ctx, local))

		got, err := env.Users.ByID(ctx, "u1")
		noError(t, err)
		wantUser(t, got, oidc)
		got, err = env.Users.ByID(ctx, "u2")
		noError(t, err)
		wantUser(t, got, local)
		got, err = env.Users.ByEmail(ctx, "u2@example.com")
		noError(t, err)
		wantUser(t, got, local)
		got, err = env.Users.ByOIDCSubject(ctx, "https://idp.test", "sub-1")
		noError(t, err)
		wantUser(t, got, oidc)
	})

	t.Run("lookups of unknown users give ErrNotFound", func(t *testing.T) {
		env := factory(t)
		oidc := newUser(env, "u1")
		oidc.OIDCIssuer, oidc.OIDCSubject = "https://idp.test", "sub-1"
		noError(t, env.Users.Create(ctx, oidc))
		noError(t, env.Users.Create(ctx, newUser(env, "u2"))) // no OIDC identity

		_, err := env.Users.ByID(ctx, "nobody")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Users.ByEmail(ctx, "nobody@example.com")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Users.ByOIDCSubject(ctx, "https://idp.test", "sub-2")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Users.ByOIDCSubject(ctx, "https://other.test", "sub-1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Users.ByOIDCSubject(ctx, "", "")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Create rejects a taken email", func(t *testing.T) {
		env := factory(t)
		first := newUser(env, "u1")
		noError(t, env.Users.Create(ctx, first))
		second := newUser(env, "u2")
		second.Email = first.Email

		wantConflict(t, env.Users.Create(ctx, second), domain.ConflictDuplicateEmail)

		_, err := env.Users.ByID(ctx, "u2")
		wantError(t, err, domain.ErrNotFound)
		got, err := env.Users.ByEmail(ctx, first.Email)
		noError(t, err)
		wantUser(t, got, first)
	})

	t.Run("Create rejects a taken OIDC subject", func(t *testing.T) {
		env := factory(t)
		first := newUser(env, "u1")
		first.OIDCIssuer, first.OIDCSubject = "https://idp.test", "sub-1"
		noError(t, env.Users.Create(ctx, first))
		second := newUser(env, "u2")
		second.OIDCIssuer, second.OIDCSubject = "https://idp.test", "sub-1"

		wantError(t, env.Users.Create(ctx, second), domain.ErrConflict)

		sameIssuer := newUser(env, "u3")
		sameIssuer.OIDCIssuer, sameIssuer.OIDCSubject = "https://idp.test", "sub-3"
		noError(t, env.Users.Create(ctx, sameIssuer))
	})

	t.Run("several accounts may have no OIDC identity", func(t *testing.T) {
		env := factory(t)
		noError(t, env.Users.Create(ctx, newUser(env, "u1")))
		noError(t, env.Users.Create(ctx, newUser(env, "u2")))
	})

	t.Run("Update stores every field", func(t *testing.T) {
		env := factory(t)
		u := newUser(env, "u1")
		noError(t, env.Users.Create(ctx, u))
		noError(t, env.Users.Create(ctx, newUser(env, "u2")))

		u.Email, u.Name, u.InstanceAdmin, u.Active = "new@example.com", "New Name", true, false
		u.OIDCIssuer, u.OIDCSubject, u.PasswordHash = "https://idp.test", "sub-1", "encoded-hash"
		noError(t, env.Users.Update(ctx, u))

		got, err := env.Users.ByID(ctx, "u1")
		noError(t, err)
		wantUser(t, got, u)
		_, err = env.Users.ByEmail(ctx, "u1@example.com")
		wantError(t, err, domain.ErrNotFound)
		other, err := env.Users.ByID(ctx, "u2")
		noError(t, err)
		wantUser(t, other, newUser(env, "u2"))
	})

	t.Run("Update of an unknown user gives ErrNotFound", func(t *testing.T) {
		env := factory(t)
		wantError(t, env.Users.Update(ctx, newUser(env, "nobody")), domain.ErrNotFound)
	})

	t.Run("Update rejects the email of another user", func(t *testing.T) {
		env := factory(t)
		noError(t, env.Users.Create(ctx, newUser(env, "u1")))
		u2 := newUser(env, "u2")
		noError(t, env.Users.Create(ctx, u2))

		changed := u2
		changed.Email = "u1@example.com"
		wantError(t, env.Users.Update(ctx, changed), domain.ErrConflict)

		got, err := env.Users.ByID(ctx, "u2")
		noError(t, err)
		wantUser(t, got, u2)
	})

	t.Run("ByIDs skips unknown ids", func(t *testing.T) {
		env := factory(t)
		for _, id := range []domain.ID{"u1", "u2", "u3"} {
			noError(t, env.Users.Create(ctx, newUser(env, id)))
		}

		got, err := env.Users.ByIDs(ctx, []domain.ID{"u3", "nobody", "u1"})
		noError(t, err)
		sameSet(t, "ByIDs", userIDs(got), []domain.ID{"u1", "u3"})
		for _, u := range got {
			wantUser(t, u, newUser(env, u.ID))
		}

		got, err = env.Users.ByIDs(ctx, nil)
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("ByIDs(nil) = %v, want none", got)
		}
		got, err = env.Users.ByIDs(ctx, []domain.ID{"nobody"})
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("ByIDs of unknown ids = %v, want none", got)
		}
	})

	t.Run("a failed transaction leaves no trace", func(t *testing.T) {
		env := factory(t)
		kept := newUser(env, "u1")
		noError(t, env.Users.Create(ctx, kept))
		failure := errors.New("boom")

		err := env.Tx.WithinTx(ctx, func(ctx context.Context) error {
			noError(t, env.Users.Create(ctx, newUser(env, "u2")))
			changed := kept
			changed.Name = "Changed"
			noError(t, env.Users.Update(ctx, changed))
			if _, err := env.Users.ByID(ctx, "u2"); err != nil {
				t.Errorf("the transaction does not see its own write: %v", err)
			}
			return failure
		})
		wantError(t, err, failure)

		_, err = env.Users.ByID(ctx, "u2")
		wantError(t, err, domain.ErrNotFound)
		got, err := env.Users.ByID(ctx, "u1")
		noError(t, err)
		wantUser(t, got, kept)
		// The rolled back email is free again.
		noError(t, env.Users.Create(ctx, newUser(env, "u2")))
	})

	t.Run("a successful transaction keeps its writes", func(t *testing.T) {
		env := factory(t)

		noError(t, env.Tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := env.Users.Create(ctx, newUser(env, "u1")); err != nil {
				return err
			}
			return env.Users.Create(ctx, newUser(env, "u2"))
		}))

		got, err := env.Users.ByIDs(ctx, []domain.ID{"u1", "u2"})
		noError(t, err)
		sameSet(t, "ByIDs", userIDs(got), []domain.ID{"u1", "u2"})
	})

	t.Run("a conflict inside a transaction can be rolled back", func(t *testing.T) {
		env := factory(t)
		noError(t, env.Users.Create(ctx, newUser(env, "u1")))
		clash := newUser(env, "u3")
		clash.Email = "u1@example.com"

		err := env.Tx.WithinTx(ctx, func(ctx context.Context) error {
			noError(t, env.Users.Create(ctx, newUser(env, "u2")))
			return env.Users.Create(ctx, clash)
		})
		wantConflict(t, err, domain.ConflictDuplicateEmail)

		got, err := env.Users.ByIDs(ctx, []domain.ID{"u1", "u2"})
		noError(t, err)
		sameSet(t, "ByIDs", userIDs(got), []domain.ID{"u1"})
	})
}
