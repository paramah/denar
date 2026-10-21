package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.UserRepository = (*UserRepository)(nil)

// UserRepository stores users. It only maps rows and constraint errors; the
// rules live in the domain and use cases.
type UserRepository struct{ db *DB }

func NewUserRepository(db *DB) *UserRepository { return &UserRepository{db: db} }

const userColumns = `id, email, name, instance_admin, oidc_issuer, oidc_subject, password_hash, active, created_at`

func scanUser(row interface{ Scan(...any) error }) (user.User, error) {
	var (
		u                             user.User
		issuer, subject, passwordHash sql.NullString
		active                        bool
		created                       int64
	)
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.InstanceAdmin, &issuer, &subject, &passwordHash, &active, &created); err != nil {
		return user.User{}, err
	}
	u.OIDCIssuer, u.OIDCSubject, u.PasswordHash = issuer.String, subject.String, passwordHash.String
	u.Active = active
	u.CreatedAt = fromMicros(created)
	return u, nil
}

func mapUserWriteError(err error) error {
	switch {
	case isUnique(err, "users.email"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateEmail}
	case isUnique(err, "users.oidc_"):
		return fmt.Errorf("%w: oidc identity already linked", domain.ErrConflict)
	case isUnique(err, "users.id"):
		return fmt.Errorf("%w: user id taken", domain.ErrConflict)
	}
	return err
}

func (r *UserRepository) Create(ctx context.Context, u user.User) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.InstanceAdmin, nullString(u.OIDCIssuer), nullString(u.OIDCSubject),
		nullString(u.PasswordHash), u.Active, toMicros(u.CreatedAt))
	return mapUserWriteError(err)
}

// Update stores every field except the id and the creation time.
func (r *UserRepository) Update(ctx context.Context, u user.User) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE users SET email = ?, name = ?, instance_admin = ?, oidc_issuer = ?, oidc_subject = ?, password_hash = ?, active = ?
		 WHERE id = ?`,
		u.Email, u.Name, u.InstanceAdmin, nullString(u.OIDCIssuer), nullString(u.OIDCSubject),
		nullString(u.PasswordHash), u.Active, u.ID)
	if err != nil {
		return mapUserWriteError(err)
	}
	return affected(res)
}

func (r *UserRepository) one(ctx context.Context, where string, args ...any) (user.User, error) {
	u, err := scanUser(r.db.Query(ctx).QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE `+where, args...))
	if err != nil {
		return user.User{}, notFound(err)
	}
	return u, nil
}

func (r *UserRepository) ByID(ctx context.Context, id domain.ID) (user.User, error) {
	return r.one(ctx, `id = ?`, id)
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (user.User, error) {
	return r.one(ctx, `email = ?`, email)
}

func (r *UserRepository) ByOIDCSubject(ctx context.Context, issuer, subject string) (user.User, error) {
	if subject == "" {
		return user.User{}, domain.ErrNotFound
	}
	return r.one(ctx, `oidc_issuer = ? AND oidc_subject = ?`, issuer, subject)
}

func (r *UserRepository) queryUsers(ctx context.Context, q string, args ...any) ([]user.User, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []user.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

const idsChunk = 500

// ByIDs returns the users that exist; unknown ids are skipped.
func (r *UserRepository) ByIDs(ctx context.Context, ids []domain.ID) ([]user.User, error) {
	out := []user.User{}
	for start := 0; start < len(ids); start += idsChunk {
		chunk := ids[start:min(start+idsChunk, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		got, err := r.queryUsers(ctx, `SELECT `+userColumns+` FROM users WHERE id IN (?`+strings.Repeat(",?", len(chunk)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}
