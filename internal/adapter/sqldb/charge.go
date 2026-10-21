package sqldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.ChargeRepository = (*ChargeRepository)(nil)

// ChargeRepository stores the charges expected from subscriptions.
type ChargeRepository struct{ db *DB }

func NewChargeRepository(db *DB) *ChargeRepository { return &ChargeRepository{db: db} }

const chargeColumns = `id, household_id, subscription_id, due_date, amount, currency, status, transaction_id, created_at, updated_at`

func scanCharge(row interface{ Scan(...any) error }) (subscription.Charge, error) {
	var (
		c                subscription.Charge
		due              string
		txID             sql.NullString
		created, updated int64
	)
	if err := row.Scan(&c.ID, &c.HouseholdID, &c.SubscriptionID, &due, &c.Amount, &c.Currency, &c.Status, &txID, &created, &updated); err != nil {
		return subscription.Charge{}, err
	}
	var err error
	if c.DueDate, err = domain.ParseDate(due); err != nil {
		return subscription.Charge{}, fmt.Errorf("sqldb: due date of charge %s: %w", c.ID, err)
	}
	c.TransactionID, c.CreatedAt, c.UpdatedAt = domain.ID(txID.String), fromMicros(created), fromMicros(updated)
	return c, nil
}

func mapChargeWriteError(err error) error {
	switch {
	case isUnique(err, "subscription_charges.subscription_id"), isUnique(err, "subscription_charges.due_date"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateCharge}
	case isUnique(err, "subscription_charges.transaction_id"):
		return domain.ConflictError{Reason: domain.ConflictTransactionLinked}
	case isUnique(err, "subscription_charges."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *ChargeRepository) Create(ctx context.Context, c subscription.Charge) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO subscription_charges (`+chargeColumns+`) VALUES (`+placeholders(10)+`)`,
		c.ID, c.HouseholdID, c.SubscriptionID, c.DueDate.String(), c.Amount, c.Currency, c.Status, nullString(string(c.TransactionID)),
		toMicros(c.CreatedAt), toMicros(c.UpdatedAt))
	return mapChargeWriteError(err)
}

func (r *ChargeRepository) Update(ctx context.Context, c subscription.Charge) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `UPDATE subscription_charges SET amount = ?, currency = ?, status = ?, transaction_id = ?,
		updated_at = ? WHERE id = ? AND household_id = ?`,
		c.Amount, c.Currency, c.Status, nullString(string(c.TransactionID)), toMicros(c.UpdatedAt), c.ID, c.HouseholdID)
	if err != nil {
		return mapChargeWriteError(err)
	}
	return affected(res)
}

func (r *ChargeRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM subscription_charges WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *ChargeRepository) one(ctx context.Context, where string, args ...any) (subscription.Charge, error) {
	c, err := scanCharge(r.db.Query(ctx).QueryRowContext(ctx, `SELECT `+chargeColumns+` FROM subscription_charges WHERE `+where, args...))
	if err != nil {
		return subscription.Charge{}, notFound(err)
	}
	return c, nil
}

func (r *ChargeRepository) ByID(ctx context.Context, householdID, id domain.ID) (subscription.Charge, error) {
	return r.one(ctx, `id = ? AND household_id = ?`, id, householdID)
}

func (r *ChargeRepository) ByTransaction(ctx context.Context, householdID, transactionID domain.ID) (subscription.Charge, error) {
	return r.one(ctx, `transaction_id = ? AND household_id = ?`, transactionID, householdID)
}

func (r *ChargeRepository) query(ctx context.Context, sqlText string, args ...any) ([]subscription.Charge, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []subscription.Charge{}
	for rows.Next() {
		c, err := scanCharge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var errBadChargeCursor = domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}

type chargeCursorJSON struct {
	F  string    `json:"f"`
	D  string    `json:"d"`
	ID domain.ID `json:"id"`
}

func chargeFingerprint(f usecase.ChargeFilter) string {
	ids := slices.Clone(f.SubscriptionIDs)
	slices.Sort(ids)
	statuses := slices.Clone(f.Statuses)
	slices.Sort(statuses)
	canon, _ := json.Marshal([]any{f.HouseholdID, ids, statuses, f.From.String(), f.To.String()})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func (r *ChargeRepository) List(ctx context.Context, f usecase.ChargeFilter, p usecase.Page) ([]subscription.Charge, string, error) {
	fp := chargeFingerprint(f)
	var after *chargeCursorJSON
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(p.Cursor)
		var c chargeCursorJSON
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", errBadChargeCursor
		}
		after = &c
	}
	if len(f.SubscriptionIDs) == 0 {
		return []subscription.Charge{}, "", nil
	}
	where := []string{"household_id = ?", "subscription_id IN (" + placeholders(len(f.SubscriptionIDs)) + ")"}
	args := append([]any{f.HouseholdID}, idArgs(f.SubscriptionIDs)...)
	if len(f.Statuses) > 0 {
		where = append(where, "status IN ("+placeholders(len(f.Statuses))+")")
		for _, s := range f.Statuses {
			args = append(args, s)
		}
	}
	if !f.From.IsZero() {
		where, args = append(where, "due_date >= ?"), append(args, f.From.String())
	}
	if !f.To.IsZero() {
		where, args = append(where, "due_date <= ?"), append(args, f.To.String())
	}
	if after != nil {
		where, args = append(where, "(due_date > ? OR (due_date = ? AND id > ?))"), append(args, after.D, after.D, after.ID)
	}
	args = append(args, p.Limit+1)
	items, err := r.query(ctx, `SELECT `+chargeColumns+` FROM subscription_charges WHERE `+strings.Join(where, " AND ")+
		` ORDER BY due_date, id LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > p.Limit {
		items = items[:p.Limit]
		last := items[len(items)-1]
		raw, _ := json.Marshal(chargeCursorJSON{F: fp, D: last.DueDate.String(), ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return items, next, nil
}

func (r *ChargeRepository) ListBySubscription(ctx context.Context, householdID, subscriptionID domain.ID, from domain.Date) ([]subscription.Charge, error) {
	return r.query(ctx, `SELECT `+chargeColumns+` FROM subscription_charges WHERE household_id = ? AND subscription_id = ? AND due_date >= ?
		ORDER BY due_date, id`, householdID, subscriptionID, from.String())
}

func (r *ChargeRepository) ListOpen(ctx context.Context, householdID domain.ID, from, to domain.Date, limit int) ([]subscription.Charge, error) {
	until := "9999-12-31"
	if !to.IsZero() {
		until = to.String()
	}
	where := `status IN ('expected', 'missed') AND due_date >= ? AND due_date <= ?`
	args := []any{from.String(), until}
	if householdID != "" {
		where += ` AND household_id = ?`
		args = append(args, householdID)
	}
	return r.query(ctx, `SELECT `+chargeColumns+` FROM subscription_charges WHERE `+where+` ORDER BY due_date, id LIMIT ?`,
		append(args, limit)...)
}
