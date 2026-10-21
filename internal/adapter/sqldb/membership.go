package sqldb

import (
	"context"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.MembershipRepository = (*MembershipRepository)(nil)

// MembershipRepository stores who belongs to which household.
type MembershipRepository struct{ db *DB }

func NewMembershipRepository(db *DB) *MembershipRepository { return &MembershipRepository{db: db} }

const membershipColumns = `household_id, user_id, role, created_at`

func scanMembership(row interface{ Scan(...any) error }) (membership.Membership, error) {
	var (
		m       membership.Membership
		role    string
		created int64
	)
	if err := row.Scan(&m.HouseholdID, &m.UserID, &role, &created); err != nil {
		return membership.Membership{}, err
	}
	m.Role, m.CreatedAt = membership.Role(role), fromMicros(created)
	return m, nil
}

func (r *MembershipRepository) Create(ctx context.Context, m membership.Membership) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO memberships (`+membershipColumns+`) VALUES (?, ?, ?, ?)`,
		m.HouseholdID, m.UserID, string(m.Role), toMicros(m.CreatedAt))
	switch {
	case isUnique(err, "memberships."):
		return domain.ConflictError{Reason: domain.ConflictAlreadyMember}
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *MembershipRepository) UpdateRole(ctx context.Context, householdID, userID domain.ID, role membership.Role) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE memberships SET role = ? WHERE household_id = ? AND user_id = ?`, string(role), householdID, userID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *MembershipRepository) Delete(ctx context.Context, householdID, userID domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`DELETE FROM memberships WHERE household_id = ? AND user_id = ?`, householdID, userID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *MembershipRepository) Get(ctx context.Context, householdID, userID domain.ID) (membership.Membership, error) {
	m, err := scanMembership(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+membershipColumns+` FROM memberships WHERE household_id = ? AND user_id = ?`, householdID, userID))
	if err != nil {
		return membership.Membership{}, notFound(err)
	}
	return m, nil
}

func (r *MembershipRepository) list(ctx context.Context, q string, arg any) ([]membership.Membership, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []membership.Membership{}
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MembershipRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Membership, error) {
	return r.list(ctx, `SELECT `+membershipColumns+` FROM memberships WHERE household_id = ? ORDER BY created_at, user_id`, householdID)
}

func (r *MembershipRepository) ListByUser(ctx context.Context, userID domain.ID) ([]membership.Membership, error) {
	return r.list(ctx, `SELECT `+membershipColumns+` FROM memberships WHERE user_id = ? ORDER BY created_at, household_id`, userID)
}
