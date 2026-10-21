package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var (
	_ usecase.EnvelopeGroupRepository = (*EnvelopeGroupRepository)(nil)
	_ usecase.EnvelopeRepository      = (*EnvelopeRepository)(nil)
	_ usecase.AssignmentRepository    = (*AssignmentRepository)(nil)
)

// EnvelopeGroupRepository stores envelope groups.
type EnvelopeGroupRepository struct{ db *DB }

func NewEnvelopeGroupRepository(db *DB) *EnvelopeGroupRepository {
	return &EnvelopeGroupRepository{db: db}
}

const groupColumns = `id, household_id, name, position, created_at, updated_at`

func scanGroup(row interface{ Scan(...any) error }) (budget.Group, error) {
	var (
		g                budget.Group
		created, updated int64
	)
	if err := row.Scan(&g.ID, &g.HouseholdID, &g.Name, &g.Position, &created, &updated); err != nil {
		return budget.Group{}, err
	}
	g.CreatedAt, g.UpdatedAt = fromMicros(created), fromMicros(updated)
	return g, nil
}

func mapNameError(err error, table string) error {
	switch {
	case isUnique(err, table+".household_id"), isUnique(err, table+".name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, table+"."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *EnvelopeGroupRepository) Create(ctx context.Context, g budget.Group) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO envelope_groups (`+groupColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		g.ID, g.HouseholdID, g.Name, g.Position, toMicros(g.CreatedAt), toMicros(g.UpdatedAt))
	return mapNameError(err, "envelope_groups")
}

func (r *EnvelopeGroupRepository) Update(ctx context.Context, g budget.Group) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE envelope_groups SET name = ?, position = ?, updated_at = ? WHERE id = ? AND household_id = ?`,
		g.Name, g.Position, toMicros(g.UpdatedAt), g.ID, g.HouseholdID)
	if err != nil {
		return mapNameError(err, "envelope_groups")
	}
	return affected(res)
}

func (r *EnvelopeGroupRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		var n int
		if err := r.db.Exec(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM envelopes WHERE group_id = ? AND household_id = ?`,
			id, householdID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return domain.ConflictError{Reason: domain.ConflictGroupNotEmpty}
		}
		res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM envelope_groups WHERE id = ? AND household_id = ?`, id, householdID)
		if err != nil {
			if isForeignKey(err) {
				return domain.ConflictError{Reason: domain.ConflictGroupNotEmpty}
			}
			return err
		}
		return affected(res)
	})
}

func (r *EnvelopeGroupRepository) ByID(ctx context.Context, householdID, id domain.ID) (budget.Group, error) {
	g, err := scanGroup(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+groupColumns+` FROM envelope_groups WHERE id = ? AND household_id = ?`, id, householdID))
	if err != nil {
		return budget.Group{}, notFound(err)
	}
	return g, nil
}

func (r *EnvelopeGroupRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Group, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+groupColumns+` FROM envelope_groups WHERE household_id = ? ORDER BY position, lower(name), id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []budget.Group{}
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// lower() in SQLite folds ASCII only; the domain order is the rule.
	budget.SortGroups(out)
	return out, nil
}

// EnvelopeRepository stores envelopes with their goals; deleting one
// removes its assignments through the foreign key.
type EnvelopeRepository struct{ db *DB }

func NewEnvelopeRepository(db *DB) *EnvelopeRepository { return &EnvelopeRepository{db: db} }

const envelopeColumns = `id, household_id, group_id, name, position, hidden, goal_type, goal_amount, goal_date, created_at, updated_at`

func scanEnvelope(row interface{ Scan(...any) error }) (budget.Envelope, error) {
	var (
		e                  budget.Envelope
		goalType, goalDate sql.NullString
		goalAmount         sql.NullInt64
		created, updated   int64
	)
	if err := row.Scan(&e.ID, &e.HouseholdID, &e.GroupID, &e.Name, &e.Position, &e.Hidden, &goalType, &goalAmount, &goalDate,
		&created, &updated); err != nil {
		return budget.Envelope{}, err
	}
	e.Goal = budget.Goal{Type: budget.GoalType(goalType.String), Amount: goalAmount.Int64}
	if goalDate.Valid {
		d, err := domain.ParseDate(goalDate.String)
		if err != nil {
			return budget.Envelope{}, fmt.Errorf("sqlite: goal date of envelope %s: %w", e.ID, err)
		}
		e.Goal.TargetDate = d
	}
	e.CreatedAt, e.UpdatedAt = fromMicros(created), fromMicros(updated)
	return e, nil
}

func goalArgs(g budget.Goal) []any {
	if g.IsZero() {
		return []any{nil, nil, nil}
	}
	return []any{string(g.Type), g.Amount, nullString(g.TargetDate.String())}
}

// groupOfHousehold is the condition that the group of an envelope write
// belongs to the household.
const groupOfHousehold = `EXISTS (SELECT 1 FROM envelope_groups WHERE id = ? AND household_id = ?)`

func (r *EnvelopeRepository) Create(ctx context.Context, e budget.Envelope) error {
	args := append([]any{e.ID, e.HouseholdID, e.GroupID, e.Name, e.Position, e.Hidden}, goalArgs(e.Goal)...)
	args = append(args, toMicros(e.CreatedAt), toMicros(e.UpdatedAt), e.GroupID, e.HouseholdID)
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO envelopes (`+envelopeColumns+`) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE `+groupOfHousehold, args...)
	if err != nil {
		return mapNameError(err, "envelopes")
	}
	return affected(res)
}

func (r *EnvelopeRepository) Update(ctx context.Context, e budget.Envelope) error {
	args := append([]any{e.GroupID, e.Name, e.Position, e.Hidden}, goalArgs(e.Goal)...)
	args = append(args, toMicros(e.UpdatedAt), e.ID, e.HouseholdID, e.GroupID, e.HouseholdID)
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE envelopes SET group_id = ?, name = ?, position = ?, hidden = ?, goal_type = ?, goal_amount = ?, goal_date = ?,
		   updated_at = ?
		 WHERE id = ? AND household_id = ? AND `+groupOfHousehold, args...)
	if err != nil {
		return mapNameError(err, "envelopes")
	}
	return affected(res)
}

func (r *EnvelopeRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM envelopes WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *EnvelopeRepository) ByID(ctx context.Context, householdID, id domain.ID) (budget.Envelope, error) {
	e, err := scanEnvelope(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+envelopeColumns+` FROM envelopes WHERE id = ? AND household_id = ?`, id, householdID))
	if err != nil {
		return budget.Envelope{}, notFound(err)
	}
	return e, nil
}

func (r *EnvelopeRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Envelope, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+envelopeColumns+` FROM envelopes WHERE household_id = ? ORDER BY position, lower(name), id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []budget.Envelope{}
	for rows.Next() {
		e, err := scanEnvelope(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	budget.SortEnvelopes(out)
	return out, nil
}

// AssignmentRepository stores the monthly assignments of envelopes.
type AssignmentRepository struct{ db *DB }

func NewAssignmentRepository(db *DB) *AssignmentRepository { return &AssignmentRepository{db: db} }

func (r *AssignmentRepository) Set(ctx context.Context, a budget.Assignment) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		var n int
		if err := r.db.Exec(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM envelopes WHERE id = ? AND household_id = ?`,
			a.EnvelopeID, a.HouseholdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		if a.Amount == 0 {
			_, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM envelope_assignments WHERE envelope_id = ? AND month = ?`,
				a.EnvelopeID, a.Month.String())
			return err
		}
		_, err := r.db.Exec(ctx).ExecContext(ctx,
			`INSERT INTO envelope_assignments (household_id, envelope_id, month, amount, updated_at) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT (envelope_id, month) DO UPDATE SET amount = excluded.amount, updated_at = excluded.updated_at`,
			a.HouseholdID, a.EnvelopeID, a.Month.String(), a.Amount, toMicros(a.UpdatedAt))
		return err
	})
}

func (r *AssignmentRepository) Get(ctx context.Context, householdID, envelopeID domain.ID, m budget.Month) (budget.Assignment, error) {
	a := budget.Assignment{HouseholdID: householdID, EnvelopeID: envelopeID, Month: m}
	var updated int64
	err := r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT amount, updated_at FROM envelope_assignments WHERE household_id = ? AND envelope_id = ? AND month = ?`,
		householdID, envelopeID, m.String()).Scan(&a.Amount, &updated)
	if err == sql.ErrNoRows {
		return a, nil
	}
	if err != nil {
		return budget.Assignment{}, err
	}
	a.UpdatedAt = fromMicros(updated)
	return a, nil
}

func (r *AssignmentRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Assignment, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT envelope_id, month, amount, updated_at FROM envelope_assignments WHERE household_id = ? ORDER BY month, envelope_id`,
		householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []budget.Assignment{}
	for rows.Next() {
		var (
			a       = budget.Assignment{HouseholdID: householdID}
			month   string
			updated int64
		)
		if err := rows.Scan(&a.EnvelopeID, &month, &a.Amount, &updated); err != nil {
			return nil, err
		}
		if a.Month, err = budget.ParseMonth("month", month); err != nil {
			return nil, fmt.Errorf("sqlite: month of assignment of %s: %w", a.EnvelopeID, err)
		}
		a.UpdatedAt = fromMicros(updated)
		out = append(out, a)
	}
	return out, rows.Err()
}
