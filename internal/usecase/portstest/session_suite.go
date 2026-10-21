package portstest

import (
	"context"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

func wantSession(t *testing.T, got, want usecase.Session) {
	t.Helper()
	if got.TokenHash != want.TokenHash || got.UserID != want.UserID {
		t.Errorf("session = %+v, want %+v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	if !got.LastSeenAt.Equal(want.LastSeenAt) {
		t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, want.LastSeenAt)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
}

// newSession returns a session created an hour ago that lasts ttl from now.
func newSession(env Env, hash string, userID domain.ID, ttl time.Duration) usecase.Session {
	return usecase.Session{
		TokenHash:  hash,
		UserID:     userID,
		CreatedAt:  now(env).Add(-time.Hour),
		LastSeenAt: now(env).Add(-time.Minute),
		ExpiresAt:  now(env).Add(ttl),
	}
}

// RunSessionSuite checks a SessionRepository. It needs Users to own the
// sessions.
func RunSessionSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	seed := func(t *testing.T) Env {
		t.Helper()
		env := factory(t)
		noError(t, env.Users.Create(ctx, newUser(env, "u1")))
		noError(t, env.Users.Create(ctx, newUser(env, "u2")))
		return env
	}

	exists := func(t *testing.T, env Env, hash string) bool {
		t.Helper()
		_, err := env.Sessions.ByTokenHash(ctx, hash)
		if err != nil {
			wantError(t, err, domain.ErrNotFound)
		}
		return err == nil
	}

	t.Run("Create then read back", func(t *testing.T) {
		env := seed(t)
		s := newSession(env, "hash-1", "u1", time.Hour)
		noError(t, env.Sessions.Create(ctx, s))

		got, err := env.Sessions.ByTokenHash(ctx, "hash-1")
		noError(t, err)
		wantSession(t, got, s)
	})

	t.Run("ByTokenHash of an unknown hash gives ErrNotFound", func(t *testing.T) {
		env := seed(t)
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-1", "u1", time.Hour)))

		_, err := env.Sessions.ByTokenHash(ctx, "hash-2")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Sessions.ByTokenHash(ctx, "")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("ByTokenHash returns an expired session", func(t *testing.T) {
		env := seed(t)
		expired := newSession(env, "hash-1", "u1", -time.Minute)
		noError(t, env.Sessions.Create(ctx, expired))

		got, err := env.Sessions.ByTokenHash(ctx, "hash-1")
		noError(t, err)
		wantSession(t, got, expired)
	})

	t.Run("Create rejects a taken token hash", func(t *testing.T) {
		env := seed(t)
		first := newSession(env, "hash-1", "u1", time.Hour)
		noError(t, env.Sessions.Create(ctx, first))

		wantError(t, env.Sessions.Create(ctx, newSession(env, "hash-1", "u2", time.Hour)), domain.ErrConflict)

		got, err := env.Sessions.ByTokenHash(ctx, "hash-1")
		noError(t, err)
		wantSession(t, got, first)
	})

	t.Run("Touch moves last seen and expiry only", func(t *testing.T) {
		env := seed(t)
		s := newSession(env, "hash-1", "u1", time.Hour)
		other := newSession(env, "hash-2", "u1", time.Hour)
		noError(t, env.Sessions.Create(ctx, s))
		noError(t, env.Sessions.Create(ctx, other))

		s.LastSeenAt, s.ExpiresAt = now(env), now(env).Add(48*time.Hour)
		noError(t, env.Sessions.Touch(ctx, "hash-1", s.LastSeenAt, s.ExpiresAt))

		got, err := env.Sessions.ByTokenHash(ctx, "hash-1")
		noError(t, err)
		wantSession(t, got, s)
		got, err = env.Sessions.ByTokenHash(ctx, "hash-2")
		noError(t, err)
		wantSession(t, got, other)
	})

	t.Run("Delete removes one session and is idempotent", func(t *testing.T) {
		env := seed(t)
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-1", "u1", time.Hour)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-2", "u1", time.Hour)))

		noError(t, env.Sessions.Delete(ctx, "hash-1"))
		noError(t, env.Sessions.Delete(ctx, "hash-1"))
		noError(t, env.Sessions.Delete(ctx, "never-there"))

		if exists(t, env, "hash-1") {
			t.Error("the deleted session is still there")
		}
		if !exists(t, env, "hash-2") {
			t.Error("Delete removed another session of the user")
		}
	})

	t.Run("DeleteByUser removes every session of that user only", func(t *testing.T) {
		env := seed(t)
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-1", "u1", time.Hour)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-2", "u1", -time.Hour)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "hash-3", "u2", time.Hour)))

		noError(t, env.Sessions.DeleteByUser(ctx, "u1"))
		noError(t, env.Sessions.DeleteByUser(ctx, "u1"))
		noError(t, env.Sessions.DeleteByUser(ctx, "nobody"))

		if exists(t, env, "hash-1") || exists(t, env, "hash-2") {
			t.Error("a session of the user is still there")
		}
		if !exists(t, env, "hash-3") {
			t.Error("DeleteByUser removed a session of another user")
		}
		// The account itself stays.
		_, err := env.Users.ByID(ctx, "u1")
		noError(t, err)
	})

	t.Run("DeleteExpired removes sessions that have ended by now", func(t *testing.T) {
		env := seed(t)
		noError(t, env.Sessions.Create(ctx, newSession(env, "old-1", "u1", -time.Hour)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "old-2", "u2", -time.Second)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "old-now", "u1", 0))) // ends at this very moment
		noError(t, env.Sessions.Create(ctx, newSession(env, "live-1", "u1", time.Second)))
		noError(t, env.Sessions.Create(ctx, newSession(env, "live-2", "u2", time.Hour)))

		noError(t, env.Sessions.DeleteExpired(ctx, now(env)))

		for _, hash := range []string{"old-1", "old-2", "old-now"} {
			if exists(t, env, hash) {
				t.Errorf("expired session %s is still there", hash)
			}
		}
		for _, hash := range []string{"live-1", "live-2"} {
			if !exists(t, env, hash) {
				t.Errorf("live session %s was removed", hash)
			}
		}
		noError(t, env.Sessions.DeleteExpired(ctx, now(env)))
	})
}
