package sqlite

import (
	"context"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.OIDCStateRepository = (*OIDCStateRepository)(nil)

// OIDCStateRepository keeps the short-lived state of a login in progress,
// keyed by the hash of the state value.
type OIDCStateRepository struct{ db *DB }

func NewOIDCStateRepository(db *DB) *OIDCStateRepository { return &OIDCStateRepository{db: db} }

func (r *OIDCStateRepository) Create(ctx context.Context, s usecase.OIDCLoginState) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO oidc_login_states (state_hash, nonce, code_verifier, return_to, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.StateHash, s.Nonce, s.CodeVerifier, s.ReturnTo, toMicros(time.Now()), toMicros(s.ExpiresAt))
	if isUnique(err, "oidc_login_states.state_hash") {
		return domain.ErrConflict
	}
	return err
}

// Consume reads and deletes the state in one statement on the single writer
// connection, so concurrent callers get it at most once.
func (r *OIDCStateRepository) Consume(ctx context.Context, stateHash string) (usecase.OIDCLoginState, error) {
	var (
		s      usecase.OIDCLoginState
		expiry int64
	)
	err := r.db.Exec(ctx).QueryRowContext(ctx,
		`DELETE FROM oidc_login_states WHERE state_hash = ?
		 RETURNING state_hash, nonce, code_verifier, return_to, expires_at`, stateHash).
		Scan(&s.StateHash, &s.Nonce, &s.CodeVerifier, &s.ReturnTo, &expiry)
	if err != nil {
		return usecase.OIDCLoginState{}, notFound(err)
	}
	s.ExpiresAt = fromMicros(expiry)
	return s, nil
}

func (r *OIDCStateRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM oidc_login_states WHERE expires_at <= ?`, toMicros(now))
	return err
}
