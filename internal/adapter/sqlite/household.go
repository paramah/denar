package sqlite

import (
	"context"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.HouseholdRepository = (*HouseholdRepository)(nil)

// HouseholdRepository stores households. Deleting one removes its
// memberships, invitations, bots and audit trail through the foreign keys.
type HouseholdRepository struct{ db *DB }

func NewHouseholdRepository(db *DB) *HouseholdRepository { return &HouseholdRepository{db: db} }

const householdColumns = `id, name, base_currency, timezone, month_start_day, created_by, created_at`

func scanHousehold(row interface{ Scan(...any) error }) (household.Household, error) {
	var (
		h       household.Household
		created int64
	)
	if err := row.Scan(&h.ID, &h.Name, &h.BaseCurrency, &h.Timezone, &h.MonthStartDay, &h.CreatedBy, &created); err != nil {
		return household.Household{}, err
	}
	h.CreatedAt = fromMicros(created)
	return h, nil
}

func (r *HouseholdRepository) Create(ctx context.Context, h household.Household) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx,
		`INSERT INTO households (`+householdColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.BaseCurrency, h.Timezone, h.MonthStartDay, h.CreatedBy, toMicros(h.CreatedAt))
	switch {
	case isUnique(err, "households.id"):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *HouseholdRepository) Update(ctx context.Context, h household.Household) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE households SET name = ?, timezone = ?, month_start_day = ? WHERE id = ?`,
		h.Name, h.Timezone, h.MonthStartDay, h.ID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *HouseholdRepository) Delete(ctx context.Context, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM households WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *HouseholdRepository) ByID(ctx context.Context, id domain.ID) (household.Household, error) {
	h, err := scanHousehold(r.db.Query(ctx).QueryRowContext(ctx, `SELECT `+householdColumns+` FROM households WHERE id = ?`, id))
	if err != nil {
		return household.Household{}, notFound(err)
	}
	return h, nil
}

func (r *HouseholdRepository) ByIDs(ctx context.Context, ids []domain.ID) ([]household.Household, error) {
	out := []household.Household{}
	for start := 0; start < len(ids); start += idsChunk {
		chunk := ids[start:min(start+idsChunk, len(ids))]
		rows, err := r.db.Query(ctx).QueryContext(ctx,
			`SELECT `+householdColumns+` FROM households WHERE id IN (`+placeholders(len(chunk))+`) ORDER BY id`, idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			h, err := scanHousehold(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, h)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *HouseholdRepository) ListIDs(ctx context.Context) ([]domain.ID, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT id FROM households ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ID{}
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
