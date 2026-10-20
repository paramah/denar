package fakes

import (
	"context"
	"maps"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// OIDCStateRepository keeps OIDC login states in memory.
type OIDCStateRepository struct {
	store  *Store
	states map[string]usecase.OIDCLoginState
}

func NewOIDCStateRepository(s *Store) *OIDCStateRepository {
	r := &OIDCStateRepository{store: s, states: map[string]usecase.OIDCLoginState{}}
	s.Register(r)
	return r
}

func (r *OIDCStateRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.states)
	return func() { r.states = saved }
}

func (r *OIDCStateRepository) Create(ctx context.Context, s usecase.OIDCLoginState) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.states[s.StateHash]; ok {
			err = domain.ErrConflict
			return
		}
		r.states[s.StateHash] = s
	})
	return err
}

// Consume does not look at the expiry; the caller does.
func (r *OIDCStateRepository) Consume(ctx context.Context, stateHash string) (s usecase.OIDCLoginState, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if s, ok = r.states[stateHash]; !ok {
			err = domain.ErrNotFound
			return
		}
		delete(r.states, stateHash)
	})
	return s, err
}

func (r *OIDCStateRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	r.store.Do(ctx, func() {
		maps.DeleteFunc(r.states, func(_ string, s usecase.OIDCLoginState) bool { return !s.ExpiresAt.After(now) })
	})
	return nil
}
