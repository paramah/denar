package usecase

import (
	"context"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/transaction"
)

// BudgetEnvelopeLine is one envelope in a budget month.
type BudgetEnvelopeLine struct {
	Envelope budget.Envelope
	budget.EnvelopeMonth
	// Goal is the goal's status; nil without a goal.
	Goal *budget.GoalStatus
}

// BudgetGroupView is a group of a budget month with the totals of its
// envelopes.
type BudgetGroupView struct {
	Group                         budget.Group
	Assigned, Activity, Available int64
	Envelopes                     []BudgetEnvelopeLine
}

// BudgetMonthView is the budget of one month (FR-214). Amounts are minor
// units of Currency, the household's base currency.
type BudgetMonthView struct {
	Month      budget.Month
	Start, End domain.Date
	Currency   string
	// ReadyToAssign is lowered by AssignedInFuture already (AD-308).
	ReadyToAssign      int64
	AssignedInFuture   int64
	OverspentLastMonth int64 // ≤ 0
	// Income, Uncategorized and Transfers are this month's inflow to Ready
	// to Assign: income, money spent or refunded without an envelope, and
	// transfer legs without one.
	Income, Uncategorized, Transfers int64
	// Totals of every envelope, hidden ones too; Underfunded of the goals
	// of the envelopes that are not hidden.
	Assigned, Activity, Available, Underfunded int64
	// Incomplete: PendingTransactions transactions up to the end of the
	// month wait for a rate and are left out (AD-307).
	Incomplete          bool
	PendingTransactions int
	Groups              []BudgetGroupView
}

// onBudgetWallets returns the ids of every on-budget wallet of the
// household, archived ones and ones the principal does not see included:
// the budget is the household's (AD-301).
func (d Deps) onBudgetWallets(ctx context.Context, householdID domain.ID) ([]domain.ID, error) {
	all, err := d.Wallets.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	ids := []domain.ID{}
	for _, w := range all {
		if w.OnBudget {
			ids = append(ids, w.ID)
		}
	}
	return ids, nil
}

// budgetMonth computes the budget of month m.
func (d Deps) budgetMonth(ctx context.Context, h household.Household, m budget.Month) (BudgetMonthView, error) {
	start := h.MonthStartDay
	groups, err := d.EnvelopeGroups.ListByHousehold(ctx, h.ID)
	if err != nil {
		return BudgetMonthView{}, err
	}
	envelopes, err := d.Envelopes.ListByHousehold(ctx, h.ID)
	if err != nil {
		return BudgetMonthView{}, err
	}
	assignments, err := d.Assignments.ListByHousehold(ctx, h.ID)
	if err != nil {
		return BudgetMonthView{}, err
	}
	wallets, err := d.onBudgetWallets(ctx, h.ID)
	if err != nil {
		return BudgetMonthView{}, err
	}
	flows, err := d.Transactions.BudgetFlows(ctx, h.ID, wallets, m.End(start))
	if err != nil {
		return BudgetMonthView{}, err
	}

	v := BudgetMonthView{Month: m, Start: m.Start(start), End: m.End(start), Currency: h.BaseCurrency, Groups: []BudgetGroupView{}}
	known := make(map[domain.ID]bool, len(envelopes))
	for _, e := range envelopes {
		known[e.ID] = true
	}
	l := budget.NewLedger()
	for _, a := range assignments {
		if known[a.EnvelopeID] {
			l.SetAssigned(a.EnvelopeID, a.Month, a.Amount)
		}
	}
	for _, f := range flows.Flows {
		fm := budget.MonthOf(f.Date, start)
		if known[f.EnvelopeID] {
			l.AddActivity(f.EnvelopeID, fm, f.Base)
			continue
		}
		// No envelope, or one that no longer exists (AD-314).
		l.AddInflow(fm, f.Base)
		if fm != m {
			continue
		}
		switch {
		case f.Kind == transaction.Income:
			v.Income += f.Base
		case f.Kind == transaction.Transfer && f.EnvelopeID == "":
			v.Transfers += f.Base
		default:
			v.Uncategorized += f.Base
		}
	}
	for _, n := range flows.Pending {
		v.PendingTransactions += n
	}
	v.Incomplete = v.PendingTransactions > 0
	s, err := l.Month(m)
	if err != nil {
		return BudgetMonthView{}, err
	}
	v.ReadyToAssign, v.AssignedInFuture, v.OverspentLastMonth = s.ReadyToAssign, s.AssignedInFuture, s.OverspentLastMonth
	v.Assigned, v.Activity, v.Available = s.Assigned, s.Activity, s.Available

	byGroup := map[domain.ID][]budget.Envelope{}
	for _, e := range envelopes {
		byGroup[e.GroupID] = append(byGroup[e.GroupID], e)
	}
	for _, g := range groups {
		gv := BudgetGroupView{Group: g, Envelopes: []BudgetEnvelopeLine{}}
		for _, e := range byGroup[g.ID] {
			line := BudgetEnvelopeLine{Envelope: e, EnvelopeMonth: s.Envelopes[e.ID]}
			if !e.Goal.IsZero() {
				st := budget.Status(e.Goal, line.EnvelopeMonth, m, start)
				line.Goal = &st
				if !e.Hidden {
					v.Underfunded += st.Underfunded
				}
			}
			gv.Assigned += line.Assigned
			gv.Activity += line.Activity
			gv.Available += line.Available
			gv.Envelopes = append(gv.Envelopes, line)
		}
		v.Groups = append(v.Groups, gv)
	}
	return v, nil
}

// GetBudgetMonth returns the budget of a month (FR-214).
type GetBudgetMonth struct{ d Deps }

func NewGetBudgetMonth(d Deps) *GetBudgetMonth { return &GetBudgetMonth{d: d} }

func (uc *GetBudgetMonth) Do(ctx context.Context, p Principal, householdID domain.ID, month string) (BudgetMonthView, error) {
	s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetRead)
	if err != nil {
		return BudgetMonthView{}, err
	}
	m, err := uc.d.parseMonth(s.Household, month)
	if err != nil {
		return BudgetMonthView{}, err
	}
	return uc.d.budgetMonth(ctx, s.Household, m)
}

type ListEnvelopeActivityInput struct {
	Month  string
	Cursor string
	Limit  domain.Optional[int]
}

// ListEnvelopeActivity lists the transactions of an envelope in a month,
// split lines included, newest first: only those of on-budget wallets the
// principal sees, and only with transactions:read too (FR-215, AD-302).
type ListEnvelopeActivity struct{ d Deps }

func NewListEnvelopeActivity(d Deps) *ListEnvelopeActivity { return &ListEnvelopeActivity{d: d} }

func (uc *ListEnvelopeActivity) Do(ctx context.Context, p Principal, householdID, envelopeID domain.ID, in ListEnvelopeActivityInput) (TransactionPage, error) {
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return TransactionPage{}, err
	}
	s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetRead)
	if err != nil {
		return TransactionPage{}, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return TransactionPage{}, err
	}
	m, err := budget.ParseMonth("month", strings.TrimSpace(in.Month))
	if err != nil {
		return TransactionPage{}, err
	}
	if _, err := uc.d.Envelopes.ByID(ctx, householdID, envelopeID); err != nil {
		return TransactionPage{}, plainNotFound(err)
	}
	ws, err := uc.d.visibleWallets(ctx, s, p)
	if err != nil {
		return TransactionPage{}, err
	}
	f := TransactionFilter{HouseholdID: householdID, EnvelopeID: envelopeID, WalletIDs: []domain.ID{},
		From: m.Start(s.Household.MonthStartDay), To: m.End(s.Household.MonthStartDay)}
	for _, w := range ws {
		if w.OnBudget {
			f.WalletIDs = append(f.WalletIDs, w.ID)
		}
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

// parseMonth reads the month of a budget operation; an empty one is the
// household's current budget month, in its time zone and from its start
// day (AD-606).
func (d Deps) parseMonth(h household.Household, month string) (budget.Month, error) {
	if month == "" {
		return budget.MonthOf(today(h, d.Clock.Now()), h.MonthStartDay), nil
	}
	return budget.ParseMonth("month", month)
}
