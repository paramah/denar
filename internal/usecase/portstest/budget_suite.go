package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func newGroup(env Env, id, householdID domain.ID, name string, position int) budget.Group {
	return budget.Group{ID: id, HouseholdID: householdID, Name: name, Position: position, CreatedAt: now(env), UpdatedAt: now(env)}
}

func newEnvelope(env Env, id, householdID, groupID domain.ID, name string, position int) budget.Envelope {
	return budget.Envelope{ID: id, HouseholdID: householdID, GroupID: groupID, Name: name, Position: position,
		CreatedAt: now(env), UpdatedAt: now(env)}
}

func month(s string) budget.Month {
	m, err := budget.ParseMonth("month", s)
	if err != nil {
		panic(err)
	}
	return m
}

func sameTimes(t *testing.T, gotCreated, gotUpdated, wantCreated, wantUpdated time.Time) {
	t.Helper()
	if !gotCreated.Equal(wantCreated) || !gotUpdated.Equal(wantUpdated) {
		t.Errorf("times = %v %v, want %v %v", gotCreated, gotUpdated, wantCreated, wantUpdated)
	}
}

func wantGroup(t *testing.T, got, want budget.Group) {
	t.Helper()
	sameTimes(t, got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	if got != want {
		t.Errorf("group = %+v, want %+v", got, want)
	}
}

func wantEnvelope(t *testing.T, got, want budget.Envelope) {
	t.Helper()
	sameTimes(t, got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("envelope = %+v, want %+v", got, want)
	}
}

// seedBudget adds to seedWallets a group g1 "Codzienne" with envelopes e1
// "Jedzenie" and e2 "Transport" in h1, and a group g9 with envelope e9 in
// h2.
func seedBudget(t *testing.T, env Env) {
	t.Helper()
	ctx := context.Background()
	seedWallets(t, env)
	noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g1", "h1", "Codzienne", 0)))
	noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e1", "h1", "g1", "Jedzenie", 0)))
	noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e2", "h1", "g1", "Transport", 1)))
	noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g9", "h2", "Obce", 0)))
	noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e9", "h2", "g9", "Obca", 0)))
}

// RunEnvelopeGroupSuite checks an EnvelopeGroupRepository. It needs Users,
// Households, Wallets and Envelopes.
func RunEnvelopeGroupSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back, update, scoped to the household", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		g := newGroup(env, "g1", "h1", "Rachunki", 3)
		noError(t, env.EnvelopeGroups.Create(ctx, g))
		got, err := env.EnvelopeGroups.ByID(ctx, "h1", "g1")
		noError(t, err)
		wantGroup(t, got, g)
		_, err = env.EnvelopeGroups.ByID(ctx, "h2", "g1")
		wantError(t, err, domain.ErrNotFound)

		changed := g
		changed.Name, changed.Position, changed.UpdatedAt = "Opłaty", 7, now(env).Add(time.Hour)
		changed.CreatedAt = now(env).Add(time.Hour)
		noError(t, env.EnvelopeGroups.Update(ctx, changed))
		got, err = env.EnvelopeGroups.ByID(ctx, "h1", "g1")
		noError(t, err)
		want := changed
		want.CreatedAt = g.CreatedAt
		wantGroup(t, got, want)
		wantError(t, env.EnvelopeGroups.Update(ctx, newGroup(env, "g1", "h2", "X", 0)), domain.ErrNotFound)
		wantError(t, env.EnvelopeGroups.Update(ctx, newGroup(env, "nope", "h1", "X", 0)), domain.ErrNotFound)
	})

	t.Run("Names are unique per household, list is ordered", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g1", "h1", "Rachunki", 1)))
		wantConflict(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g2", "h1", "RACHUNKI", 0)), domain.ConflictDuplicateName)
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g3", "h2", "Rachunki", 0)))
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g4", "h1", "Zabawa", 0)))
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g5", "h1", "auto", 1)))
		wantConflict(t, env.EnvelopeGroups.Update(ctx, newGroup(env, "g4", "h1", "rachunki", 0)), domain.ConflictDuplicateName)
		list, err := env.EnvelopeGroups.ListByHousehold(ctx, "h1")
		noError(t, err)
		var ids []domain.ID
		for _, g := range list {
			ids = append(ids, g.ID)
		}
		sameOrder(t, "groups", ids, []domain.ID{"g4", "g5", "g1"})
	})

	t.Run("Delete only an empty group", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		wantConflict(t, env.EnvelopeGroups.Delete(ctx, "h1", "g1"), domain.ConflictGroupNotEmpty)
		wantError(t, env.EnvelopeGroups.Delete(ctx, "h2", "g1"), domain.ErrNotFound)
		noError(t, env.Envelopes.Delete(ctx, "h1", "e1"))
		noError(t, env.Envelopes.Delete(ctx, "h1", "e2"))
		noError(t, env.EnvelopeGroups.Delete(ctx, "h1", "g1"))
		_, err := env.EnvelopeGroups.ByID(ctx, "h1", "g1")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.EnvelopeGroups.Delete(ctx, "h1", "g1"), domain.ErrNotFound)
	})
}

// RunEnvelopeSuite checks an EnvelopeRepository. It needs Users,
// Households, Wallets, EnvelopeGroups and Assignments.
func RunEnvelopeSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back every field, update", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g2", "h1", "Cele", 1)))
		e := newEnvelope(env, "e3", "h1", "g1", "Wakacje", 4)
		e.Hidden, e.Goal = true, budget.Goal{Type: budget.GoalTargetByDate, Amount: 500000, TargetDate: day("2027-06-30")}
		noError(t, env.Envelopes.Create(ctx, e))
		got, err := env.Envelopes.ByID(ctx, "h1", "e3")
		noError(t, err)
		wantEnvelope(t, got, e)
		_, err = env.Envelopes.ByID(ctx, "h2", "e3")
		wantError(t, err, domain.ErrNotFound)

		changed := e
		changed.GroupID, changed.Name, changed.Position, changed.Hidden = "g2", "Urlop", 0, false
		changed.Goal, changed.UpdatedAt = budget.Goal{Type: budget.GoalMonthly, Amount: 1000}, now(env).Add(time.Hour)
		noError(t, env.Envelopes.Update(ctx, changed))
		got, err = env.Envelopes.ByID(ctx, "h1", "e3")
		noError(t, err)
		wantEnvelope(t, got, changed)

		changed.Goal = budget.Goal{}
		noError(t, env.Envelopes.Update(ctx, changed))
		got, err = env.Envelopes.ByID(ctx, "h1", "e3")
		noError(t, err)
		wantEnvelope(t, got, changed)
		wantError(t, env.Envelopes.Update(ctx, newEnvelope(env, "e3", "h2", "g9", "X", 0)), domain.ErrNotFound)
	})

	t.Run("The group must be one of the household", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		wantError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e3", "h1", "g9", "X", 0)), domain.ErrNotFound)
		wantError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e3", "h1", "nope", "X", 0)), domain.ErrNotFound)
		wantError(t, env.Envelopes.Update(ctx, newEnvelope(env, "e1", "h1", "g9", "Jedzenie", 0)), domain.ErrNotFound)
	})

	t.Run("Names are unique per household, list is ordered", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g2", "h1", "Cele", 1)))
		wantConflict(t, env.Envelopes.Create(ctx, newEnvelope(env, "e3", "h1", "g2", "JEDZENIE", 0)), domain.ConflictDuplicateName)
		noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e3", "h2", "g9", "Jedzenie", 1)))
		noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e4", "h1", "g2", "Auto", 1)))
		wantConflict(t, env.Envelopes.Update(ctx, newEnvelope(env, "e4", "h1", "g2", "transport", 0)), domain.ConflictDuplicateName)
		list, err := env.Envelopes.ListByHousehold(ctx, "h1")
		noError(t, err)
		var ids []domain.ID
		for _, e := range list {
			ids = append(ids, e.ID)
		}
		sameOrder(t, "envelopes", ids, []domain.ID{"e1", "e4", "e2"})
	})

	t.Run("Delete takes the assignments along", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "e1", Month: month("2026-10"), Amount: 100, UpdatedAt: now(env)}))
		noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "e2", Month: month("2026-10"), Amount: 200, UpdatedAt: now(env)}))
		wantError(t, env.Envelopes.Delete(ctx, "h2", "e1"), domain.ErrNotFound)
		noError(t, env.Envelopes.Delete(ctx, "h1", "e1"))
		_, err := env.Envelopes.ByID(ctx, "h1", "e1")
		wantError(t, err, domain.ErrNotFound)
		list, err := env.Assignments.ListByHousehold(ctx, "h1")
		noError(t, err)
		if len(list) != 1 || list[0].EnvelopeID != "e2" {
			t.Errorf("assignments = %+v", list)
		}
		wantError(t, env.Envelopes.Delete(ctx, "h1", "e1"), domain.ErrNotFound)
	})
}

// RunAssignmentSuite checks an AssignmentRepository. It needs Users,
// Households, Wallets, EnvelopeGroups and Envelopes.
func RunAssignmentSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Set, replace, remove, list", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		a := budget.Assignment{HouseholdID: "h1", EnvelopeID: "e1", Month: month("2026-10"), Amount: 50000, UpdatedAt: now(env)}
		noError(t, env.Assignments.Set(ctx, a))
		got, err := env.Assignments.Get(ctx, "h1", "e1", month("2026-10"))
		noError(t, err)
		if got.Amount != 50000 || got.Month != a.Month || got.EnvelopeID != "e1" || got.HouseholdID != "h1" || !got.UpdatedAt.Equal(a.UpdatedAt) {
			t.Errorf("assignment = %+v", got)
		}
		a.Amount, a.UpdatedAt = -700, now(env).Add(time.Minute)
		noError(t, env.Assignments.Set(ctx, a))
		got, err = env.Assignments.Get(ctx, "h1", "e1", month("2026-10"))
		noError(t, err)
		if got.Amount != -700 || !got.UpdatedAt.Equal(a.UpdatedAt) {
			t.Errorf("replaced = %+v", got)
		}
		none, err := env.Assignments.Get(ctx, "h1", "e1", month("2026-11"))
		noError(t, err)
		if none.Amount != 0 || none.EnvelopeID != "e1" || none.Month != month("2026-11") {
			t.Errorf("missing = %+v", none)
		}
		other, err := env.Assignments.Get(ctx, "h2", "e1", month("2026-10"))
		noError(t, err)
		if other.Amount != 0 {
			t.Errorf("another household's = %+v", other)
		}

		noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "e2", Month: month("2026-09"), Amount: 1, UpdatedAt: now(env)}))
		noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "e2", Month: month("2026-10"), Amount: 2, UpdatedAt: now(env)}))
		noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h2", EnvelopeID: "e9", Month: month("2026-10"), Amount: 3, UpdatedAt: now(env)}))
		list, err := env.Assignments.ListByHousehold(ctx, "h1")
		noError(t, err)
		var keys []string
		for _, a := range list {
			keys = append(keys, a.Month.String()+"/"+string(a.EnvelopeID))
		}
		if !reflect.DeepEqual(keys, []string{"2026-09/e2", "2026-10/e1", "2026-10/e2"}) {
			t.Errorf("list = %v", keys)
		}

		a.Amount = 0
		noError(t, env.Assignments.Set(ctx, a))
		list, err = env.Assignments.ListByHousehold(ctx, "h1")
		noError(t, err)
		if len(list) != 2 {
			t.Errorf("after removal = %+v", list)
		}
	})

	t.Run("The envelope must be one of the household", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		wantError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "e9", Month: month("2026-10"), Amount: 1, UpdatedAt: now(env)}), domain.ErrNotFound)
		wantError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: "h1", EnvelopeID: "nope", Month: month("2026-10"), Amount: 1, UpdatedAt: now(env)}), domain.ErrNotFound)
	})
}

// runBudgetFlowsTests checks TransactionRepository.BudgetFlows and
// CountByEnvelope.
func runBudgetFlowsTests(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("BudgetFlows sums by day, envelope and kind", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w3", "h1", "u1", "Poza")))
		add := func(id, wallet domain.ID, date string, base int64, kind transaction.Kind, envelope domain.ID, splits ...transaction.Split) {
			t.Helper()
			tx := newTransaction(env, id, "h1", wallet, date, base)
			tx.Kind, tx.EnvelopeID = kind, envelope
			if kind == transaction.Transfer {
				tx.TransferPairID = "pair-" + id
			}
			if len(splits) > 0 {
				tx.Splits = splits
			}
			noError(t, env.Transactions.Create(ctx, tx))
		}
		add("t1", "w1", "2026-10-01", 500000, transaction.Income, "")
		add("t2", "w1", "2026-10-02", -1000, transaction.Expense, "e1")
		add("t3", "w1", "2026-10-02", -2500, transaction.Expense, "e1")
		add("t4", "w1", "2026-10-02", -300, transaction.Expense, "")
		add("t5", "w2", "2026-10-02", -4000, transaction.Expense, "", // USD, base in PLN
			transaction.Split{EnvelopeID: "e1", Amount: -600, BaseAmount: -2400}, transaction.Split{Amount: -400, BaseAmount: -1600})
		add("t6", "w1", "2026-10-03", -10000, transaction.Transfer, "")
		add("t7", "w1", "2026-11-01", -50, transaction.Expense, "e2")
		add("t8", "w3", "2026-10-02", -999, transaction.Expense, "e1") // not asked for
		pending := newTransaction(env, "t9", "h1", "w2", "2026-10-02", -100)
		pending.Currency, pending.RatePending, pending.BaseAmount, pending.EnvelopeID = "USD", true, 0, "e1"
		noError(t, env.Transactions.Create(ctx, pending))
		pending2 := newTransaction(env, "t10", "h1", "w2", "2026-10-02", -100)
		pending2.Currency, pending2.RatePending, pending2.BaseAmount = "USD", true, 0
		pending2.Splits = []transaction.Split{{EnvelopeID: "e1", Amount: -50}, {EnvelopeID: "e2", Amount: -50}}
		noError(t, env.Transactions.Create(ctx, pending2))

		got, err := env.Transactions.BudgetFlows(ctx, "h1", []domain.ID{"w1", "w2"}, day("2026-10-31"))
		noError(t, err)
		want := []usecase.BudgetFlow{
			{Date: day("2026-10-01"), Kind: transaction.Income, Base: 500000},
			{Date: day("2026-10-02"), Kind: transaction.Expense, Base: -1900},
			{Date: day("2026-10-02"), EnvelopeID: "e1", Kind: transaction.Expense, Base: -5900},
			{Date: day("2026-10-03"), Kind: transaction.Transfer, Base: -10000},
		}
		if !reflect.DeepEqual(got.Flows, want) {
			t.Errorf("flows =\n %+v\nwant\n %+v", got.Flows, want)
		}
		if len(got.Pending) != 1 || got.Pending[day("2026-10-02")] != 2 {
			t.Errorf("pending = %v", got.Pending)
		}
		all, err := env.Transactions.BudgetFlows(ctx, "h1", []domain.ID{"w1", "w2"}, domain.Date{})
		noError(t, err)
		if len(all.Flows) != 5 || all.Flows[4].EnvelopeID != "e2" || all.Flows[4].Base != -50 {
			t.Errorf("all flows = %+v", all.Flows)
		}
		none, err := env.Transactions.BudgetFlows(ctx, "h2", []domain.ID{"w1"}, domain.Date{})
		noError(t, err)
		if len(none.Flows) != 0 || len(none.Pending) != 0 {
			t.Errorf("other household = %+v", none)
		}
		empty, err := env.Transactions.BudgetFlows(ctx, "h1", nil, domain.Date{})
		noError(t, err)
		if len(empty.Flows) != 0 || empty.Pending == nil {
			t.Errorf("no wallets = %+v", empty)
		}
	})

	t.Run("CountByEnvelope counts transactions and split lines", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		a := newTransaction(env, "t1", "h1", "w1", "2026-10-02", -1000)
		a.EnvelopeID = "e1"
		noError(t, env.Transactions.Create(ctx, a))
		b := newTransaction(env, "t2", "h1", "w1", "2026-10-02", -1000)
		b.Splits = []transaction.Split{{EnvelopeID: "e1", Amount: -500, BaseAmount: -500}, {EnvelopeID: "e1", Amount: -500, BaseAmount: -500}}
		noError(t, env.Transactions.Create(ctx, b))
		c := newTransaction(env, "t3", "h2", "w9", "2026-10-02", -1)
		c.EnvelopeID = "e1"
		noError(t, env.Transactions.Create(ctx, c))
		for envelope, want := range map[domain.ID]int{"e1": 2, "e2": 0} {
			n, err := env.Transactions.CountByEnvelope(ctx, "h1", envelope)
			noError(t, err)
			if n != want {
				t.Errorf("CountByEnvelope(%s) = %d, want %d", envelope, n, want)
			}
		}
	})
}
