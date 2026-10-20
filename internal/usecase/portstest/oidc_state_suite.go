package portstest

import (
	"context"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func newOIDCState(env Env, hash string, ttl time.Duration) usecase.OIDCLoginState {
	return usecase.OIDCLoginState{
		StateHash:    hash,
		Nonce:        "nonce-" + hash,
		CodeVerifier: "verifier-" + hash,
		ReturnTo:     "/teams/ENG?tab=" + hash,
		ExpiresAt:    now(env).Add(ttl),
	}
}

// RunOIDCStateSuite checks an OIDCStateRepository.
func RunOIDCStateSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Consume returns the state once", func(t *testing.T) {
		env := factory(t)
		s := newOIDCState(env, "state-1", 10*time.Minute)
		other := newOIDCState(env, "state-2", 10*time.Minute)
		noError(t, env.OIDCStates.Create(ctx, s))
		noError(t, env.OIDCStates.Create(ctx, other))

		got, err := env.OIDCStates.Consume(ctx, "state-1")
		noError(t, err)
		if !got.ExpiresAt.Equal(s.ExpiresAt) {
			t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, s.ExpiresAt)
		}
		got.ExpiresAt = s.ExpiresAt
		if got != s {
			t.Errorf("state = %+v, want %+v", got, s)
		}

		_, err = env.OIDCStates.Consume(ctx, "state-1")
		wantError(t, err, domain.ErrNotFound)

		// Consuming one state leaves the others alone.
		got, err = env.OIDCStates.Consume(ctx, "state-2")
		noError(t, err)
		if got.Nonce != other.Nonce {
			t.Errorf("nonce = %q, want %q", got.Nonce, other.Nonce)
		}
	})

	t.Run("Consume of an unknown state gives ErrNotFound", func(t *testing.T) {
		env := factory(t)
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "state-1", 10*time.Minute)))

		_, err := env.OIDCStates.Consume(ctx, "state-2")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.OIDCStates.Consume(ctx, "")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Create rejects a taken state hash", func(t *testing.T) {
		env := factory(t)
		first := newOIDCState(env, "state-1", 10*time.Minute)
		noError(t, env.OIDCStates.Create(ctx, first))
		clash := newOIDCState(env, "state-1", 10*time.Minute)
		clash.Nonce = "another"

		wantError(t, env.OIDCStates.Create(ctx, clash), domain.ErrConflict)

		got, err := env.OIDCStates.Consume(ctx, "state-1")
		noError(t, err)
		if got.Nonce != first.Nonce {
			t.Errorf("nonce = %q, want the first state's %q", got.Nonce, first.Nonce)
		}
	})

	t.Run("DeleteExpired removes states that have ended by now", func(t *testing.T) {
		env := factory(t)
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "old-1", -time.Hour)))
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "old-2", -time.Second)))
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "old-now", 0))) // ends at this very moment
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "live-1", time.Second)))
		noError(t, env.OIDCStates.Create(ctx, newOIDCState(env, "live-2", time.Hour)))

		noError(t, env.OIDCStates.DeleteExpired(ctx, now(env)))
		noError(t, env.OIDCStates.DeleteExpired(ctx, now(env)))

		for _, hash := range []string{"old-1", "old-2", "old-now"} {
			_, err := env.OIDCStates.Consume(ctx, hash)
			wantError(t, err, domain.ErrNotFound)
		}
		for _, hash := range []string{"live-1", "live-2"} {
			_, err := env.OIDCStates.Consume(ctx, hash)
			noError(t, err)
		}
	})
}
