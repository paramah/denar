package usecase

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/transaction"
)

// Grouping of the spending report.
const (
	SpendingByEnvelope = "envelope"
	SpendingByPayee    = "payee"
	SpendingByVendor   = "vendor"
	SpendingByWallet   = "wallet"
)

// maxSpendingDays is the longest period a spending report covers.
const maxSpendingDays = 366

// spendingScanLimit is how many transactions one report reads at most
// (AD-603); a test lowers it.
var spendingScanLimit = 10_000

type SpendingReportInput struct {
	GroupBy string // envelope, payee, vendor or wallet
	// From and To bound the period, inclusive; From defaults to the start
	// of the budget month of To, To to today in the household's time zone.
	From, To domain.Date
	// WalletID narrows the report to one wallet the principal sees.
	WalletID string
}

// SpendingGroup is what was spent in one envelope, payee, vendor or
// wallet. Key "" is the group of expenses without one.
type SpendingGroup struct {
	Key   domain.ID
	Name  string
	Count int   // transactions, or split lines for envelopes
	Total int64 // minor units of the base currency spent; refunds lower it
}

// SpendingReport sums the expenses of a period in the base currency.
type SpendingReport struct {
	GroupBy  string
	From, To domain.Date
	Currency string
	Total    int64
	Count    int // expenses counted
	Groups   []SpendingGroup
	// PendingTransactions wait for a rate and are left out (Incomplete).
	PendingTransactions int
	Incomplete          bool
	// Truncated: the period has more expenses than one report reads.
	Truncated bool
}

// SpendingReporter groups the expenses of the wallets a principal sees
// over a period by envelope, payee, vendor or wallet (FR-623). It needs
// reports:read. Transfers and income are not spending; a refund (an
// expense with a positive amount) lowers it. The vendor of an expense is
// that of the subscription whose charge it pays.
type SpendingReporter struct{ d Deps }

func NewSpendingReport(d Deps) *SpendingReporter { return &SpendingReporter{d: d} }

func (uc *SpendingReporter) Do(ctx context.Context, p Principal, householdID domain.ID, in SpendingReportInput) (SpendingReport, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return SpendingReport{}, err
	}
	if err := requireScope(s, bot.ScopeReportsRead); err != nil {
		return SpendingReport{}, err
	}
	switch in.GroupBy {
	case SpendingByEnvelope, SpendingByPayee, SpendingByVendor, SpendingByWallet:
	default:
		return SpendingReport{}, domain.ValidationError{Field: "group_by", Message: "must be envelope, payee, vendor or wallet"}
	}
	h := s.Household
	from, to := in.From, in.To
	if to.IsZero() {
		to = today(h, uc.d.Clock.Now())
	}
	if from.IsZero() {
		from = budget.MonthOf(to, h.MonthStartDay).Start(h.MonthStartDay)
	}
	if to.Before(from) {
		return SpendingReport{}, domain.ValidationError{Field: "to", Message: "must not be before from"}
	}
	if from.AddDays(maxSpendingDays - 1).Before(to) {
		return SpendingReport{}, domain.ValidationError{Field: "from", Message: "the period may be at most 366 days long"}
	}

	ws, err := uc.d.visibleWallets(ctx, s, p)
	if err != nil {
		return SpendingReport{}, err
	}
	if in.WalletID != "" {
		w, err := uc.d.visibleWallet(ctx, s, p, domain.ID(strings.TrimSpace(in.WalletID)))
		if err != nil {
			return SpendingReport{}, err
		}
		ws = ws[:0]
		ws = append(ws, w)
	}
	names := map[domain.ID]string{}
	for _, w := range ws {
		if in.GroupBy == SpendingByWallet {
			names[w.ID] = w.Name
		}
	}
	vendorOf := map[domain.ID]string{} // transaction id → vendor key
	switch in.GroupBy {
	case SpendingByPayee:
		ps, err := uc.d.Payees.ListByHousehold(ctx, h.ID)
		if err != nil {
			return SpendingReport{}, err
		}
		for _, pe := range ps {
			names[pe.ID] = pe.Name
		}
	case SpendingByEnvelope:
		es, err := uc.d.Envelopes.ListByHousehold(ctx, h.ID)
		if err != nil {
			return SpendingReport{}, err
		}
		for _, e := range es {
			names[e.ID] = e.Name
		}
	case SpendingByVendor:
		if vendorOf, err = uc.vendors(ctx, s, p); err != nil {
			return SpendingReport{}, err
		}
		for _, v := range subscription.Vendors {
			names[domain.ID(v.Key)] = v.Name
		}
	}

	out := SpendingReport{GroupBy: in.GroupBy, From: from, To: to, Currency: h.BaseCurrency, Groups: []SpendingGroup{}}
	sums := map[domain.ID]*SpendingGroup{}
	add := func(key domain.ID, spent int64) error {
		g, ok := sums[key]
		if !ok {
			g = &SpendingGroup{Key: key, Name: names[key]}
			if key != "" && g.Name == "" && in.GroupBy == SpendingByVendor {
				g.Name = subscription.VendorName(string(key))
			}
			sums[key] = g
		}
		total, ok := money.AddInt64(g.Total, spent)
		if !ok {
			return money.ErrOverflow
		}
		g.Total, g.Count = total, g.Count+1
		return nil
	}

	f := TransactionFilter{HouseholdID: h.ID, WalletIDs: walletIDs(ws), From: from, To: to, Kind: transaction.Expense}
	page := Page{Limit: MaxPageLimit}
	read := 0
	for len(f.WalletIDs) > 0 {
		items, next, err := uc.d.Transactions.List(ctx, f, page)
		if err != nil {
			return SpendingReport{}, err
		}
		for _, t := range items {
			if read == spendingScanLimit {
				out.Truncated = true
				break
			}
			read++
			if t.RatePending {
				out.PendingTransactions++
				continue
			}
			out.Count++
			total, ok := money.AddInt64(out.Total, -t.BaseAmount)
			if !ok {
				return SpendingReport{}, money.ErrOverflow
			}
			out.Total = total
			var err error
			switch in.GroupBy {
			case SpendingByEnvelope:
				if len(t.Splits) == 0 {
					err = add(t.EnvelopeID, -t.BaseAmount)
				}
				for _, line := range t.Splits {
					if err == nil {
						err = add(line.EnvelopeID, -line.BaseAmount)
					}
				}
			case SpendingByPayee:
				err = add(t.PayeeID, -t.BaseAmount)
			case SpendingByWallet:
				err = add(t.WalletID, -t.BaseAmount)
			case SpendingByVendor:
				err = add(domain.ID(vendorOf[t.ID]), -t.BaseAmount)
			}
			if err != nil {
				return SpendingReport{}, err
			}
		}
		if out.Truncated || next == "" {
			break
		}
		page.Cursor = next
	}
	out.Incomplete = out.PendingTransactions > 0
	for _, g := range sums {
		out.Groups = append(out.Groups, *g)
	}
	slices.SortFunc(out.Groups, func(a, b SpendingGroup) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.Key, b.Key))
	})
	return out, nil
}

// vendors maps the transactions that pay a charge of a subscription the
// principal sees to the subscription's vendor.
func (uc *SpendingReporter) vendors(ctx context.Context, s HouseholdScope, p Principal) (map[domain.ID]string, error) {
	out := map[domain.ID]string{}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil || len(subs) == 0 {
		return out, err
	}
	vendor := map[domain.ID]string{}
	for _, sub := range subs {
		vendor[sub.ID] = sub.Vendor
	}
	f := ChargeFilter{HouseholdID: s.Household.ID, SubscriptionIDs: subscriptionIDs(subs),
		Statuses: []subscription.ChargeStatus{subscription.Matched, subscription.Posted}}
	page := Page{Limit: MaxPageLimit}
	for read := 0; read < spendingScanLimit; {
		items, next, err := uc.d.Charges.List(ctx, f, page)
		if err != nil {
			return nil, err
		}
		for _, c := range items {
			if c.TransactionID != "" {
				out[c.TransactionID] = vendor[c.SubscriptionID]
			}
		}
		read += len(items)
		if next == "" {
			break
		}
		page.Cursor = next
	}
	return out, nil
}
