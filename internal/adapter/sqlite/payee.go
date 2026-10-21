package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.PayeeRepository = (*PayeeRepository)(nil)

// PayeeRepository stores payees with their patterns as a JSON array.
// Deleting one sets payee_id of its transactions to NULL through the
// foreign key.
type PayeeRepository struct{ db *DB }

func NewPayeeRepository(db *DB) *PayeeRepository { return &PayeeRepository{db: db} }

const payeeColumns = `id, household_id, name, patterns, default_envelope_id, created_at, updated_at`

func scanPayee(row interface{ Scan(...any) error }) (payee.Payee, error) {
	var (
		p                payee.Payee
		patterns         string
		envelope         *string
		created, updated int64
	)
	if err := row.Scan(&p.ID, &p.HouseholdID, &p.Name, &patterns, &envelope, &created, &updated); err != nil {
		return payee.Payee{}, err
	}
	if err := json.Unmarshal([]byte(patterns), &p.Patterns); err != nil {
		return payee.Payee{}, fmt.Errorf("sqlite: patterns of payee %s: %w", p.ID, err)
	}
	if envelope != nil {
		p.DefaultEnvelopeID = domain.ID(*envelope)
	}
	p.CreatedAt, p.UpdatedAt = fromMicros(created), fromMicros(updated)
	return p, nil
}

func stringsJSON(s []string) string {
	if s == nil {
		s = []string{}
	}
	raw, _ := json.Marshal(s) // a slice of strings always marshals
	return string(raw)
}

func mapPayeeWriteError(err error) error {
	switch {
	case isUnique(err, "payees.household_id"), isUnique(err, "payees.name"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateName}
	case isUnique(err, "payees."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *PayeeRepository) Create(ctx context.Context, p payee.Payee) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO payees (`+payeeColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.HouseholdID, p.Name, stringsJSON(p.Patterns), nullString(string(p.DefaultEnvelopeID)), toMicros(p.CreatedAt), toMicros(p.UpdatedAt))
	return mapPayeeWriteError(err)
}

func (r *PayeeRepository) Update(ctx context.Context, p payee.Payee) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE payees SET name = ?, patterns = ?, default_envelope_id = ?, updated_at = ? WHERE id = ? AND household_id = ?`,
		p.Name, stringsJSON(p.Patterns), nullString(string(p.DefaultEnvelopeID)), toMicros(p.UpdatedAt), p.ID, p.HouseholdID)
	if err != nil {
		return mapPayeeWriteError(err)
	}
	return affected(res)
}

func (r *PayeeRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM payees WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *PayeeRepository) ByID(ctx context.Context, householdID, id domain.ID) (payee.Payee, error) {
	p, err := scanPayee(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+payeeColumns+` FROM payees WHERE id = ? AND household_id = ?`, id, householdID))
	if err != nil {
		return payee.Payee{}, notFound(err)
	}
	return p, nil
}

func (r *PayeeRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]payee.Payee, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT `+payeeColumns+` FROM payees WHERE household_id = ? ORDER BY lower(name), id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []payee.Payee{}
	for rows.Next() {
		p, err := scanPayee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
