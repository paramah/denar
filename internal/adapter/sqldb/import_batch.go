package sqldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.ImportBatchRepository = (*ImportBatchRepository)(nil)

// ImportBatchRepository stores import batches with their row errors as a
// JSON array and their transactions in import_batch_transactions.
type ImportBatchRepository struct{ db *DB }

func NewImportBatchRepository(db *DB) *ImportBatchRepository { return &ImportBatchRepository{db: db} }

const batchColumns = `id, household_id, wallet_id, source, source_ref, preset, status, rows_total, created_count, duplicate_count,
	error_count, matched_count, reverted_count, errors, created_by_kind, created_by_id, created_at, reverted_by_kind, reverted_by_id, reverted_at`

type rowErrorJSON struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func scanBatch(row interface{ Scan(...any) error }) (imports.Batch, error) {
	var (
		b                    imports.Batch
		errs                 string
		createdKind, revKind string
		created              int64
		reverted             sql.NullInt64
	)
	if err := row.Scan(&b.ID, &b.HouseholdID, &b.WalletID, &b.Source, &b.SourceRef, &b.Preset, &b.Status, &b.Rows, &b.Created,
		&b.Duplicates, &b.Failed, &b.Matched, &b.Reverted, &errs, &createdKind, &b.CreatedBy.ID, &created, &revKind,
		&b.RevertedBy.ID, &reverted); err != nil {
		return imports.Batch{}, err
	}
	var stored []rowErrorJSON
	if err := json.Unmarshal([]byte(errs), &stored); err != nil {
		return imports.Batch{}, fmt.Errorf("sqldb: errors of import batch %s: %w", b.ID, err)
	}
	b.Errors = make([]imports.RowError, len(stored))
	for i, e := range stored {
		b.Errors[i] = imports.RowError{Row: e.Row, Field: e.Field, Message: e.Message}
	}
	b.CreatedBy.Kind, b.RevertedBy.Kind = domain.ActorKind(createdKind), domain.ActorKind(revKind)
	b.CreatedAt, b.RevertedAt = fromMicros(created), fromNullMicros(reverted)
	return b, nil
}

func errorsJSON(errs []imports.RowError) string {
	out := make([]rowErrorJSON, len(errs))
	for i, e := range errs {
		out[i] = rowErrorJSON{Row: e.Row, Field: e.Field, Message: e.Message}
	}
	raw, _ := json.Marshal(out) // plain structs always marshal
	return string(raw)
}

func (r *ImportBatchRepository) Create(ctx context.Context, b imports.Batch) error {
	_, err := r.db.Exec(ctx).ExecContext(ctx, `INSERT INTO import_batches (`+batchColumns+`) VALUES (`+placeholders(20)+`)`,
		b.ID, b.HouseholdID, b.WalletID, b.Source, b.SourceRef, b.Preset, b.Status, b.Rows, b.Created, b.Duplicates, b.Failed,
		b.Matched, b.Reverted, errorsJSON(b.Errors), string(b.CreatedBy.Kind), b.CreatedBy.ID, toMicros(b.CreatedAt),
		string(b.RevertedBy.Kind), b.RevertedBy.ID, nullMicros(b.RevertedAt))
	switch {
	case isUnique(err, "import_batches."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *ImportBatchRepository) Update(ctx context.Context, b imports.Batch) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx,
		`UPDATE import_batches SET status = ?, matched_count = ?, reverted_count = ?, reverted_by_kind = ?, reverted_by_id = ?,
		   reverted_at = ? WHERE id = ? AND household_id = ?`,
		b.Status, b.Matched, b.Reverted, string(b.RevertedBy.Kind), b.RevertedBy.ID, nullMicros(b.RevertedAt), b.ID, b.HouseholdID)
	if err != nil {
		return err
	}
	return affected(res)
}

func (r *ImportBatchRepository) ByID(ctx context.Context, householdID, id domain.ID) (imports.Batch, error) {
	b, err := scanBatch(r.db.Query(ctx).QueryRowContext(ctx,
		`SELECT `+batchColumns+` FROM import_batches WHERE id = ? AND household_id = ?`, id, householdID))
	if err != nil {
		return imports.Batch{}, notFound(err)
	}
	return b, nil
}

var errBadImportCursor = domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}

type importCursorJSON struct {
	F  string    `json:"f"`
	ID domain.ID `json:"id"`
}

func importFingerprint(f usecase.ImportBatchFilter) string {
	ids := slices.Clone(f.WalletIDs)
	slices.Sort(ids)
	canon, _ := json.Marshal([]any{f.HouseholdID, ids})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func (r *ImportBatchRepository) List(ctx context.Context, f usecase.ImportBatchFilter, p usecase.Page) ([]imports.Batch, string, error) {
	fp := importFingerprint(f)
	where, args := "household_id = ? AND wallet_id IN ("+placeholders(len(f.WalletIDs))+")", append([]any{f.HouseholdID}, idArgs(f.WalletIDs)...)
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(p.Cursor)
		var c importCursorJSON
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", errBadImportCursor
		}
		where, args = where+" AND id < ?", append(args, c.ID)
	}
	items := []imports.Batch{}
	if len(f.WalletIDs) == 0 {
		return items, "", nil
	}
	rows, err := r.db.Query(ctx).QueryContext(ctx, `SELECT `+batchColumns+` FROM import_batches WHERE `+where+` ORDER BY id DESC LIMIT ?`,
		append(args, p.Limit+1)...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, b)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > p.Limit {
		items = items[:p.Limit]
		raw, _ := json.Marshal(importCursorJSON{F: fp, ID: items[len(items)-1].ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return items, next, nil
}

func (r *ImportBatchRepository) AddTransactions(ctx context.Context, batchID domain.ID, ids []domain.ID) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			_, err := r.db.Exec(ctx).ExecContext(ctx,
				`INSERT INTO import_batch_transactions (batch_id, transaction_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, batchID, id)
			if isForeignKey(err) {
				return domain.ErrNotFound
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ImportBatchRepository) TransactionIDs(ctx context.Context, householdID, batchID domain.ID) ([]domain.ID, error) {
	if _, err := r.ByID(ctx, householdID, batchID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx).QueryContext(ctx,
		`SELECT transaction_id FROM import_batch_transactions WHERE batch_id = ? ORDER BY transaction_id`, batchID)
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
