package sqlite

import (
	"context"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.SessionRepository = (*SessionRepository)(nil)

// SessionRepository stores sessions keyed by the hash of the session token.
type SessionRepository struct{ db *DB }

func NewSessionRepository(db *DB) *SessionRepository { return &SessionRepository{db: db} }

func (r *SessionRepository) Create(ctx context.Context, s usecase.Session) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		s.TokenHash, s.UserID, toMicros(s.CreatedAt), toMicros(s.LastSeenAt), toMicros(s.ExpiresAt))
	switch {
	case isUnique(err, "sessions.token_hash"):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

// ByTokenHash does not check expiry; the caller does.
func (r *SessionRepository) ByTokenHash(ctx context.Context, hash string) (usecase.Session, error) {
	var (
		s                     usecase.Session
		created, seen, expiry int64
	)
	err := r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT token_hash, user_id, created_at, last_seen_at, expires_at FROM sessions WHERE token_hash = ?`, hash).
		Scan(&s.TokenHash, &s.UserID, &created, &seen, &expiry)
	if err != nil {
		return usecase.Session{}, notFound(err)
	}
	s.CreatedAt, s.LastSeenAt, s.ExpiresAt = fromMicros(created), fromMicros(seen), fromMicros(expiry)
	return s, nil
}

func (r *SessionRepository) Touch(ctx context.Context, hash string, lastSeen, expiresAt time.Time) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE token_hash = ?`,
		toMicros(lastSeen), toMicros(expiresAt), hash)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *SessionRepository) Delete(ctx context.Context, hash string) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash)
	return err
}

func (r *SessionRepository) DeleteByUser(ctx context.Context, userID domain.ID) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// DeleteExpired removes the sessions that have ended by now. A session
// ending at this very moment is over, as it is for Authenticate.
func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, toMicros(now))
	return err
}
