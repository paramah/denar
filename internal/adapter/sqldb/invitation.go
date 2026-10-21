package sqldb

import (
	"context"
	"database/sql"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.InvitationRepository = (*InvitationRepository)(nil)

// InvitationRepository stores invitations keyed also by the hash of their
// token.
type InvitationRepository struct{ db *DB }

func NewInvitationRepository(db *DB) *InvitationRepository { return &InvitationRepository{db: db} }

const invitationColumns = `id, household_id, email, role, token_hash, created_by, created_at, expires_at, accepted_at, accepted_by, revoked_at`

func scanInvitation(row interface{ Scan(...any) error }) (membership.Invitation, error) {
	var (
		i                 membership.Invitation
		role              string
		created, expires  int64
		accepted, revoked sql.NullInt64
		acceptedBy        sql.NullString
	)
	if err := row.Scan(&i.ID, &i.HouseholdID, &i.Email, &role, &i.TokenHash, &i.CreatedBy, &created, &expires,
		&accepted, &acceptedBy, &revoked); err != nil {
		return membership.Invitation{}, err
	}
	i.Role = membership.Role(role)
	i.CreatedAt, i.ExpiresAt = fromMicros(created), fromMicros(expires)
	i.AcceptedAt, i.AcceptedBy, i.RevokedAt = fromNullMicros(accepted), domain.ID(acceptedBy.String), fromNullMicros(revoked)
	return i, nil
}

func (r *InvitationRepository) Create(ctx context.Context, i membership.Invitation) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO invitations (`+invitationColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.HouseholdID, i.Email, string(i.Role), i.TokenHash, i.CreatedBy, toMicros(i.CreatedAt), toMicros(i.ExpiresAt),
		nullMicros(i.AcceptedAt), nullString(string(i.AcceptedBy)), nullMicros(i.RevokedAt))
	switch {
	case isUnique(err, "invitations."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *InvitationRepository) Update(ctx context.Context, i membership.Invitation) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE invitations SET accepted_at = ?, accepted_by = ?, revoked_at = ? WHERE id = ?`,
		nullMicros(i.AcceptedAt), nullString(string(i.AcceptedBy)), nullMicros(i.RevokedAt), i.ID)
	if err != nil {
		if isForeignKey(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return affected(res)
}

func (r *InvitationRepository) one(ctx context.Context, where string, arg any) (membership.Invitation, error) {
	i, err := scanInvitation(r.db.Query(ctx).QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM invitations WHERE `+where, arg))
	if err != nil {
		return membership.Invitation{}, notFound(err)
	}
	return i, nil
}

func (r *InvitationRepository) ByID(ctx context.Context, id domain.ID) (membership.Invitation, error) {
	return r.one(ctx, `id = ?`, id)
}

func (r *InvitationRepository) ByTokenHash(ctx context.Context, hash string) (membership.Invitation, error) {
	if hash == "" {
		return membership.Invitation{}, domain.ErrNotFound
	}
	return r.one(ctx, `token_hash = ?`, hash)
}

func (r *InvitationRepository) list(ctx context.Context, where string, args ...any) ([]membership.Invitation, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+invitationColumns+` FROM invitations WHERE `+where+` ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []membership.Invitation{}
	for rows.Next() {
		i, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *InvitationRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Invitation, error) {
	return r.list(ctx, `household_id = ?`, householdID)
}

func (r *InvitationRepository) PendingByEmail(ctx context.Context, email string, now time.Time) ([]membership.Invitation, error) {
	return r.list(ctx, `email = ? AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > ?`, email, toMicros(now))
}
