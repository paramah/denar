package usecase_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (f *family) assign(envelope domain.ID, month, amount string) usecase.BudgetMonthView {
	f.t.Helper()
	v, err := usecase.NewSetAssignment(f.deps).Do(ctx, human(f.member), "h1", envelope, usecase.SetAssignmentInput{Month: month, Amount: pln(amount)})
	f.must(err)
	return v
}

func (f *family) month(p usecase.Principal, m string) usecase.BudgetMonthView {
	f.t.Helper()
	v, err := usecase.NewGetBudgetMonth(f.deps).Do(ctx, p, "h1", m)
	f.must(err)
	return v
}

func line(t *testing.T, v usecase.BudgetMonthView, id domain.ID) usecase.BudgetEnvelopeLine {
	t.Helper()
	for _, g := range v.Groups {
		for _, l := range g.Envelopes {
			if l.Envelope.ID == id {
				return l
			}
		}
	}
	t.Fatalf("envelope %s not in the month", id)
	return usecase.BudgetEnvelopeLine{}
}

func available(v usecase.BudgetMonthView) int64 {
	var sum int64
	for _, g := range v.Groups {
		for _, l := range g.Envelopes {
			sum += l.Available
		}
	}
	return sum
}

// onBudgetMoney sums the base amounts of the on-budget wallets' known
// transactions dated up to the day: the right side of the invariant.
func onBudgetMoney(f *family, until domain.Date) int64 {
	f.t.Helper()
	ws, _ := f.Wallets.ListByHousehold(ctx, "h1")
	var ids []domain.ID
	for _, w := range ws {
		if w.OnBudget {
			ids = append(ids, w.ID)
		}
	}
	items, _, err := f.Transactions.List(ctx, usecase.TransactionFilter{HouseholdID: "h1", WalletIDs: ids, To: until}, usecase.Page{Limit: 100000})
	f.must(err)
	var sum int64
	for _, t := range items {
		if !t.RatePending {
			sum += t.BaseAmount
		}
	}
	return sum
}

func wantInvariant(t *testing.T, f *family, v usecase.BudgetMonthView) {
	t.Helper()
	if got, want := available(v)+v.ReadyToAssign+v.AssignedInFuture, onBudgetMoney(f, v.End); got != want || available(v) != v.Available {
		t.Errorf("%s: Σ available %d + RTA %d + future %d = %d, want %d", v.Month, available(v), v.ReadyToAssign, v.AssignedInFuture, got, want)
	}
}

// TestBudgetMonth walks one month with every kind of money: income,
// envelope spending with overspending, money without an envelope, a split,
// a foreign currency, transfers on and off budget, another person's
// private wallet, a transaction waiting for a rate; then the next month.
func TestBudgetMonth(t *testing.T) {
	f := budgetFamily(t)
	food, fun := f.vars["food"], f.vars["fun"]
	off := f.addWallet("wo", "h1", f.member, wallet.Shared)
	off.OnBudget = false
	f.must(f.Wallets.Update(ctx, off))
	f.addWallet("wp", "h1", f.owner, wallet.Private)
	m := human(f.member)
	rec := func(p usecase.Principal, in usecase.RecordTransactionInput) {
		t.Helper()
		_, err := record(f, p, in)
		noErr(t, err)
	}
	move := func(in usecase.CreateTransferInput) {
		t.Helper()
		_, err := transfer(f, m, in)
		noErr(t, err)
	}
	rec(m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("5000"), Kind: "income", Date: day("2026-10-01")})
	f.assign(food, "2026-10", "1000")
	f.assign(fun, "2026-10", "300")
	rec(m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1200"), EnvelopeID: string(food), Date: day("2026-10-05")})
	rec(m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-50"), Date: day("2026-10-06")})
	rec(m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-100"), Date: day("2026-10-07"),
		Splits: []usecase.SplitInput{{EnvelopeID: string(food), Amount: pln("-40")}, {Amount: pln("-60")}}})
	rec(m, usecase.RecordTransactionInput{WalletID: "w2", Amount: usd("-10"), EnvelopeID: string(fun), Date: day("2026-10-20")})
	move(usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w2", Amount: pln("400"), ToAmount: &usecase.MoneyInput{Amount: "100", Currency: "USD"}, Date: day("2026-10-20")})
	move(usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w2", Amount: pln("390"), ToAmount: &usecase.MoneyInput{Amount: "100", Currency: "USD"}, Date: day("2026-10-20")})
	move(usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "wo", Amount: pln("100"), EnvelopeID: string(fun), Date: day("2026-10-20")})
	move(usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "wo", Amount: pln("20"), Date: day("2026-10-20")})
	rec(human(f.owner), usecase.RecordTransactionInput{WalletID: "wp", Amount: pln("-30"), EnvelopeID: string(food), Date: day("2026-10-08")})
	rec(m, usecase.RecordTransactionInput{WalletID: "w2", Amount: usd("-5"), EnvelopeID: string(fun), Date: day("2026-10-05")}) // no rate
	// Money of other months and of off-budget wallets stays out.
	rec(m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-7"), EnvelopeID: string(food), Date: day("2026-11-02")})
	rec(m, usecase.RecordTransactionInput{WalletID: "wo", Amount: pln("-9"), EnvelopeID: string(food), Date: day("2026-10-09")})

	v := f.month(m, "2026-10")
	if v.Start != day("2026-10-01") || v.End != day("2026-10-31") || v.Currency != "PLN" {
		t.Errorf("range = %s..%s %s", v.Start, v.End, v.Currency)
	}
	fl, fn := line(t, v, food), line(t, v, fun)
	if fl.EnvelopeMonth != (budget.EnvelopeMonth{Assigned: 100000, Activity: -127000, Available: -27000}) {
		t.Errorf("food = %+v", fl.EnvelopeMonth)
	}
	if fn.EnvelopeMonth != (budget.EnvelopeMonth{Assigned: 30000, Activity: -14000, Available: 16000}) {
		t.Errorf("fun = %+v", fn.EnvelopeMonth)
	}
	if v.Income != 500000 || v.Uncategorized != -11000 || v.Transfers != -1000 {
		t.Errorf("inflow = %d / %d / %d", v.Income, v.Uncategorized, v.Transfers)
	}
	if v.ReadyToAssign != 358000 || v.AssignedInFuture != 0 || v.OverspentLastMonth != 0 {
		t.Errorf("RTA = %d, future %d, overspent %d", v.ReadyToAssign, v.AssignedInFuture, v.OverspentLastMonth)
	}
	if !v.Incomplete || v.PendingTransactions != 1 {
		t.Errorf("incomplete = %v (%d)", v.Incomplete, v.PendingTransactions)
	}
	if v.Assigned != 130000 || v.Activity != -141000 || v.Available != -11000 {
		t.Errorf("totals = %d %d %d", v.Assigned, v.Activity, v.Available)
	}
	if g := v.Groups[0]; g.Group.ID != f.vars["group"] || g.Assigned != 130000 || g.Available != -11000 || len(g.Envelopes) != 2 {
		t.Errorf("group = %+v", g)
	}
	wantInvariant(t, f, v)

	// The next month: food starts again at zero, its overspending lowers
	// Ready to Assign; fun carries over.
	nov := f.month(m, "2026-11")
	if l := line(t, nov, food); l.EnvelopeMonth != (budget.EnvelopeMonth{Activity: -700, Available: -700}) {
		t.Errorf("food in November = %+v", l.EnvelopeMonth)
	}
	if l := line(t, nov, fun); l.EnvelopeMonth != (budget.EnvelopeMonth{CarriedIn: 16000, Available: 16000}) {
		t.Errorf("fun in November = %+v", l.EnvelopeMonth)
	}
	if nov.OverspentLastMonth != -27000 || nov.ReadyToAssign != 331000 || nov.Income != 0 {
		t.Errorf("November RTA = %d, overspent %d", nov.ReadyToAssign, nov.OverspentLastMonth)
	}
	wantInvariant(t, f, nov)

	// An assignment of a later month is already taken from Ready to Assign.
	f.assign(fun, "2026-12", "500")
	v = f.month(m, "2026-10")
	if v.ReadyToAssign != 308000 || v.AssignedInFuture != 50000 {
		t.Errorf("with a future assignment: RTA %d, future %d", v.ReadyToAssign, v.AssignedInFuture)
	}
	wantInvariant(t, f, v)
	dec := f.month(m, "2026-12")
	if l := line(t, dec, fun); l.Assigned != 50000 || l.Available != 66000 || dec.AssignedInFuture != 0 {
		t.Errorf("December fun = %+v", l)
	}
	wantInvariant(t, f, dec)

	// The viewer sees the same budget, the other person's private wallet
	// included (AD-301).
	if vv := f.month(human(f.viewer), "2026-10"); vv.ReadyToAssign != v.ReadyToAssign || line(t, vv, food).Activity != -127000 {
		t.Errorf("viewer's month = %+v", vv)
	}
	_, err := usecase.NewGetBudgetMonth(f.deps).Do(ctx, m, "h1", "2026-13")
	wantField(t, err, "month")
	_, err = usecase.NewGetBudgetMonth(f.deps).Do(ctx, human(f.outsider), "h1", "2026-10")
	wantErr(t, err, domain.ErrNotFound)
}

func TestBudgetMonthStartDay(t *testing.T) {
	f := budgetFamily(t)
	h, _ := f.Households.ByID(ctx, "h1")
	h.MonthStartDay = 10
	f.must(f.Households.Update(ctx, h))
	food := f.vars["food"]
	for _, d := range []string{"2026-10-09", "2026-10-10", "2026-11-09", "2026-11-10"} {
		_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), EnvelopeID: string(food), Date: day(d)})
		noErr(t, err)
	}
	v := f.month(human(f.member), "2026-10")
	if v.Start != day("2026-10-10") || v.End != day("2026-11-09") || line(t, v, food).Activity != -200 {
		t.Errorf("month = %s..%s, food %+v", v.Start, v.End, line(t, v, food))
	}
	if l := line(t, f.month(human(f.member), "2026-09"), food); l.Activity != -100 {
		t.Errorf("September = %+v", l)
	}
}

func TestAssignAndMove(t *testing.T) {
	f := budgetFamily(t)
	food, fun := f.vars["food"], f.vars["fun"]
	set := usecase.NewSetAssignment(f.deps)
	v := f.assign(food, "2026-10", "100.50")
	if line(t, v, food).Assigned != 10050 || v.ReadyToAssign != -10050 {
		t.Errorf("after assign = %+v RTA %d", line(t, v, food), v.ReadyToAssign)
	}
	_, err := set.Do(ctx, human(f.member), "h1", food, usecase.SetAssignmentInput{Month: "2026-10", Amount: usd("1")})
	wantField(t, err, "amount.currency")
	_, err = set.Do(ctx, human(f.member), "h1", food, usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("1.001")})
	wantField(t, err, "amount.amount")
	_, err = set.Do(ctx, human(f.member), "h1", food, usecase.SetAssignmentInput{Month: "10/2026", Amount: pln("1")})
	wantField(t, err, "month")
	_, err = set.Do(ctx, human(f.member), "h1", "nope", usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("1")})
	wantErr(t, err, domain.ErrNotFound)
	_, err = set.Do(ctx, human(f.viewer), "h1", food, usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("1")})
	wantForbidden(t, err, domain.ForbiddenRole)

	mv := usecase.NewMoveBetweenEnvelopes(f.deps)
	v, err = mv.Do(ctx, human(f.member), "h1", usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food),
		ToEnvelopeID: string(fun), Amount: pln("150")})
	noErr(t, err)
	if line(t, v, food).Assigned != -4950 || line(t, v, fun).Assigned != 15000 || v.ReadyToAssign != -10050 {
		t.Errorf("after move: food %+v, fun %+v", line(t, v, food), line(t, v, fun))
	}
	for _, tt := range []struct {
		in    usecase.MoveBetweenEnvelopesInput
		field string
	}{
		{usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food), ToEnvelopeID: string(food), Amount: pln("1")}, "to_envelope_id"},
		{usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food), ToEnvelopeID: string(fun), Amount: pln("0")}, "amount.amount"},
		{usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food), ToEnvelopeID: string(fun), Amount: pln("-1")}, "amount.amount"},
		{usecase.MoveBetweenEnvelopesInput{Month: "x", FromEnvelopeID: string(food), ToEnvelopeID: string(fun), Amount: pln("1")}, "month"},
	} {
		_, err := mv.Do(ctx, human(f.member), "h1", tt.in)
		wantField(t, err, tt.field)
	}
	_, err = mv.Do(ctx, human(f.member), "h1", usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food), ToEnvelopeID: "nope", Amount: pln("1")})
	wantErr(t, err, domain.ErrNotFound)

	// A move is atomic: a failing audit leaves both assignments as they were.
	f.Audit.AppendErr = fmt.Errorf("disk full")
	_, err = mv.Do(ctx, human(f.member), "h1", usecase.MoveBetweenEnvelopesInput{Month: "2026-10", FromEnvelopeID: string(food), ToEnvelopeID: string(fun), Amount: pln("1")})
	if err == nil {
		t.Fatal("move without audit succeeded")
	}
	f.Audit.AppendErr = nil
	a, _ := f.Assignments.Get(ctx, "h1", food, budget.NewMonth(2026, 10))
	b, _ := f.Assignments.Get(ctx, "h1", fun, budget.NewMonth(2026, 10))
	if a.Amount != -4950 || b.Amount != 15000 {
		t.Errorf("after a failed move: %d %d", a.Amount, b.Amount)
	}

	// Zero removes the assignment.
	f.assign(fun, "2026-10", "0")
	if list, _ := f.Assignments.ListByHousehold(ctx, "h1"); len(list) != 1 {
		t.Errorf("assignments = %+v", list)
	}
	audit := f.Audit.All()
	var sets []string
	for _, e := range audit {
		if e.Action == "assignment.set" {
			sets = append(sets, fmt.Sprintf("%s %v→%v %v", e.ResourceID, e.Changes["assigned"].From, e.Changes["assigned"].To, e.Changes["via"].To))
		}
	}
	want := []string{
		fmt.Sprintf("%s 0.00 PLN→100.50 PLN set", food),
		fmt.Sprintf("%s 100.50 PLN→-49.50 PLN move", food),
		fmt.Sprintf("%s 0.00 PLN→150.00 PLN move", fun),
		fmt.Sprintf("%s 150.00 PLN→0.00 PLN set", fun),
	}
	if !slices.Equal(sets, want) {
		t.Errorf("audit =\n%v\nwant\n%v", sets, want)
	}
	// Setting the same amount again records nothing.
	before := len(f.Audit.All())
	f.assign(food, "2026-10", "-49.50")
	if len(f.Audit.All()) != before {
		t.Error("a no-op assignment was recorded")
	}
}

func TestGoalsAndAutoAssign(t *testing.T) {
	f := budgetFamily(t)
	food, fun := f.vars["food"], f.vars["fun"]
	group := string(f.vars["group"])
	update := usecase.NewUpdateEnvelope(f.deps)
	_, err := update.Do(ctx, human(f.member), "h1", food, usecase.UpdateEnvelopeInput{Goal: domain.Some(&usecase.GoalInput{Type: "monthly", Amount: pln("800")})})
	noErr(t, err)
	_, err = update.Do(ctx, human(f.member), "h1", fun, usecase.UpdateEnvelopeInput{Goal: domain.Some(&usecase.GoalInput{Type: "monthly", Amount: pln("200")})})
	noErr(t, err)
	trip, err := usecase.NewCreateEnvelope(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: group, Name: "Wakacje", Position: 9,
		Goal: &usecase.GoalInput{Type: "target_by_date", Amount: pln("1200"), TargetDate: day("2027-03-15")}})
	noErr(t, err)
	hidden, err := usecase.NewCreateEnvelope(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: group, Name: "Stare", Position: 0,
		Hidden: true, Goal: &usecase.GoalInput{Type: "monthly", Amount: pln("50")}})
	noErr(t, err)
	_, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("1000"), Kind: "income", Date: day("2026-10-01")})
	noErr(t, err)
	f.assign(food, "2026-10", "300")

	v := f.month(human(f.member), "2026-10")
	if g := line(t, v, food).Goal; g == nil || *g != (budget.GoalStatus{Needed: 80000, Underfunded: 50000, Progress: 37}) {
		t.Errorf("food goal = %+v", g)
	}
	if g := line(t, v, trip.ID).Goal; g == nil || *g != (budget.GoalStatus{Needed: 20000, Underfunded: 20000}) {
		t.Errorf("trip goal = %+v", g)
	}
	if line(t, v, f.vars["food"]).Goal == nil || v.Underfunded != 50000+20000+20000 {
		t.Errorf("underfunded = %d", v.Underfunded)
	}

	// 700 PLN is left: food takes 500, fun 200, nothing is left for the
	// trip; the hidden envelope is skipped.
	res, err := usecase.NewAutoAssign(f.deps).Do(ctx, human(f.member), "h1", "2026-10")
	noErr(t, err)
	if !slices.Equal(res.Assigned, []budget.Allocation{{EnvelopeID: food, Amount: 50000}, {EnvelopeID: fun, Amount: 20000}}) {
		t.Errorf("assigned = %+v", res.Assigned)
	}
	if res.Month.ReadyToAssign != 0 || line(t, res.Month, food).Assigned != 80000 || line(t, res.Month, hidden.ID).Assigned != 0 ||
		!line(t, res.Month, food).Goal.Funded {
		t.Errorf("after auto-assign = %+v", res.Month)
	}
	wantInvariant(t, f, res.Month)
	// Nothing left: nothing more is assigned.
	res, err = usecase.NewAutoAssign(f.deps).Do(ctx, human(f.member), "h1", "2026-10")
	noErr(t, err)
	if len(res.Assigned) != 0 {
		t.Errorf("second run = %+v", res.Assigned)
	}
	_, err = usecase.NewAutoAssign(f.deps).Do(ctx, human(f.viewer), "h1", "2026-10")
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = usecase.NewAutoAssign(f.deps).Do(ctx, human(f.member), "h1", "2026")
	wantField(t, err, "month")
}

func TestEnvelopeActivity(t *testing.T) {
	f := budgetFamily(t)
	food := f.vars["food"]
	f.addWallet("wp", "h1", f.owner, wallet.Private)
	off := f.addWallet("wo", "h1", f.member, wallet.Shared)
	off.OnBudget = false
	f.must(f.Wallets.Update(ctx, off))
	var mine []domain.ID
	for _, in := range []usecase.RecordTransactionInput{
		{WalletID: "w1", Amount: pln("-1"), EnvelopeID: string(food), Date: day("2026-10-01")},
		{WalletID: "w1", Amount: pln("-2"), Date: day("2026-10-31"), Splits: []usecase.SplitInput{{EnvelopeID: string(food), Amount: pln("-2")}}},
		{WalletID: "w1", Amount: pln("-3"), EnvelopeID: string(food), Date: day("2026-11-01")},
		{WalletID: "w1", Amount: pln("-4"), EnvelopeID: string(f.vars["fun"]), Date: day("2026-10-02")},
		{WalletID: "wo", Amount: pln("-5"), EnvelopeID: string(food), Date: day("2026-10-02")},
	} {
		v, err := record(f, human(f.member), in)
		noErr(t, err)
		mine = append(mine, v.Transaction.ID)
	}
	_, err := record(f, human(f.owner), usecase.RecordTransactionInput{WalletID: "wp", Amount: pln("-6"), EnvelopeID: string(food), Date: day("2026-10-03")})
	noErr(t, err)

	list := usecase.NewListEnvelopeActivity(f.deps)
	page, err := list.Do(ctx, human(f.member), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-10"})
	noErr(t, err)
	got := ids(page.Items, func(v usecase.TransactionView) domain.ID { return v.Transaction.ID })
	if !slices.Equal(got, []domain.ID{mine[1], mine[0]}) {
		t.Errorf("member sees %v", got)
	}
	page, err = list.Do(ctx, human(f.owner), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-10", Limit: domain.Some(2)})
	noErr(t, err)
	if len(page.Items) != 2 || page.NextCursor == "" {
		t.Errorf("owner's first page = %+v", page)
	}
	page, err = list.Do(ctx, human(f.owner), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-10", Limit: domain.Some(2), Cursor: page.NextCursor})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Transaction.ID != mine[0] {
		t.Errorf("owner's second page = %+v", page)
	}
	_, err = list.Do(ctx, human(f.member), "h1", "nope", usecase.ListEnvelopeActivityInput{Month: "2026-10"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = list.Do(ctx, human(f.member), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-1"})
	wantField(t, err, "month")

	// A bot needs transactions:read besides budget:read, and sees only its
	// wallets; the sums of the month still have everything.
	b := f.addBot("b1", "h1", f.member, "budget:read")
	_, err = list.Do(ctx, asBot(b), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-10"})
	wantForbidden(t, err, domain.ForbiddenScope)
	b.Scopes = append(b.Scopes, "transactions:read")
	b.WalletIDs = []domain.ID{"wp"}
	f.must(f.Bots.Update(ctx, b))
	page, err = list.Do(ctx, asBot(b), "h1", food, usecase.ListEnvelopeActivityInput{Month: "2026-10"})
	noErr(t, err)
	if len(page.Items) != 0 {
		t.Errorf("bot sees %+v", page.Items)
	}
	v, err := usecase.NewGetBudgetMonth(f.deps).Do(ctx, asBot(b), "h1", "2026-10")
	noErr(t, err)
	if line(t, v, food).Activity != -900 {
		t.Errorf("bot's food activity = %d", line(t, v, food).Activity)
	}
}

// TestBudgetInvariant checks FR-205 on random households: whatever is
// recorded, assigned and moved, in any month, the envelopes, Ready to
// Assign and the future assignments add up to the on-budget money.
func TestBudgetInvariant(t *testing.T) {
	for seed := range uint64(12) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 99))
			f := budgetFamily(t)
			f.addEnvelopes("h1", "a", "b")
			envs := []domain.ID{f.vars["food"], f.vars["fun"], "a", "b", ""}
			off := f.addWallet("wo", "h1", f.member, wallet.Shared)
			off.OnBudget = false
			f.must(f.Wallets.Update(ctx, off))
			for d := 1; d <= 31; d += 2 {
				f.Rates.Set("USD", fmt.Sprintf("2026-%02d-%02d", 8+d%4, d%28+1), fmt.Sprintf("3.%d", 7+d%3))
			}
			wallets := []string{"w1", "w2", "wo"}
			date := func() domain.Date { return domain.NewDate(2026, 8+timeMonth(r.IntN(5)), 1+r.IntN(28)) }
			amountIn := func(w string) usecase.MoneyInput {
				a := fmt.Sprintf("%d.%02d", r.IntN(300), r.IntN(100))
				if w == "w2" {
					return usd(a)
				}
				return pln(a)
			}
			for range 60 {
				w := wallets[r.IntN(len(wallets))]
				env := envs[r.IntN(len(envs))]
				switch r.IntN(6) {
				case 0:
					_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: w, Amount: amountIn(w), Kind: "income", Date: date()})
					noErr(t, err)
				case 1:
					a := amountIn(w)
					a.Amount = "-" + a.Amount
					_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: w, Amount: a, EnvelopeID: string(env), Date: date()})
					noErr(t, err)
				case 2:
					cur := usd
					if w != "w2" {
						cur = pln
					}
					_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: w, Amount: cur("-3.00"), Date: date(),
						Splits: []usecase.SplitInput{{EnvelopeID: string(env), Amount: cur("-1.00")}, {EnvelopeID: string(envs[r.IntN(len(envs))]), Amount: cur("-2.00")}}})
					noErr(t, err)
				case 3:
					to := wallets[r.IntN(len(wallets))]
					if to == w {
						continue
					}
					in := usecase.CreateTransferInput{FromWalletID: w, ToWalletID: to, Amount: amountIn(w), Date: date()}
					if in.Amount.Amount == "0.00" {
						continue
					}
					if (w == "w2") != (to == "w2") {
						ta := amountIn(to)
						if ta.Amount == "0.00" {
							continue
						}
						in.ToAmount = &ta
					}
					if (w == "wo") != (to == "wo") {
						in.EnvelopeID = string(env)
					}
					_, err := transfer(f, human(f.member), in)
					noErr(t, err)
				default:
					if env == "" {
						continue
					}
					m := budget.NewMonth(2026, 8+timeMonth(r.IntN(6)))
					f.assign(env, m.String(), fmt.Sprintf("%d", r.IntN(500)-150))
				}
			}
			for i := range 6 {
				v := f.month(human(f.member), budget.NewMonth(2026, 8+timeMonth(i)).String())
				wantInvariant(t, f, v)
			}
			// Transactions recorded before stage 003 with a gone envelope
			// count as money without an envelope.
			all, _, _ := f.Transactions.List(ctx, usecase.TransactionFilter{HouseholdID: "h1", WalletIDs: []domain.ID{"w1"}}, usecase.Page{Limit: 1})
			if len(all) == 1 && !all[0].IsTransfer() && len(all[0].Splits) == 0 && all[0].Kind == transaction.Expense {
				legacy := all[0]
				legacy.EnvelopeID = "gone"
				f.must(f.Transactions.Update(ctx, legacy))
				wantInvariant(t, f, f.month(human(f.member), "2026-12"))
			}
		})
	}
}

func timeMonth(n int) time.Month { return time.Month(n) }
