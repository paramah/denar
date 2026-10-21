package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/payee"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// sourceOf is the source of what the principal records by hand: a bot's
// entries are marked as such (FR-130).
func sourceOf(p Principal) transaction.Source {
	if p.IsBot() {
		return transaction.SourceBot
	}
	return transaction.SourceManual
}

// TransactionView is a transaction with the name of its payee. The wallet
// of the other leg of a transfer is left out when the principal does not
// see it.
type TransactionView struct {
	Transaction transaction.Transaction
	PayeeName   string
	// BaseCurrency is the household's, the currency of the base amounts.
	BaseCurrency string
}

// viewer turns transactions into views for one principal.
type viewer struct {
	payees  map[domain.ID]string
	visible map[domain.ID]bool
	base    string
}

func (d Deps) viewer(ctx context.Context, s HouseholdScope, p Principal) (viewer, error) {
	ps, err := d.Payees.ListByHousehold(ctx, s.Household.ID)
	if err != nil {
		return viewer{}, err
	}
	ws, err := d.visibleWallets(ctx, s, p)
	if err != nil {
		return viewer{}, err
	}
	v := viewer{payees: map[domain.ID]string{}, visible: map[domain.ID]bool{}, base: s.Household.BaseCurrency}
	for _, pe := range ps {
		v.payees[pe.ID] = pe.Name
	}
	for _, w := range ws {
		v.visible[w.ID] = true
	}
	return v, nil
}

func (v viewer) view(t transaction.Transaction) TransactionView {
	if t.TransferWalletID != "" && !v.visible[t.TransferWalletID] {
		t.TransferWalletID = ""
	}
	return TransactionView{Transaction: t, PayeeName: v.payees[t.PayeeID], BaseCurrency: v.base}
}

func (d Deps) viewOne(ctx context.Context, s HouseholdScope, p Principal, t transaction.Transaction) (TransactionView, error) {
	v, err := d.viewer(ctx, s, p)
	if err != nil {
		return TransactionView{}, err
	}
	return v.view(t), nil
}

// txChanges describes a transaction for the audit log.
func txChanges(t transaction.Transaction, baseCurrency string) audit.Changes {
	c := audit.Changes{}.Set("wallet_id", string(t.WalletID)).Set("date", t.Date.String()).
		Set("amount", formatMinor(t.Amount, t.Currency)).Set("kind", string(t.Kind)).Set("status", string(t.Status)).
		Set("source", string(t.Source))
	if !t.RatePending {
		c.Set("base_amount", formatMinor(t.BaseAmount, baseCurrency))
	}
	for field, v := range map[string]string{"payee_id": string(t.PayeeID), "envelope_id": string(t.EnvelopeID),
		"note": t.Note, "external_id": t.ExternalID, "transfer_pair_id": string(t.TransferPairID), "tags": strings.Join(t.Tags, " ")} {
		if v != "" {
			c.Set(field, v)
		}
	}
	if len(t.Splits) > 0 {
		c.Set("splits", splitsText(t))
	}
	return c
}

func splitsText(t transaction.Transaction) string {
	parts := make([]string, len(t.Splits))
	for i, s := range t.Splits {
		parts[i] = fmt.Sprintf("%s:%s", cmpOr(string(s.EnvelopeID), "-"), formatMinor(s.Amount, t.Currency))
	}
	return strings.Join(parts, ", ")
}

func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// insertTransaction stores a new transaction and records it. Call it in a
// transaction.
func insertTransaction(ctx context.Context, d Deps, baseCurrency string, t transaction.Transaction, actor domain.Actor) error {
	if err := d.Transactions.Create(ctx, t); err != nil {
		return err
	}
	return d.record(ctx, t.HouseholdID, actor, "transaction.created", audit.ResourceTransaction, t.ID, txChanges(t, baseCurrency))
}

// writableWallet is a wallet the principal sees and may write to.
func (d Deps) writableWallet(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (wallet.Wallet, error) {
	w, err := d.visibleWallet(ctx, s, p, id)
	if err != nil {
		return wallet.Wallet{}, err
	}
	if w.Archived() {
		return wallet.Wallet{}, domain.ConflictError{Reason: domain.ConflictWalletArchived}
	}
	return w, nil
}

// SplitInput is one line of a split as clients write it.
type SplitInput struct {
	EnvelopeID string
	Amount     MoneyInput
	Note       string
}

func parseSplits(in []SplitInput, currency string) ([]transaction.Split, error) {
	out := make([]transaction.Split, len(in))
	var errs []error
	for i, s := range in {
		m, err := parseMoney(fmt.Sprintf("splits[%d].amount", i), s.Amount, currency)
		errs = append(errs, err)
		out[i] = transaction.Split{EnvelopeID: domain.ID(strings.TrimSpace(s.EnvelopeID)), Amount: m.Amount, Note: s.Note}
	}
	return out, errors.Join(errs...)
}

// payeeChoice is how a transaction names its payee: by id, by name (found
// or created, FR-140), or not at all.
type payeeChoice struct {
	set  bool // for an update: the payee changes
	id   domain.ID
	name string
}

func choosePayee(id, name string) (payeeChoice, error) {
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if id != "" && name != "" {
		return payeeChoice{}, domain.ValidationError{Field: "payee_name", Message: "give payee_id or payee_name, not both"}
	}
	return payeeChoice{set: true, id: domain.ID(id), name: name}, nil
}

// resolve returns the payee the choice names, creating it by name when the
// household has none that matches. Call it in a transaction.
func (c payeeChoice) resolve(ctx context.Context, d Deps, householdID domain.ID, actor domain.Actor) (payee.Payee, error) {
	switch {
	case c.id != "":
		p, err := d.Payees.ByID(ctx, householdID, c.id)
		if errors.Is(err, domain.ErrNotFound) {
			return payee.Payee{}, domain.ValidationError{Field: "payee_id", Message: "is not a payee of this household"}
		}
		return p, err
	case c.name != "":
		all, err := d.Payees.ListByHousehold(ctx, householdID)
		if err != nil {
			return payee.Payee{}, err
		}
		if p, ok := payee.Resolve(all, c.name); ok {
			return p, nil
		}
		now := d.Clock.Now()
		p := payee.Payee{ID: d.IDs.NewID(), HouseholdID: householdID, Name: c.name, CreatedAt: now, UpdatedAt: now}.Normalize()
		if err := p.Validate(); err != nil {
			return payee.Payee{}, domain.ValidationError{Field: "payee_name", Message: "must be 1 to 100 characters"}
		}
		if err := d.Payees.Create(ctx, p); err != nil {
			return payee.Payee{}, err
		}
		return p, d.record(ctx, householdID, actor, "payee.created", audit.ResourcePayee, p.ID, audit.Changes{}.Set("name", p.Name))
	}
	return payee.Payee{}, nil
}

type RecordTransactionInput struct {
	WalletID   string
	Date       domain.Date // zero for today in the household's time zone
	Amount     MoneyInput
	PayeeID    string
	PayeeName  string
	EnvelopeID string
	Kind       string // expense (default) or income
	Status     string // cleared by default
	Tags       []string
	Note       string
	ExternalID string
	Splits     []SplitInput
}

// RecordTransaction adds an expense or income to a wallet (FR-130). The
// rate to the base currency is looked up first, outside the database
// transaction; without one the transaction waits for it (FR-111).
type RecordTransaction struct{ d Deps }

func NewRecordTransaction(d Deps) *RecordTransaction { return &RecordTransaction{d: d} }

func (uc *RecordTransaction) Do(ctx context.Context, p Principal, householdID domain.ID, in RecordTransactionInput) (TransactionView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return TransactionView{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
		return TransactionView{}, err
	}
	w, err := uc.d.writableWallet(ctx, s, p, domain.ID(strings.TrimSpace(in.WalletID)))
	if err != nil {
		return TransactionView{}, err
	}
	kind := transaction.Kind(cmpOr(in.Kind, string(transaction.Expense)))
	if kind == transaction.Transfer {
		return TransactionView{}, domain.ValidationError{Field: "kind", Message: "a transfer is made with the transfer operation"}
	}
	now := uc.d.Clock.Now()
	t := transaction.Transaction{ID: uc.d.IDs.NewID(), HouseholdID: householdID, WalletID: w.ID, Date: in.Date,
		Currency: w.Currency, EnvelopeID: domain.ID(strings.TrimSpace(in.EnvelopeID)), Kind: kind,
		Status: transaction.Status(cmpOr(in.Status, string(transaction.Cleared))), Source: sourceOf(p), Tags: in.Tags,
		Note: in.Note, ExternalID: in.ExternalID, CreatedBy: p.Actor(), CreatedAt: now, UpdatedAt: now}
	if t.Date.IsZero() {
		t.Date = today(s.Household, now)
	}
	m, amountErr := parseMoney("amount", in.Amount, w.Currency)
	t.Amount = m.Amount
	var splitErr error
	t.Splits, splitErr = parseSplits(in.Splits, w.Currency)
	choice, payeeErr := choosePayee(in.PayeeID, in.PayeeName)
	t = t.Normalize()
	if err := errors.Join(amountErr, splitErr, payeeErr); err != nil {
		return TransactionView{}, err
	}
	if err := t.Validate(); err != nil {
		return TransactionView{}, err
	}
	if t, err = uc.d.fx().withBase(ctx, s.Household, t); err != nil {
		return TransactionView{}, err
	}
	var view TransactionView
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
		pe, err := choice.resolve(ctx, uc.d, householdID, p.Actor())
		if err != nil {
			return err
		}
		t.PayeeID = pe.ID
		if t.Kind == transaction.Expense && t.EnvelopeID == "" && len(t.Splits) == 0 && pe.DefaultEnvelopeID != "" {
			// The payee's default envelope, while it exists (FR-221).
			ok, err := uc.d.envelopeExists(ctx, householdID, pe.DefaultEnvelopeID)
			if err != nil {
				return err
			}
			if ok {
				t.EnvelopeID = pe.DefaultEnvelopeID
			}
		}
		if err := uc.d.checkTxEnvelopes(ctx, t); err != nil {
			return err
		}
		if err := insertTransaction(ctx, uc.d, s.Household.BaseCurrency, t, p.Actor()); err != nil {
			return err
		}
		view, err = uc.d.viewOne(ctx, s, p, t)
		return err
	})
	return view, err
}

// GetTransaction returns a transaction of a wallet the principal sees.
type GetTransaction struct{ d Deps }

func NewGetTransaction(d Deps) *GetTransaction { return &GetTransaction{d: d} }

// visibleTransaction is a transaction of a wallet the principal sees;
// others are domain.ErrNotFound.
func (d Deps) visibleTransaction(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (transaction.Transaction, wallet.Wallet, error) {
	t, err := d.Transactions.ByID(ctx, s.Household.ID, id)
	if err != nil {
		return transaction.Transaction{}, wallet.Wallet{}, plainNotFound(err)
	}
	w, err := d.visibleWallet(ctx, s, p, t.WalletID)
	if err != nil {
		return transaction.Transaction{}, wallet.Wallet{}, err
	}
	return t, w, nil
}

func (uc *GetTransaction) Do(ctx context.Context, p Principal, householdID, id domain.ID) (TransactionView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return TransactionView{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return TransactionView{}, err
	}
	t, _, err := uc.d.visibleTransaction(ctx, s, p, id)
	if err != nil {
		return TransactionView{}, err
	}
	return uc.d.viewOne(ctx, s, p, t)
}

type SearchTransactionsInput struct {
	WalletID   string
	PayeeID    string
	EnvelopeID string
	From, To   domain.Date
	Text       string
	Kind       string
	Status     string
	Tag        string
	Cursor     string
	Limit      domain.Optional[int]
}

type TransactionPage struct {
	Items      []TransactionView
	NextCursor string
}

// SearchTransactions lists transactions of the wallets the principal sees,
// newest first (FR-133). Naming a wallet it does not see is not found.
type SearchTransactions struct{ d Deps }

func NewSearchTransactions(d Deps) *SearchTransactions { return &SearchTransactions{d: d} }

func (uc *SearchTransactions) Do(ctx context.Context, p Principal, householdID domain.ID, in SearchTransactionsInput) (TransactionPage, error) {
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return TransactionPage{}, err
	}
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return TransactionPage{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return TransactionPage{}, err
	}
	f := TransactionFilter{HouseholdID: householdID, PayeeID: domain.ID(in.PayeeID), EnvelopeID: domain.ID(in.EnvelopeID),
		From: in.From, To: in.To, Text: strings.TrimSpace(in.Text), Kind: transaction.Kind(in.Kind),
		Status: transaction.Status(in.Status), Tag: strings.ToLower(strings.TrimSpace(in.Tag))}
	var errs []error
	if f.Kind != "" && !f.Kind.Valid() {
		errs = append(errs, domain.ValidationError{Field: "kind", Message: "must be expense, income or transfer"})
	}
	if f.Status != "" && !f.Status.Valid() {
		errs = append(errs, domain.ValidationError{Field: "status", Message: "must be pending, cleared or reconciled"})
	}
	if !f.From.IsZero() && !f.To.IsZero() && f.To.Before(f.From) {
		errs = append(errs, domain.ValidationError{Field: "to", Message: "must not be before from"})
	}
	if utf8.RuneCountInString(f.Text) > 100 {
		errs = append(errs, domain.ValidationError{Field: "q", Message: "must be at most 100 characters"})
	}
	if err := errors.Join(errs...); err != nil {
		return TransactionPage{}, err
	}
	if in.WalletID != "" {
		w, err := uc.d.visibleWallet(ctx, s, p, domain.ID(in.WalletID))
		if err != nil {
			return TransactionPage{}, err
		}
		f.WalletIDs = []domain.ID{w.ID}
	} else {
		ws, err := uc.d.visibleWallets(ctx, s, p)
		if err != nil {
			return TransactionPage{}, err
		}
		f.WalletIDs = walletIDs(ws)
	}
	items, next, err := uc.d.Transactions.List(ctx, f, page)
	if err != nil {
		return TransactionPage{}, cursorError(err, in.Cursor)
	}
	v, err := uc.d.viewer(ctx, s, p)
	if err != nil {
		return TransactionPage{}, err
	}
	out := TransactionPage{Items: make([]TransactionView, len(items)), NextCursor: next}
	for i, t := range items {
		out.Items[i] = v.view(t)
	}
	return out, nil
}

type UpdateTransactionInput struct {
	Date       domain.Optional[domain.Date]
	Amount     domain.Optional[MoneyInput]
	PayeeID    domain.Optional[string] // "" removes the payee
	PayeeName  domain.Optional[string]
	EnvelopeID domain.Optional[string] // "" removes the envelope
	Kind       domain.Optional[string]
	Status     domain.Optional[string]
	Tags       domain.Optional[[]string]
	Note       domain.Optional[string]
	ExternalID domain.Optional[string]
	Splits     domain.Optional[[]SplitInput] // empty removes the split
}

// UpdateTransaction changes a transaction. Its wallet never changes. A
// new amount or date gets a new rate. Of a transfer leg only the date
// (of both legs), the status, the note and the tags change (AD-107).
type UpdateTransaction struct{ d Deps }

func NewUpdateTransaction(d Deps) *UpdateTransaction { return &UpdateTransaction{d: d} }

func (uc *UpdateTransaction) Do(ctx context.Context, p Principal, householdID, id domain.ID, in UpdateTransactionInput) (TransactionView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return TransactionView{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
		return TransactionView{}, err
	}
	old, w, err := uc.d.visibleTransaction(ctx, s, p, id)
	if err != nil {
		return TransactionView{}, err
	}
	if w.Archived() {
		return TransactionView{}, domain.ConflictError{Reason: domain.ConflictWalletArchived}
	}
	if old.IsTransfer() {
		return uc.updateTransfer(ctx, p, s, old, in)
	}
	t := old
	var errs []error
	if in.Date.Set {
		t.Date = in.Date.Value
	}
	if in.Amount.Set {
		m, err := parseMoney("amount", in.Amount.Value, t.Currency)
		errs = append(errs, err)
		t.Amount = m.Amount
	}
	choice := payeeChoice{}
	if in.PayeeID.Set || in.PayeeName.Set {
		c, err := choosePayee(in.PayeeID.Value, in.PayeeName.Value)
		errs = append(errs, err)
		choice = c
	}
	if in.EnvelopeID.Set {
		t.EnvelopeID = domain.ID(strings.TrimSpace(in.EnvelopeID.Value))
	}
	if in.Kind.Set {
		t.Kind = transaction.Kind(in.Kind.Value)
		if t.Kind == transaction.Transfer {
			errs = append(errs, domain.ValidationError{Field: "kind", Message: "a transfer is made with the transfer operation"})
		}
	}
	if in.Status.Set {
		t.Status = transaction.Status(in.Status.Value)
	}
	if in.Tags.Set {
		t.Tags = in.Tags.Value
	}
	if in.Note.Set {
		t.Note = in.Note.Value
	}
	if in.ExternalID.Set {
		t.ExternalID = in.ExternalID.Value
	}
	if in.Splits.Set {
		splits, err := parseSplits(in.Splits.Value, t.Currency)
		errs = append(errs, err)
		t.Splits = splits
	}
	t = t.Normalize()
	if err := errors.Join(errs...); err != nil {
		return TransactionView{}, err
	}
	if err := t.Validate(); err != nil {
		return TransactionView{}, err
	}
	if t.Amount != old.Amount || t.Date != old.Date || in.Splits.Set || old.RatePending {
		if t, err = uc.d.fx().withBase(ctx, s.Household, t); err != nil {
			return TransactionView{}, err
		}
	}
	t.UpdatedAt = uc.d.Clock.Now()
	var view TransactionView
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		if _, _, err := uc.d.visibleTransaction(ctx, s, p, id); err != nil {
			return err
		}
		if choice.set {
			pe, err := choice.resolve(ctx, uc.d, householdID, p.Actor())
			if err != nil {
				return err
			}
			t.PayeeID = pe.ID
		}
		// Only envelopes the update names are checked, so that a
		// transaction of an envelope from before stage 003 stays editable.
		changed := transaction.Transaction{HouseholdID: t.HouseholdID}
		if in.EnvelopeID.Set {
			changed.EnvelopeID = t.EnvelopeID
		}
		if in.Splits.Set {
			changed.Splits = t.Splits
		}
		if err := uc.d.checkTxEnvelopes(ctx, changed); err != nil {
			return err
		}
		if err := uc.d.Transactions.Update(ctx, t); err != nil {
			return err
		}
		if err := recordTxUpdate(ctx, uc.d, s.Household.BaseCurrency, old, t, p.Actor()); err != nil {
			return err
		}
		view, err = uc.d.viewOne(ctx, s, p, t)
		return err
	})
	return view, err
}

func recordTxUpdate(ctx context.Context, d Deps, baseCurrency string, old, t transaction.Transaction, actor domain.Actor) error {
	before, after := txChanges(old, baseCurrency), txChanges(t, baseCurrency)
	changes := audit.Changes{}
	for _, field := range []string{"date", "amount", "base_amount", "kind", "status", "payee_id", "envelope_id", "note",
		"external_id", "tags", "splits"} {
		changes.Diff(field, before[field].To, after[field].To)
	}
	if len(changes) == 0 {
		return nil
	}
	return d.record(ctx, t.HouseholdID, actor, "transaction.updated", audit.ResourceTransaction, t.ID, changes)
}

// otherLeg returns the other leg of a transfer and checks the principal
// sees its wallet.
func (d Deps) otherLeg(ctx context.Context, s HouseholdScope, p Principal, t transaction.Transaction) (transaction.Transaction, error) {
	legs, err := d.Transactions.ByPair(ctx, s.Household.ID, t.TransferPairID)
	if err != nil {
		return transaction.Transaction{}, err
	}
	i := slices.IndexFunc(legs, func(l transaction.Transaction) bool { return l.ID != t.ID })
	if i < 0 {
		return transaction.Transaction{}, fmt.Errorf("transfer %s has one leg", t.TransferPairID)
	}
	if _, err := d.visibleWallet(ctx, s, p, legs[i].WalletID); err != nil {
		return transaction.Transaction{}, domain.ForbiddenError{Reason: domain.ForbiddenWallet}
	}
	return legs[i], nil
}

func (uc *UpdateTransaction) updateTransfer(ctx context.Context, p Principal, s HouseholdScope, old transaction.Transaction, in UpdateTransactionInput) (TransactionView, error) {
	for field, set := range map[string]bool{"amount": in.Amount.Set, "payee_id": in.PayeeID.Set, "payee_name": in.PayeeName.Set,
		"envelope_id": in.EnvelopeID.Set, "kind": in.Kind.Set, "external_id": in.ExternalID.Set, "splits": in.Splits.Set} {
		if set {
			return TransactionView{}, domain.ValidationError{Field: field, Message: "cannot change on a transfer leg; delete the transfer and make it again"}
		}
	}
	t := old
	if in.Status.Set {
		t.Status = transaction.Status(in.Status.Value)
	}
	if in.Tags.Set {
		t.Tags = in.Tags.Value
	}
	if in.Note.Set {
		t.Note = in.Note.Value
	}
	var other, otherOld transaction.Transaction
	moved := in.Date.Set && in.Date.Value != old.Date
	if moved {
		t.Date = in.Date.Value
		var err error
		if otherOld, err = uc.d.otherLeg(ctx, s, p, old); err != nil {
			return TransactionView{}, err
		}
		other = otherOld
		other.Date = t.Date
	}
	t = t.Normalize()
	if err := t.Validate(); err != nil {
		return TransactionView{}, err
	}
	var err error
	if moved {
		if t, err = uc.d.fx().withBase(ctx, s.Household, t); err != nil {
			return TransactionView{}, err
		}
		if other, err = uc.d.fx().withBase(ctx, s.Household, other); err != nil {
			return TransactionView{}, err
		}
	}
	now := uc.d.Clock.Now()
	t.UpdatedAt, other.UpdatedAt = now, now
	var view TransactionView
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, s.Household.ID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		if _, _, err := uc.d.visibleTransaction(ctx, s, p, old.ID); err != nil {
			return err
		}
		if err := uc.d.Transactions.Update(ctx, t); err != nil {
			return err
		}
		if err := recordTxUpdate(ctx, uc.d, s.Household.BaseCurrency, old, t, p.Actor()); err != nil {
			return err
		}
		if moved {
			if err := uc.d.Transactions.Update(ctx, other); err != nil {
				return err
			}
			if err := recordTxUpdate(ctx, uc.d, s.Household.BaseCurrency, otherOld, other, p.Actor()); err != nil {
				return err
			}
		}
		view, err = uc.d.viewOne(ctx, s, p, t)
		return err
	})
	return view, err
}

// DeleteTransaction removes a transaction; for a transfer, both legs.
type DeleteTransaction struct{ d Deps }

func NewDeleteTransaction(d Deps) *DeleteTransaction { return &DeleteTransaction{d: d} }

func (uc *DeleteTransaction) Do(ctx context.Context, p Principal, householdID, id domain.ID) error {
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		t, w, err := uc.d.visibleTransaction(ctx, s, p, id)
		if err != nil {
			return err
		}
		doomed := []transaction.Transaction{t}
		if t.IsTransfer() {
			other, err := uc.d.otherLeg(ctx, s, p, t)
			if err != nil {
				return err
			}
			ow, err := uc.d.Wallets.ByID(ctx, householdID, other.WalletID)
			if err != nil {
				return err
			}
			if ow.Archived() {
				return domain.ConflictError{Reason: domain.ConflictWalletArchived}
			}
			doomed = append(doomed, other)
		}
		if w.Archived() {
			return domain.ConflictError{Reason: domain.ConflictWalletArchived}
		}
		for _, t := range doomed {
			if err := uc.d.unlinkCharge(ctx, s.Household, p.Actor(), t.ID); err != nil {
				return err
			}
			if err := uc.d.Transactions.Delete(ctx, householdID, t.ID); err != nil {
				return err
			}
			changes := audit.Changes{}
			for field, c := range txChanges(t, s.Household.BaseCurrency) {
				changes[field] = audit.Change{From: c.To}
			}
			if err := uc.d.record(ctx, householdID, p.Actor(), "transaction.deleted", audit.ResourceTransaction, t.ID, changes); err != nil {
				return err
			}
		}
		return nil
	})
}
