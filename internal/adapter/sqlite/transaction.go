package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.TransactionRepository = (*TransactionRepository)(nil)

// TransactionRepository stores transactions, their tags as a JSON array
// and their split lines in transaction_splits.
type TransactionRepository struct{ db *DB }

func NewTransactionRepository(db *DB) *TransactionRepository { return &TransactionRepository{db: db} }

const txColumns = `t.id, t.household_id, t.wallet_id, t.date, t.amount, t.currency, t.base_amount, t.rate_pending, t.fx_rate,
	t.payee_id, t.envelope_id, t.kind, t.status, t.source, t.tags, t.note, t.external_id, t.transfer_pair_id,
	t.transfer_wallet_id, t.created_by_kind, t.created_by_id, t.created_at, t.updated_at`

func scanTx(row interface{ Scan(...any) error }) (transaction.Transaction, error) {
	var (
		t                                       transaction.Transaction
		date, tags, kind                        string
		base                                    sql.NullInt64
		fxRate, payeeID, envelopeID, externalID sql.NullString
		pairID, otherWallet                     sql.NullString
		created, updated                        int64
	)
	if err := row.Scan(&t.ID, &t.HouseholdID, &t.WalletID, &date, &t.Amount, &t.Currency, &base, &t.RatePending, &fxRate,
		&payeeID, &envelopeID, &t.Kind, &t.Status, &t.Source, &tags, &t.Note, &externalID, &pairID, &otherWallet,
		&kind, &t.CreatedBy.ID, &created, &updated); err != nil {
		return transaction.Transaction{}, err
	}
	var err error
	if t.Date, err = domain.ParseDate(date); err != nil {
		return transaction.Transaction{}, fmt.Errorf("sqlite: date of transaction %s: %w", t.ID, err)
	}
	if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
		return transaction.Transaction{}, fmt.Errorf("sqlite: tags of transaction %s: %w", t.ID, err)
	}
	t.BaseAmount, t.FxRate = base.Int64, fxRate.String
	t.PayeeID, t.EnvelopeID, t.ExternalID = domain.ID(payeeID.String), domain.ID(envelopeID.String), externalID.String
	t.TransferPairID, t.TransferWalletID = domain.ID(pairID.String), domain.ID(otherWallet.String)
	t.CreatedBy.Kind, t.CreatedAt, t.UpdatedAt = domain.ActorKind(kind), fromMicros(created), fromMicros(updated)
	t.Splits = []transaction.Split{}
	return t, nil
}

func nullBase(t transaction.Transaction, amount int64) sql.NullInt64 {
	return sql.NullInt64{Int64: amount, Valid: !t.RatePending}
}

func mapTxWriteError(err error) error {
	switch {
	case isUnique(err, "transactions.wallet_id"), isUnique(err, "transactions.external_id"):
		return domain.ConflictError{Reason: domain.ConflictDuplicateExternalID}
	case isUnique(err, "transactions."):
		return domain.ErrConflict
	case isForeignKey(err):
		return domain.ErrNotFound
	}
	return err
}

func (r *TransactionRepository) putSplits(ctx context.Context, t transaction.Transaction) error {
	if _, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM transaction_splits WHERE transaction_id = ?`, t.ID); err != nil {
		return err
	}
	for i, s := range t.Splits {
		if _, err := r.db.Exec(ctx).ExecContext(ctx,
			`INSERT INTO transaction_splits (transaction_id, position, envelope_id, amount, base_amount, note) VALUES (?, ?, ?, ?, ?, ?)`,
			t.ID, i, nullString(string(s.EnvelopeID)), s.Amount, nullBase(t, s.BaseAmount), s.Note); err != nil {
			return err
		}
	}
	return nil
}

func (r *TransactionRepository) Create(ctx context.Context, t transaction.Transaction) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		_, err := r.db.Exec(ctx).ExecContext(ctx,
			`INSERT INTO transactions (id, household_id, wallet_id, date, amount, currency, base_amount, rate_pending, fx_rate,
			   payee_id, envelope_id, kind, status, source, tags, note, external_id, transfer_pair_id, transfer_wallet_id,
			   created_by_kind, created_by_id, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.HouseholdID, t.WalletID, t.Date.String(), t.Amount, t.Currency, nullBase(t, t.BaseAmount), t.RatePending,
			nullString(t.FxRate), nullString(string(t.PayeeID)), nullString(string(t.EnvelopeID)), t.Kind, t.Status, t.Source,
			stringsJSON(t.Tags), t.Note, nullString(t.ExternalID), nullString(string(t.TransferPairID)),
			nullString(string(t.TransferWalletID)), string(t.CreatedBy.Kind), t.CreatedBy.ID, toMicros(t.CreatedAt), toMicros(t.UpdatedAt))
		if err != nil {
			return mapTxWriteError(err)
		}
		return r.putSplits(ctx, t)
	})
}

func (r *TransactionRepository) Update(ctx context.Context, t transaction.Transaction) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		res, err := r.db.Exec(ctx).ExecContext(ctx,
			`UPDATE transactions SET date = ?, amount = ?, base_amount = ?, rate_pending = ?, fx_rate = ?, payee_id = ?,
			   envelope_id = ?, kind = ?, status = ?, tags = ?, note = ?, external_id = ?, transfer_pair_id = ?,
			   transfer_wallet_id = ?, updated_at = ?
			 WHERE id = ? AND household_id = ?`,
			t.Date.String(), t.Amount, nullBase(t, t.BaseAmount), t.RatePending, nullString(t.FxRate), nullString(string(t.PayeeID)),
			nullString(string(t.EnvelopeID)), t.Kind, t.Status, stringsJSON(t.Tags), t.Note, nullString(t.ExternalID),
			nullString(string(t.TransferPairID)), nullString(string(t.TransferWalletID)), toMicros(t.UpdatedAt), t.ID, t.HouseholdID)
		if err != nil {
			return mapTxWriteError(err)
		}
		if err := affected(res); err != nil {
			return err
		}
		// The splits belong to the stored row, whose rate state was just set.
		return r.putSplits(ctx, t)
	})
}

func (r *TransactionRepository) Delete(ctx context.Context, householdID, id domain.ID) error {
	res, err := r.db.Exec(ctx).ExecContext(ctx, `DELETE FROM transactions WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	return affected(res)
}

// query reads transactions and their splits.
func (r *TransactionRepository) query(ctx context.Context, sqlText string, args ...any) ([]transaction.Transaction, error) {
	rows, err := r.db.Query(ctx).QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	out := []transaction.Transaction{}
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.withSplits(ctx, out)
}

func (r *TransactionRepository) withSplits(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, error) {
	if len(txs) == 0 {
		return txs, nil
	}
	index := make(map[domain.ID]int, len(txs))
	ids := make([]domain.ID, len(txs))
	for i, t := range txs {
		index[t.ID], ids[i] = i, t.ID
	}
	for start := 0; start < len(ids); start += idsChunk {
		chunk := ids[start:min(start+idsChunk, len(ids))]
		rows, err := r.db.Query(ctx).QueryContext(ctx,
			`SELECT transaction_id, envelope_id, amount, base_amount, note FROM transaction_splits
			 WHERE transaction_id IN (`+placeholders(len(chunk))+`) ORDER BY transaction_id, position`, idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				id       domain.ID
				s        transaction.Split
				envelope sql.NullString
				base     sql.NullInt64
			)
			if err := rows.Scan(&id, &envelope, &s.Amount, &base, &s.Note); err != nil {
				rows.Close()
				return nil, err
			}
			s.EnvelopeID, s.BaseAmount = domain.ID(envelope.String), base.Int64
			i := index[id]
			txs[i].Splits = append(txs[i].Splits, s)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return txs, nil
}

func (r *TransactionRepository) ByID(ctx context.Context, householdID, id domain.ID) (transaction.Transaction, error) {
	found, err := r.query(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.id = ? AND t.household_id = ?`, id, householdID)
	if err != nil {
		return transaction.Transaction{}, err
	}
	if len(found) == 0 {
		return transaction.Transaction{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *TransactionRepository) ByPair(ctx context.Context, householdID, pairID domain.ID) ([]transaction.Transaction, error) {
	if pairID == "" {
		return []transaction.Transaction{}, nil
	}
	return r.query(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.transfer_pair_id = ? AND t.household_id = ? ORDER BY t.id`,
		pairID, householdID)
}

var errBadTxCursor = domain.ValidationError{Field: "cursor", Message: "is not a valid cursor of this query"}

type txCursorJSON struct {
	F  string    `json:"f"`
	D  string    `json:"d"`
	ID domain.ID `json:"id"`
}

func txFingerprint(f usecase.TransactionFilter) string {
	ids := slices.Clone(f.WalletIDs)
	slices.Sort(ids)
	canon, _ := json.Marshal([]any{f.HouseholdID, ids, f.PayeeID, f.EnvelopeID, f.From.String(), f.To.String(), f.Text,
		f.Kind, f.Status, f.Tag})
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

// likePattern makes text a LIKE pattern matching it anywhere, with the
// wildcards in it escaped.
func likePattern(text string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.ToLower(text)) + "%"
}

func (r *TransactionRepository) List(ctx context.Context, f usecase.TransactionFilter, p usecase.Page) ([]transaction.Transaction, string, error) {
	fp := txFingerprint(f)
	var after *txCursorJSON
	if p.Cursor != "" {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(p.Cursor)
		var c txCursorJSON
		if err != nil || json.Unmarshal(raw, &c) != nil || c.F != fp || c.ID == "" {
			return nil, "", errBadTxCursor
		}
		after = &c
	}
	if len(f.WalletIDs) == 0 {
		return []transaction.Transaction{}, "", nil
	}
	where := []string{"t.household_id = ?", "t.wallet_id IN (" + placeholders(len(f.WalletIDs)) + ")"}
	args := append([]any{f.HouseholdID}, idArgs(f.WalletIDs)...)
	add := func(cond string, a ...any) {
		where, args = append(where, cond), append(args, a...)
	}
	if f.PayeeID != "" {
		add("t.payee_id = ?", f.PayeeID)
	}
	if f.EnvelopeID != "" {
		add("(t.envelope_id = ? OR EXISTS (SELECT 1 FROM transaction_splits s WHERE s.transaction_id = t.id AND s.envelope_id = ?))",
			f.EnvelopeID, f.EnvelopeID)
	}
	if !f.From.IsZero() {
		add("t.date >= ?", f.From.String())
	}
	if !f.To.IsZero() {
		add("t.date <= ?", f.To.String())
	}
	if f.Kind != "" {
		add("t.kind = ?", f.Kind)
	}
	if f.Status != "" {
		add("t.status = ?", f.Status)
	}
	if f.Tag != "" {
		add("EXISTS (SELECT 1 FROM json_each(t.tags) WHERE json_each.value = ?)", f.Tag)
	}
	if f.Text != "" {
		pat := likePattern(f.Text)
		add(`(fold(t.note) LIKE ? ESCAPE '\' OR fold(COALESCE(p.name, '')) LIKE ? ESCAPE '\')`, pat, pat)
	}
	if after != nil {
		add("(t.date < ? OR (t.date = ? AND t.id < ?))", after.D, after.D, after.ID)
	}
	args = append(args, p.Limit+1)
	items, err := r.query(ctx, `SELECT `+txColumns+` FROM transactions t LEFT JOIN payees p ON p.id = t.payee_id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY t.date DESC, t.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > p.Limit {
		items = items[:p.Limit]
		last := items[len(items)-1]
		raw, _ := json.Marshal(txCursorJSON{F: fp, D: last.Date.String(), ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return items, next, nil
}

func (r *TransactionRepository) Balances(ctx context.Context, householdID domain.ID, walletIDs []domain.ID) (map[domain.ID]usecase.WalletBalance, error) {
	out := make(map[domain.ID]usecase.WalletBalance, len(walletIDs))
	for _, id := range walletIDs {
		out[id] = usecase.WalletBalance{}
	}
	for start := 0; start < len(walletIDs); start += idsChunk {
		chunk := walletIDs[start:min(start+idsChunk, len(walletIDs))]
		rows, err := r.db.Query(ctx).QueryContext(ctx,
			`SELECT wallet_id, SUM(amount), SUM(CASE WHEN status <> 'pending' THEN amount ELSE 0 END),
			        COALESCE(SUM(base_amount), 0), SUM(rate_pending)
			 FROM transactions WHERE household_id = ? AND wallet_id IN (`+placeholders(len(chunk))+`) GROUP BY wallet_id`,
			append([]any{householdID}, idArgs(chunk)...)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				id domain.ID
				b  usecase.WalletBalance
			)
			if err := rows.Scan(&id, &b.Total, &b.Cleared, &b.Base, &b.Pending); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = b
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

func (r *TransactionRepository) ListRatePending(ctx context.Context, householdID domain.ID, limit int) ([]transaction.Transaction, error) {
	if householdID == "" {
		return r.query(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.rate_pending = 1 ORDER BY t.id LIMIT ?`, limit)
	}
	return r.query(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.rate_pending = 1 AND t.household_id = ? ORDER BY t.id LIMIT ?`,
		householdID, limit)
}
