package usecase

import (
	"cmp"
	"context"
	"errors"
	"math/big"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/transaction"
)

// ChargeView is a charge with the subscription it comes from.
type ChargeView struct {
	Charge           subscription.Charge
	SubscriptionName string
	Vendor           string
	WalletID         domain.ID
}

func chargeView(c subscription.Charge, s subscription.Subscription) ChargeView {
	return ChargeView{Charge: c, SubscriptionName: s.Name, Vendor: s.Vendor, WalletID: s.WalletID}
}

// visibleCharge returns a charge of a subscription the principal sees.
func (d Deps) visibleCharge(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (subscription.Charge, subscription.Subscription, error) {
	c, err := d.Charges.ByID(ctx, s.Household.ID, id)
	if err != nil {
		return subscription.Charge{}, subscription.Subscription{}, plainNotFound(err)
	}
	sub, err := d.visibleSubscription(ctx, s, p, c.SubscriptionID)
	if err != nil {
		return subscription.Charge{}, subscription.Subscription{}, err
	}
	return c, sub, nil
}

func chargeChanges(old, c subscription.Charge, via string) audit.Changes {
	ch := audit.Changes{}.Diff("status", string(old.Status), string(c.Status)).Diff("transaction_id", string(old.TransactionID), string(c.TransactionID)).
		Set("due_date", c.DueDate.String()).Set("subscription_id", string(c.SubscriptionID))
	if via != "" {
		ch.Set("via", via)
	}
	return ch
}

// setChargeStatus stores the charge's new status and transaction and
// records it; call it in a transaction.
func (d Deps) setChargeStatus(ctx context.Context, actor domain.Actor, old subscription.Charge, status subscription.ChargeStatus,
	txID domain.ID, action, via string, extra audit.Changes) (subscription.Charge, error) {
	c := old
	c.Status, c.TransactionID, c.UpdatedAt = status, txID, d.Clock.Now()
	if err := d.Charges.Update(ctx, c); err != nil {
		return subscription.Charge{}, err
	}
	changes := chargeChanges(old, c, via)
	for k, v := range extra {
		changes[k] = v
	}
	return c, d.record(ctx, c.HouseholdID, actor, action, audit.ResourceCharge, c.ID, changes)
}

type ListChargesInput struct {
	SubscriptionID string
	Statuses       []string
	From, To       domain.Date
	Cursor         string
	Limit          domain.Optional[int]
}

type ChargePage struct {
	Items      []ChargeView
	NextCursor string
}

// ListCharges lists the charges of the subscriptions the principal sees,
// by due date.
type ListCharges struct{ d Deps }

func NewListCharges(d Deps) *ListCharges { return &ListCharges{d: d} }

func (uc *ListCharges) Do(ctx context.Context, p Principal, householdID domain.ID, in ListChargesInput) (ChargePage, error) {
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return ChargePage{}, err
	}
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return ChargePage{}, err
	}
	f := ChargeFilter{HouseholdID: householdID, From: in.From, To: in.To}
	var errs []error
	for _, st := range in.Statuses {
		cs := subscription.ChargeStatus(st)
		if !cs.Valid() {
			errs = append(errs, domain.ValidationError{Field: "status", Message: "must be expected, matched, missed, skipped or posted"})
			break
		}
		f.Statuses = append(f.Statuses, cs)
	}
	if !in.From.IsZero() && !in.To.IsZero() && in.To.Before(in.From) {
		errs = append(errs, domain.ValidationError{Field: "to", Message: "must not be before from"})
	}
	if err := errors.Join(errs...); err != nil {
		return ChargePage{}, err
	}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return ChargePage{}, err
	}
	byID := map[domain.ID]subscription.Subscription{}
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	if in.SubscriptionID != "" {
		sub, ok := byID[domain.ID(in.SubscriptionID)]
		if !ok {
			return ChargePage{}, domain.ErrNotFound
		}
		f.SubscriptionIDs = []domain.ID{sub.ID}
	} else {
		f.SubscriptionIDs = subscriptionIDs(subs)
	}
	items, next, err := uc.d.Charges.List(ctx, f, page)
	if err != nil {
		return ChargePage{}, cursorError(err, in.Cursor)
	}
	out := ChargePage{Items: make([]ChargeView, len(items)), NextCursor: next}
	for i, c := range items {
		out.Items[i] = chargeView(c, byID[c.SubscriptionID])
	}
	return out, nil
}

// GetCharge returns one charge of a subscription the principal sees.
type GetCharge struct{ d Deps }

func NewGetCharge(d Deps) *GetCharge { return &GetCharge{d: d} }

func (uc *GetCharge) Do(ctx context.Context, p Principal, householdID, id domain.ID) (ChargeView, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return ChargeView{}, err
	}
	c, sub, err := uc.d.visibleCharge(ctx, s, p, id)
	if err != nil {
		return ChargeView{}, err
	}
	return chargeView(c, sub), nil
}

// converter turns charge amounts into other currencies at the rates of
// their due dates, through the household's base currency (AD-401). Rates
// are looked up once per currency and day.
type converter struct {
	d      Deps
	h      household.Household
	quotes map[string]*big.Rat // "USD/2026-10-31" → rate to base; nil: unavailable
}

func (d Deps) converter(h household.Household) *converter {
	return &converter{d: d, h: h, quotes: map[string]*big.Rat{}}
}

func (cv *converter) rate(ctx context.Context, currency string, date domain.Date) (*big.Rat, error) {
	key := currency + "/" + date.String()
	if r, ok := cv.quotes[key]; ok {
		return r, nil
	}
	q, err := cv.d.fx().quote(ctx, cv.h, currency, date)
	if errors.Is(err, ErrRateUnavailable) {
		cv.quotes[key] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cv.quotes[key] = q.Rate
	return q.Rate, nil
}

// convert returns amount of from in to on the day; ok is false without a
// rate.
func (cv *converter) convert(ctx context.Context, amount int64, from, to string, date domain.Date) (int64, bool, error) {
	if from == to {
		return amount, true, nil
	}
	fromRate, err := cv.rate(ctx, from, date)
	if err != nil || fromRate == nil {
		return 0, false, err
	}
	toRate, err := cv.rate(ctx, to, date)
	if err != nil || toRate == nil {
		return 0, false, err
	}
	fc, ok1 := money.LookupCurrency(from)
	tc, ok2 := money.LookupCurrency(to)
	if !ok1 || !ok2 {
		return 0, false, nil
	}
	m, err := money.Convert(money.Money{Amount: amount, Currency: fc}, new(big.Rat).Quo(fromRate, toRate), tc)
	if err != nil {
		return 0, false, nil
	}
	return m.Amount, true, nil
}

// candidate describes a transaction for scoring against a charge; ok is
// false for what cannot pay a charge: a transfer, an income, a refund.
func (cv *converter) candidate(ctx context.Context, c subscription.Charge, t transaction.Transaction, payeeName string) (subscription.Candidate, bool, error) {
	if t.Kind != transaction.Expense || t.Amount >= 0 {
		return subscription.Candidate{}, false, nil
	}
	expected, _, err := cv.convert(ctx, c.Amount, c.Currency, t.Currency, c.DueDate)
	if err != nil {
		return subscription.Candidate{}, false, err
	}
	return subscription.Candidate{WalletID: t.WalletID, Date: t.Date, Amount: -t.Amount, Expected: expected,
		Text: payeeName + " " + t.Note}, true, nil
}

// TransactionSuggestion is a transaction that may pay a charge.
type TransactionSuggestion struct {
	Transaction TransactionView
	Score       int
}

// ChargeSuggestion is a charge a transaction may pay.
type ChargeSuggestion struct {
	Charge ChargeView
	Score  int
}

// transactionCandidates scores the transactions of the wallets the
// principal sees within the charge's date window that no other charge
// holds, best first.
func (d Deps) transactionCandidates(ctx context.Context, s HouseholdScope, p Principal, cv *converter, c subscription.Charge,
	sub subscription.Subscription) ([]TransactionSuggestion, error) {
	ws, err := d.visibleWallets(ctx, s, p)
	if err != nil {
		return nil, err
	}
	w := sub.Match.DateWindowDays
	txs, _, err := d.Transactions.List(ctx, TransactionFilter{HouseholdID: s.Household.ID, WalletIDs: walletIDs(ws),
		From: c.DueDate.AddDays(-w), To: c.DueDate.AddDays(w), Kind: transaction.Expense}, Page{Limit: MaxPageLimit})
	if err != nil {
		return nil, err
	}
	v, err := d.viewer(ctx, s, p)
	if err != nil {
		return nil, err
	}
	out := []TransactionSuggestion{}
	for _, t := range txs {
		other, err := d.Charges.ByTransaction(ctx, s.Household.ID, t.ID)
		if found, err := ignoreNotFound(err); err != nil {
			return nil, err
		} else if found && other.ID != c.ID {
			continue
		}
		tv := v.view(t)
		x, ok, err := cv.candidate(ctx, c, t, tv.PayeeName)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if score, ok := subscription.MatchScore(sub, c, x); ok {
			out = append(out, TransactionSuggestion{Transaction: tv, Score: score})
		}
	}
	slices.SortStableFunc(out, func(a, b TransactionSuggestion) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), b.Transaction.Transaction.Date.Compare(a.Transaction.Transaction.Date),
			cmp.Compare(a.Transaction.Transaction.ID, b.Transaction.Transaction.ID))
	})
	return out, nil
}

// suggestionWindow is how many days around a transaction its charges are
// looked for: the widest date window a subscription may have.
const suggestionWindow = subscription.MaxDateWindowDays

// chargeCandidates scores the open charges of the subscriptions the
// principal sees against the transaction, best first.
func (d Deps) chargeCandidates(ctx context.Context, s HouseholdScope, p Principal, cv *converter, t transaction.Transaction,
	payeeName string) ([]ChargeSuggestion, error) {
	subs, err := d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return nil, err
	}
	byID := map[domain.ID]subscription.Subscription{}
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	open, err := d.Charges.ListOpen(ctx, s.Household.ID, t.Date.AddDays(-suggestionWindow), t.Date.AddDays(suggestionWindow), 1000)
	if err != nil {
		return nil, err
	}
	out := []ChargeSuggestion{}
	for _, c := range open {
		sub, ok := byID[c.SubscriptionID]
		if !ok {
			continue
		}
		x, ok, err := cv.candidate(ctx, c, t, payeeName)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if score, ok := subscription.MatchScore(sub, c, x); ok {
			out = append(out, ChargeSuggestion{Charge: chargeView(c, sub), Score: score})
		}
	}
	slices.SortStableFunc(out, func(a, b ChargeSuggestion) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), a.Charge.Charge.DueDate.Compare(b.Charge.Charge.DueDate),
			cmp.Compare(a.Charge.Charge.ID, b.Charge.Charge.ID))
	})
	return out, nil
}

// MaxSuggestions is the most suggestions one call returns.
const MaxSuggestions = 20

// SuggestTransactions returns the transactions that may pay a charge,
// best first (FR-308).
type SuggestTransactions struct{ d Deps }

func NewSuggestTransactions(d Deps) *SuggestTransactions { return &SuggestTransactions{d: d} }

func (uc *SuggestTransactions) Do(ctx context.Context, p Principal, householdID, chargeID domain.ID, limit domain.Optional[int]) ([]TransactionSuggestion, error) {
	n := 5
	if limit.Set {
		if limit.Value < 1 || limit.Value > MaxSuggestions {
			return nil, domain.ValidationError{Field: "limit", Message: "must be between 1 and 20"}
		}
		n = limit.Value
	}
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead, bot.ScopeTransactionsRead)
	if err != nil {
		return nil, err
	}
	c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
	if err != nil {
		return nil, err
	}
	out, err := uc.d.transactionCandidates(ctx, s, p, uc.d.converter(s.Household), c, sub)
	if err != nil {
		return nil, err
	}
	return out[:min(n, len(out))], nil
}

// SuggestCharges returns the open charges a transaction may pay, best
// first (FR-308). Transactions recorded by hand or by a bot are never
// matched on their own (AD-402); this is how they find their charge.
type SuggestCharges struct{ d Deps }

func NewSuggestCharges(d Deps) *SuggestCharges { return &SuggestCharges{d: d} }

func (uc *SuggestCharges) Do(ctx context.Context, p Principal, householdID, transactionID domain.ID) ([]ChargeSuggestion, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead, bot.ScopeTransactionsRead)
	if err != nil {
		return nil, err
	}
	t, _, err := uc.d.visibleTransaction(ctx, s, p, transactionID)
	if err != nil {
		return nil, err
	}
	if _, err := uc.d.Charges.ByTransaction(ctx, householdID, t.ID); err == nil {
		return []ChargeSuggestion{}, nil // it pays one already
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	tv, err := uc.d.viewOne(ctx, s, p, t)
	if err != nil {
		return nil, err
	}
	out, err := uc.d.chargeCandidates(ctx, s, p, uc.d.converter(s.Household), t, tv.PayeeName)
	if err != nil {
		return nil, err
	}
	return out[:min(MaxSuggestions, len(out))], nil
}

// confirm links an open charge to a transaction the principal sees, in a
// transaction of the database.
func (d Deps) confirm(ctx context.Context, s HouseholdScope, p Principal, c subscription.Charge, txID domain.ID, via string,
	extra audit.Changes) (subscription.Charge, error) {
	if !c.Open() {
		return subscription.Charge{}, domain.ConflictError{Reason: domain.ConflictChargeState}
	}
	t, _, err := d.visibleTransaction(ctx, s, p, txID)
	if errors.Is(err, domain.ErrNotFound) {
		return subscription.Charge{}, domain.ValidationError{Field: "transaction_id", Message: "is not a transaction you can see"}
	}
	if err != nil {
		return subscription.Charge{}, err
	}
	if t.IsTransfer() || t.Kind == transaction.Income {
		return subscription.Charge{}, domain.ValidationError{Field: "transaction_id", Message: "must be an expense, not a transfer or an income"}
	}
	other, err := d.Charges.ByTransaction(ctx, s.Household.ID, t.ID)
	if found, err := ignoreNotFound(err); err != nil {
		return subscription.Charge{}, err
	} else if found && other.ID != c.ID {
		return subscription.Charge{}, domain.ConflictError{Reason: domain.ConflictTransactionLinked}
	}
	return d.setChargeStatus(ctx, p.Actor(), c, subscription.Matched, t.ID, "charge.matched", via, extra)
}

// ConfirmChargeMatch links an open charge to a transaction that pays it
// (FR-309). Any expense the principal sees may be given, whatever its
// score (AD-412); one transaction pays at most one charge.
type ConfirmChargeMatch struct{ d Deps }

func NewConfirmChargeMatch(d Deps) *ConfirmChargeMatch { return &ConfirmChargeMatch{d: d} }

func (uc *ConfirmChargeMatch) Do(ctx context.Context, p Principal, householdID, chargeID, transactionID domain.ID) (ChargeView, error) {
	var view ChargeView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite, bot.ScopeTransactionsRead)
		if err != nil {
			return err
		}
		c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
		if err != nil {
			return err
		}
		c, err = uc.d.confirm(ctx, s, p, c, domain.ID(strings.TrimSpace(string(transactionID))), "manual", nil)
		view = chargeView(c, sub)
		return err
	})
	return view, err
}

// UnmatchCharge unlinks a matched or posted charge from its transaction;
// the charge is open again, missed when its window has passed. A posted
// transaction stays (FR-309).
type UnmatchCharge struct{ d Deps }

func NewUnmatchCharge(d Deps) *UnmatchCharge { return &UnmatchCharge{d: d} }

func (uc *UnmatchCharge) Do(ctx context.Context, p Principal, householdID, chargeID domain.ID) (ChargeView, error) {
	var view ChargeView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
		if err != nil {
			return err
		}
		if !c.Linked() {
			return domain.ConflictError{Reason: domain.ConflictChargeState}
		}
		status := c.OpenStatus(sub.Match.DateWindowDays, today(s.Household, uc.d.Clock.Now()))
		c, err = uc.d.setChargeStatus(ctx, p.Actor(), c, status, "", "charge.unmatched", "manual", nil)
		view = chargeView(c, sub)
		return err
	})
	return view, err
}

// SkipCharge marks an open charge as one that will not be paid.
type SkipCharge struct{ d Deps }

func NewSkipCharge(d Deps) *SkipCharge { return &SkipCharge{d: d} }

func (uc *SkipCharge) Do(ctx context.Context, p Principal, householdID, chargeID domain.ID) (ChargeView, error) {
	var view ChargeView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
		if err != nil {
			return err
		}
		if !c.Open() {
			return domain.ConflictError{Reason: domain.ConflictChargeState}
		}
		c, err = uc.d.setChargeStatus(ctx, p.Actor(), c, subscription.Skipped, "", "charge.skipped", "", nil)
		view = chargeView(c, sub)
		return err
	})
	return view, err
}

type PostChargeInput struct {
	// WalletID is the wallet it is paid from; empty for the
	// subscription's.
	WalletID string
	// Amount is what was paid, > 0, in the wallet's currency; nil for the
	// charge's amount, converted at the rate of the due date when the
	// currencies differ.
	Amount *MoneyInput
	// Date is the day it was paid; zero for the due date.
	Date      domain.Date
	PayeeName string // empty for the vendor's or the subscription's name
	Note      string // empty for "Subskrypcja: <name>"
}

// PostedCharge is a charge with the transaction recorded from it.
type PostedCharge struct {
	Charge      ChargeView
	Transaction TransactionView
}

// PostCharge records the expense of an open charge and links it (FR-309):
// source subscription, the subscription's envelope while it exists.
type PostCharge struct{ d Deps }

func NewPostCharge(d Deps) *PostCharge { return &PostCharge{d: d} }

func (uc *PostCharge) Do(ctx context.Context, p Principal, householdID, chargeID domain.ID, in PostChargeInput) (PostedCharge, error) {
	scopes := []bot.Scope{bot.ScopeSubscriptionsWrite, bot.ScopeTransactionsWrite}
	s, err := uc.d.subscriptionScope(ctx, p, householdID, scopes...)
	if err != nil {
		return PostedCharge{}, err
	}
	c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
	if err != nil {
		return PostedCharge{}, err
	}
	if !c.Open() {
		return PostedCharge{}, domain.ConflictError{Reason: domain.ConflictChargeState}
	}
	walletID := domain.ID(cmpOr(strings.TrimSpace(in.WalletID), string(sub.WalletID)))
	if walletID == "" {
		return PostedCharge{}, domain.ValidationError{Field: "wallet_id", Message: "is required: the subscription has no wallet"}
	}
	w, err := uc.d.writableWallet(ctx, s, p, walletID)
	if err != nil {
		return PostedCharge{}, err
	}
	var paid int64
	if in.Amount != nil {
		m, err := parseMoney("amount", *in.Amount, w.Currency)
		if err != nil {
			return PostedCharge{}, err
		}
		if m.Amount <= 0 {
			return PostedCharge{}, domain.ValidationError{Field: "amount.amount", Message: "must be greater than zero: what was paid"}
		}
		paid = m.Amount
	} else {
		amount, ok, err := uc.d.converter(s.Household).convert(ctx, c.Amount, c.Currency, w.Currency, c.DueDate)
		if err != nil {
			return PostedCharge{}, err
		}
		if !ok {
			return PostedCharge{}, domain.ValidationError{Field: "amount", Message: "is required: no exchange rate from " + c.Currency +
				" to " + w.Currency + " is known for the due date"}
		}
		paid = amount
	}
	now := uc.d.Clock.Now()
	t := transaction.Transaction{ID: uc.d.IDs.NewID(), HouseholdID: householdID, WalletID: w.ID, Date: cmpDate(in.Date, c.DueDate),
		Amount: -paid, Currency: w.Currency, EnvelopeID: sub.EnvelopeID, Kind: transaction.Expense, Status: transaction.Cleared,
		Source: transaction.SourceSubscription, Note: cmpOr(strings.TrimSpace(in.Note), "Subskrypcja: "+sub.Name),
		CreatedBy: p.Actor(), CreatedAt: now, UpdatedAt: now}.Normalize()
	if err := t.Validate(); err != nil {
		return PostedCharge{}, err
	}
	if t, err = uc.d.fx().withBase(ctx, s.Household, t); err != nil {
		return PostedCharge{}, err
	}
	payeeName := strings.TrimSpace(in.PayeeName)
	if payeeName == "" {
		payeeName = sub.Name
		if sub.Vendor != subscription.VendorOther {
			payeeName = subscription.VendorName(sub.Vendor)
		}
	}
	var out PostedCharge
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, scopes...)
		if err != nil {
			return err
		}
		c, sub, err := uc.d.visibleCharge(ctx, s, p, chargeID)
		if err != nil {
			return err
		}
		if !c.Open() {
			return domain.ConflictError{Reason: domain.ConflictChargeState}
		}
		if _, err := uc.d.writableWallet(ctx, s, p, w.ID); err != nil {
			return err
		}
		pe, err := payeeChoice{set: true, name: payeeName}.resolve(ctx, uc.d, householdID, p.Actor())
		if err != nil {
			return err
		}
		t.PayeeID = pe.ID
		if t.EnvelopeID != "" {
			ok, err := uc.d.envelopeExists(ctx, householdID, t.EnvelopeID)
			if err != nil {
				return err
			}
			if !ok {
				t.EnvelopeID = ""
			}
		}
		if err := insertTransaction(ctx, uc.d, s.Household.BaseCurrency, t, p.Actor()); err != nil {
			return err
		}
		c, err = uc.d.setChargeStatus(ctx, p.Actor(), c, subscription.Posted, t.ID, "charge.posted", "", nil)
		if err != nil {
			return err
		}
		tv, err := uc.d.viewOne(ctx, s, p, t)
		out = PostedCharge{Charge: chargeView(c, sub), Transaction: tv}
		return err
	})
	return out, err
}

func cmpDate(d, fallback domain.Date) domain.Date {
	if d.IsZero() {
		return fallback
	}
	return d
}

// AutoMatch is a link AutoMatchTransactions made.
type AutoMatch struct {
	ChargeID      domain.ID
	TransactionID domain.ID
	Score         int
}

// AutoMatchTransactions links transactions to the charges they pay when
// there is no doubt (FR-310): the transaction's best charge scores at
// least subscription.AutoMatchThreshold, no other charge does, and no
// other transaction does for that charge. It is for imports (stage 008);
// what people and bots record gets suggestions instead (AD-402).
type AutoMatchTransactions struct{ d Deps }

func NewAutoMatchTransactions(d Deps) *AutoMatchTransactions { return &AutoMatchTransactions{d: d} }

func (uc *AutoMatchTransactions) Do(ctx context.Context, p Principal, householdID domain.ID, transactionIDs []domain.ID) ([]AutoMatch, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite, bot.ScopeTransactionsRead)
	if err != nil {
		return nil, err
	}
	cv := uc.d.converter(s.Household)
	out := []AutoMatch{}
	for _, id := range transactionIDs {
		t, _, err := uc.d.visibleTransaction(ctx, s, p, id)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return out, err
		}
		if _, err := uc.d.Charges.ByTransaction(ctx, householdID, t.ID); err == nil {
			continue
		} else if !errors.Is(err, domain.ErrNotFound) {
			return out, err
		}
		tv, err := uc.d.viewOne(ctx, s, p, t)
		if err != nil {
			return out, err
		}
		charges, err := uc.d.chargeCandidates(ctx, s, p, cv, t, tv.PayeeName)
		if err != nil {
			return out, err
		}
		if len(charges) == 0 || charges[0].Score < subscription.AutoMatchThreshold ||
			(len(charges) > 1 && charges[1].Score >= subscription.AutoMatchThreshold) {
			continue
		}
		best := charges[0]
		sub, err := uc.d.Subscriptions.ByID(ctx, householdID, best.Charge.Charge.SubscriptionID)
		if err != nil {
			return out, err
		}
		txs, err := uc.d.transactionCandidates(ctx, s, p, cv, best.Charge.Charge, sub)
		if err != nil {
			return out, err
		}
		rivals := 0
		for _, x := range txs {
			if x.Transaction.Transaction.ID != t.ID && x.Score >= subscription.AutoMatchThreshold {
				rivals++
			}
		}
		if rivals > 0 {
			continue
		}
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite, bot.ScopeTransactionsRead)
			if err != nil {
				return err
			}
			c, _, err := uc.d.visibleCharge(ctx, s, p, best.Charge.Charge.ID)
			if err != nil {
				return err
			}
			_, err = uc.d.confirm(ctx, s, p, c, t.ID, "auto", audit.Changes{}.Set("score", best.Score))
			return err
		})
		var ce domain.ConflictError
		if errors.As(err, &ce) {
			continue // matched meanwhile
		}
		if err != nil {
			return out, err
		}
		out = append(out, AutoMatch{ChargeID: best.Charge.Charge.ID, TransactionID: t.ID, Score: best.Score})
	}
	return out, nil
}
