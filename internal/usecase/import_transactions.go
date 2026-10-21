package usecase

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/imports"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// Preview limits of a dry run.
const (
	DefaultPreviewRows = 20
	MaxPreviewRows     = 100
)

// maxNote is the longest note of a transaction.
const maxNote = 1000

// ImportInput is a batch to import into one wallet.
type ImportInput struct {
	WalletID  string
	Source    imports.Source
	SourceRef string
	Preset    string
	Rows      []ImportedTransaction
	// RowErrors are the rows the source could not read; they count as
	// failed rows of the batch.
	RowErrors []imports.RowError
	// DryRun checks the batch and stores nothing; PreviewLimit rows of it
	// come back (0 means DefaultPreviewRows).
	DryRun       bool
	PreviewLimit int
}

// ImportRowStatus is what an import does with a row.
type ImportRowStatus string

const (
	ImportRowNew       ImportRowStatus = "new"
	ImportRowDuplicate ImportRowStatus = "duplicate"
)

// ImportPreviewRow is one row of a dry run as it would be imported.
type ImportPreviewRow struct {
	Row        int
	Date       domain.Date
	Amount     int64 // minor units of Currency, the wallet's
	Currency   string
	PayeeRaw   string
	Memo       string
	ExternalID string
	Kind       transaction.Kind
	Status     ImportRowStatus
	PayeeID    domain.ID
	PayeeName  string
	EnvelopeID domain.ID
}

// ImportOutcome is what an import did, or for a dry run would do. The
// batch has no id after a dry run.
type ImportOutcome struct {
	Batch   imports.Batch
	Preview []ImportPreviewRow
	Matches []AutoMatch
}

// importRow is a row on its way into the wallet.
type importRow struct {
	row       int
	tx        transaction.Transaction
	payeeRaw  string
	memo      string
	payeeName string
	duplicate bool
}

// ImportTransactions brings a batch of transactions from any source into a
// wallet (FR-800...FR-806): it skips the external ids the wallet has, sets
// payees and envelopes from the payee rules, stores the transactions and
// the batch in one database transaction and then links the new
// transactions to subscription charges.
type ImportTransactions struct{ d Deps }

func NewImportTransactions(d Deps) *ImportTransactions { return &ImportTransactions{d: d} }

func (uc *ImportTransactions) Do(ctx context.Context, p Principal, householdID domain.ID, in ImportInput) (ImportOutcome, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return ImportOutcome{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
		return ImportOutcome{}, err
	}
	w, err := uc.d.writableWallet(ctx, s, p, domain.ID(strings.TrimSpace(in.WalletID)))
	if err != nil {
		return ImportOutcome{}, err
	}
	if !in.Source.Valid() {
		return ImportOutcome{}, domain.ValidationError{Field: "source", Message: "must be csv or connection"}
	}
	total := len(in.Rows) + len(in.RowErrors)
	switch {
	case total == 0:
		return ImportOutcome{}, domain.ValidationError{Field: "file", Message: "has no transactions"}
	case total > imports.MaxRows:
		return ImportOutcome{}, domain.ValidationError{Field: "file", Message: "has more than " + strconv.Itoa(imports.MaxRows) + " rows"}
	}
	payees, err := uc.d.Payees.ListByHousehold(ctx, householdID)
	if err != nil {
		return ImportOutcome{}, err
	}
	now := uc.d.Clock.Now()
	rowErrs := slices.Clone(in.RowErrors)
	rows := []importRow{}
	occurrences := map[string]int{}
	for _, r := range in.Rows {
		ir, rerr := uc.prepare(r, w, s.Household, p, payees, occurrences, now)
		if rerr != nil {
			rowErrs = append(rowErrs, *rerr)
			continue
		}
		rows = append(rows, ir)
	}
	if err := uc.markDuplicates(ctx, householdID, w.ID, rows); err != nil {
		return ImportOutcome{}, err
	}
	for i := range rows {
		if err := uc.setEnvelope(ctx, &rows[i], payees); err != nil {
			return ImportOutcome{}, err
		}
	}
	batch := imports.Batch{HouseholdID: householdID, WalletID: w.ID, Source: in.Source, SourceRef: strings.TrimSpace(in.SourceRef),
		Preset: in.Preset, Status: imports.Completed, Rows: total, CreatedBy: p.Actor(), CreatedAt: now}
	if in.DryRun {
		return uc.preview(batch, rows, rowErrs, in.PreviewLimit), nil
	}

	// Rates are looked up before the database transaction (NFR-080).
	fresh := []importRow{}
	quotes := map[domain.Date]*FxQuote{}
	for _, r := range rows {
		if r.duplicate {
			fresh = append(fresh, r)
			continue
		}
		t, err := uc.withRate(ctx, s.Household, r.tx, quotes)
		var ve domain.ValidationError
		if errors.As(err, &ve) {
			rowErrs = append(rowErrs, imports.RowError{Row: r.row, Field: ve.Field, Message: ve.Message})
			continue
		}
		if err != nil {
			return ImportOutcome{}, err
		}
		r.tx = t
		fresh = append(fresh, r)
	}
	rows = fresh

	var created []domain.ID
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		if _, err := uc.d.writableWallet(ctx, s, p, w.ID); err != nil {
			return err
		}
		if err := uc.markDuplicates(ctx, householdID, w.ID, rows); err != nil {
			return err
		}
		created = nil
		dups := 0
		for _, r := range rows {
			if r.duplicate {
				dups++
				continue
			}
			err := insertTransaction(ctx, uc.d, s.Household.BaseCurrency, r.tx, p.Actor())
			var ce domain.ConflictError
			if errors.As(err, &ce) && ce.Reason == domain.ConflictDuplicateExternalID {
				dups++
				continue
			}
			if err != nil {
				return err
			}
			created = append(created, r.tx.ID)
		}
		sortRowErrors(rowErrs)
		batch.ID = uc.d.IDs.NewID()
		batch.Created, batch.Duplicates, batch.Failed = len(created), dups, len(rowErrs)
		batch.Errors = imports.KeepErrors(rowErrs)
		if err := uc.d.ImportBatches.Create(ctx, batch); err != nil {
			return err
		}
		if err := uc.d.ImportBatches.AddTransactions(ctx, batch.ID, created); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "import.created", audit.ResourceImport, batch.ID, batchChanges(batch))
	})
	if err != nil {
		return ImportOutcome{}, err
	}
	out := ImportOutcome{Batch: batch, Matches: []AutoMatch{}}
	out.Matches, out.Batch = uc.autoMatch(ctx, p, s, batch, created)
	return out, nil
}

func batchChanges(b imports.Batch) audit.Changes {
	c := audit.Changes{}.Set("wallet_id", string(b.WalletID)).Set("source", string(b.Source)).Set("rows", b.Rows).
		Set("created", b.Created).Set("duplicates", b.Duplicates).Set("failed", b.Failed)
	if b.SourceRef != "" {
		c.Set("source_ref", b.SourceRef)
	}
	if b.Preset != "" {
		c.Set("preset", b.Preset)
	}
	return c
}

func sortRowErrors(errs []imports.RowError) {
	slices.SortStableFunc(errs, func(a, b imports.RowError) int { return cmp.Compare(a.Row, b.Row) })
}

// prepare turns a row into a transaction, or says why it cannot.
func (uc *ImportTransactions) prepare(r ImportedTransaction, w wallet.Wallet, h household.Household, p Principal,
	payees []payee.Payee, occurrences map[string]int, now time.Time) (importRow, *imports.RowError) {
	fail := func(field, msg string) (importRow, *imports.RowError) {
		return importRow{}, &imports.RowError{Row: r.Row, Field: field, Message: msg}
	}
	if r.Date.IsZero() {
		return fail("date", "is required")
	}
	if c := strings.ToUpper(strings.TrimSpace(r.Currency)); c != "" && c != w.Currency {
		return fail("currency", "is "+c+", not the wallet's "+w.Currency)
	}
	raw := strings.TrimPrefix(strings.TrimSpace(r.Amount), "+")
	if raw == "" {
		return fail("amount", "is required")
	}
	m, err := money.Parse("amount", raw, w.Currency)
	if err != nil {
		var ve domain.ValidationError
		if errors.As(err, &ve) {
			return fail("amount", ve.Message)
		}
		return fail("amount", "is not an amount")
	}
	if m.Amount == 0 {
		return fail("amount", "must not be zero")
	}
	payeeRaw, memo := strings.Join(strings.Fields(r.PayeeRaw), " "), strings.Join(strings.Fields(r.Memo), " ")
	ext := strings.TrimSpace(r.ExternalID)
	if utf8.RuneCountInString(ext) > imports.MaxExternalID {
		return fail("external_id", "must be at most 200 characters")
	}
	if ext == "" {
		text := imports.NormalizeText(payeeRaw + " " + memo)
		key := r.Date.String() + "|" + strconv.FormatInt(m.Amount, 10) + "|" + text
		occurrences[key]++
		ext = imports.HashExternalID(r.Date, m.Amount, text, occurrences[key])
	}
	kind := transaction.Expense
	if m.Amount > 0 {
		kind = transaction.Income
	}
	note := payeeRaw
	if memo != "" && !strings.EqualFold(memo, payeeRaw) {
		note = strings.TrimPrefix(note+" · "+memo, " · ")
	}
	t := transaction.Transaction{ID: uc.d.IDs.NewID(), HouseholdID: h.ID, WalletID: w.ID, Date: r.Date, Amount: m.Amount,
		Currency: w.Currency, Kind: kind, Status: transaction.Cleared, Source: transaction.SourceImport, Tags: []string{},
		Note: truncateRunes(note, maxNote), ExternalID: ext, CreatedBy: p.Actor(), CreatedAt: now, UpdatedAt: now}.Normalize()
	if err := t.Validate(); err != nil {
		var ve domain.ValidationError
		if errors.As(err, &ve) {
			return fail(ve.Field, ve.Message)
		}
		return fail("row", "is not a valid transaction")
	}
	ir := importRow{row: r.Row, tx: t, payeeRaw: payeeRaw, memo: memo}
	pe, ok := payee.Resolve(payees, payeeRaw)
	if !ok {
		pe, ok = payee.Resolve(payees, note)
	}
	if ok {
		ir.tx.PayeeID, ir.payeeName = pe.ID, pe.Name
	}
	return ir, nil
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}

// markDuplicates marks the rows whose external id the wallet has, or an
// earlier row of the batch has.
func (uc *ImportTransactions) markDuplicates(ctx context.Context, householdID, walletID domain.ID, rows []importRow) error {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.tx.ExternalID)
	}
	existing, err := uc.d.Transactions.ExternalIDs(ctx, householdID, walletID, ids)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for i := range rows {
		ext := rows[i].tx.ExternalID
		_, stored := existing[ext]
		rows[i].duplicate = stored || seen[ext]
		seen[ext] = true
	}
	return nil
}

// setEnvelope gives an expense the default envelope of its payee, while
// the envelope exists (FR-802).
func (uc *ImportTransactions) setEnvelope(ctx context.Context, r *importRow, payees []payee.Payee) error {
	if r.tx.PayeeID == "" || r.tx.Kind != transaction.Expense {
		return nil
	}
	i := slices.IndexFunc(payees, func(p payee.Payee) bool { return p.ID == r.tx.PayeeID })
	if i < 0 || payees[i].DefaultEnvelopeID == "" {
		return nil
	}
	ok, err := uc.d.envelopeExists(ctx, r.tx.HouseholdID, payees[i].DefaultEnvelopeID)
	if err != nil || !ok {
		return err
	}
	r.tx.EnvelopeID = payees[i].DefaultEnvelopeID
	return nil
}

// withRate sets the base amount, one rate lookup per day.
func (uc *ImportTransactions) withRate(ctx context.Context, h household.Household, t transaction.Transaction,
	quotes map[domain.Date]*FxQuote) (transaction.Transaction, error) {
	q, seen := quotes[t.Date]
	if !seen {
		got, err := uc.d.fx().quote(ctx, h, t.Currency, t.Date)
		switch {
		case errors.Is(err, ErrRateUnavailable):
		case err != nil:
			return transaction.Transaction{}, err
		default:
			q = &got
		}
		quotes[t.Date] = q
	}
	if q == nil {
		return markPending(t), nil
	}
	return applyRate(t, h, *q)
}

func (uc *ImportTransactions) preview(batch imports.Batch, rows []importRow, rowErrs []imports.RowError, limit int) ImportOutcome {
	if limit <= 0 {
		limit = DefaultPreviewRows
	}
	sortRowErrors(rowErrs)
	out := ImportOutcome{Preview: []ImportPreviewRow{}, Matches: []AutoMatch{}}
	for _, r := range rows {
		if r.duplicate {
			batch.Duplicates++
		} else {
			batch.Created++
		}
		if len(out.Preview) < limit {
			status := ImportRowNew
			if r.duplicate {
				status = ImportRowDuplicate
			}
			out.Preview = append(out.Preview, ImportPreviewRow{Row: r.row, Date: r.tx.Date, Amount: r.tx.Amount, Currency: r.tx.Currency,
				PayeeRaw: r.payeeRaw, Memo: r.memo, ExternalID: r.tx.ExternalID, Kind: r.tx.Kind, Status: status,
				PayeeID: r.tx.PayeeID, PayeeName: r.payeeName, EnvelopeID: r.tx.EnvelopeID})
		}
	}
	batch.Failed, batch.Errors = len(rowErrs), imports.KeepErrors(rowErrs)
	out.Batch = batch
	return out
}

// autoMatch links the new transactions to subscription charges when the
// principal may (AD-806) and stores how many it linked. A failure is
// logged: the import stands.
func (uc *ImportTransactions) autoMatch(ctx context.Context, p Principal, s HouseholdScope, batch imports.Batch,
	created []domain.ID) ([]AutoMatch, imports.Batch) {
	if len(created) == 0 || uc.d.Charges == nil || uc.d.Subscriptions == nil ||
		requireScope(s, bot.ScopeSubscriptionsWrite) != nil || requireScope(s, bot.ScopeTransactionsRead) != nil {
		return []AutoMatch{}, batch
	}
	matches, err := NewAutoMatchTransactions(uc.d).Do(ctx, p, batch.HouseholdID, created)
	if err != nil {
		uc.d.log().WarnContext(ctx, "matching imported transactions", "import_id", string(batch.ID), "error", err.Error())
	}
	if len(matches) == 0 {
		return []AutoMatch{}, batch
	}
	batch.Matched = len(matches)
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error { return uc.d.ImportBatches.Update(ctx, batch) })
	if err != nil {
		uc.d.log().WarnContext(ctx, "storing the matches of an import", "import_id", string(batch.ID), "error", err.Error())
	}
	return matches, batch
}
