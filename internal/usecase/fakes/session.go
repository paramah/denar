package fakes

import (
	"context"
	"maps"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

// SessionRepository keeps sessions in memory. Set DeleteByUserErr to make
// that method fail, to test that a use case rolls back what it wrote before.
type SessionRepository struct {
	DeleteByUserErr error

	store    *Store
	sessions map[string]usecase.Session
}

func NewSessionRepository(s *Store) *SessionRepository {
	r := &SessionRepository{store: s, sessions: map[string]usecase.Session{}}
	s.Register(r)
	return r
}

func (r *SessionRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.sessions)
	return func() { r.sessions = saved }
}

func (r *SessionRepository) Create(ctx context.Context, s usecase.Session) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.sessions[s.TokenHash]; ok {
			err = domain.ErrConflict
			return
		}
		r.sessions[s.TokenHash] = s
	})
	return err
}

func (r *SessionRepository) ByTokenHash(ctx context.Context, hash string) (s usecase.Session, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if s, ok = r.sessions[hash]; !ok {
			err = domain.ErrNotFound
		}
	})
	return s, err
}

func (r *SessionRepository) Touch(ctx context.Context, hash string, lastSeen, expiresAt time.Time) (err error) {
	r.store.Do(ctx, func() {
		s, ok := r.sessions[hash]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		s.LastSeenAt, s.ExpiresAt = lastSeen, expiresAt
		r.sessions[hash] = s
	})
	return err
}

func (r *SessionRepository) Delete(ctx context.Context, hash string) error {
	r.store.Do(ctx, func() { delete(r.sessions, hash) })
	return nil
}

func (r *SessionRepository) DeleteByUser(ctx context.Context, userID domain.ID) error {
	if r.DeleteByUserErr != nil {
		return r.DeleteByUserErr
	}
	r.store.Do(ctx, func() {
		maps.DeleteFunc(r.sessions, func(_ string, s usecase.Session) bool { return s.UserID == userID })
	})
	return nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	r.store.Do(ctx, func() {
		maps.DeleteFunc(r.sessions, func(_ string, s usecase.Session) bool { return !s.ExpiresAt.After(now) })
	})
	return nil
}
