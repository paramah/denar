package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func newTransaction(env Env, id, householdID, walletID domain.ID, date string, amount int64) transaction.Transaction {
	return transaction.Transaction{ID: id, HouseholdID: householdID, WalletID: walletID, Date: day(date), Amount: amount,
		Currency: "PLN", BaseAmount: amount, Kind: transaction.Expense, Status: transaction.Cleared, Source: transaction.SourceManual,
		Tags: []string{}, Splits: []transaction.Split{}, CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "u1"},
		CreatedAt: now(env), UpdatedAt: now(env)}
}

func wantTransaction(t *testing.T, got, want transaction.Transaction) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("times = %v %v, want %v %v", got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	}
	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transaction =\n %+v\nwant\n %+v", got, want)
	}
}

func txIDs(ts []transaction.Transaction) []domain.ID {
	ids := []domain.ID{}
	for _, t := range ts {
		ids = append(ids, t.ID)
	}
	return ids
}

// seedTransactions adds a second wallet w2 (USD) to seedWallets.
func seedTransactions(t *testing.T, env Env) {
	t.Helper()
	seedWallets(t, env)
	w2 := newWallet(env, "w2", "h1", "u1", "Dolary")
	w2.Currency = "USD"
	noError(t, env.Wallets.Create(context.Background(), w2))
	noError(t, env.Wallets.Create(context.Background(), newWallet(env, "w9", "h2", "u1", "Obcy")))
}

// RunTransactionSuite checks a TransactionRepository. It needs Users,
// Households, Wallets and Payees.
func RunTransactionSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back every field", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p1", "h1", "Lidl")))
		tx := newTransaction(env, "t1", "h1", "w2", "2026-10-20", -1000)
		tx.Currency, tx.BaseAmount, tx.FxRate = "USD", -4000, "4"
		tx.PayeeID, tx.Tags, tx.Note, tx.ExternalID, tx.Status = "p1", []string{"food", "trip"}, "Zakupy", "ext-1", transaction.Pending
		tx.Source, tx.CreatedBy = transaction.SourceBot, domain.Actor{Kind: domain.ActorBot, ID: "b1"}
		tx.Splits = []transaction.Split{{EnvelopeID: "e1", Amount: -600, BaseAmount: -2400, Note: "jedzenie"}, {Amount: -400, BaseAmount: -1600}}
		noError(t, env.Transactions.Create(ctx, tx))
		got, err := env.Transactions.ByID(ctx, "h1", "t1")
		noError(t, err)
		wantTransaction(t, got, tx)
		_, err = env.Transactions.ByID(ctx, "h2", "t1")
		wantError(t, err, domain.ErrNotFound)

		pending := newTransaction(env, "t2", "h1", "w1", "2026-10-21", 500)
		pending.Kind, pending.RatePending, pending.BaseAmount = transaction.Income, true, 0
		noError(t, env.Transactions.Create(ctx, pending))
		got, err = env.Transactions.ByID(ctx, "h1", "t2")
		noError(t, err)
		wantTransaction(t, got, pending)

		leg := newTransaction(env, "t3", "h1", "w1", "2026-10-21", -500)
		leg.Kind, leg.TransferPairID, leg.TransferWalletID, leg.EnvelopeID = transaction.Transfer, "pair1", "w2", "e7"
		noError(t, env.Transactions.Create(ctx, leg))
		got, err = env.Transactions.ByID(ctx, "h1", "t3")
		noError(t, err)
		wantTransaction(t, got, leg)
	})

	t.Run("External ids are unique per wallet", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		a := newTransaction(env, "t1", "h1", "w1", "2026-10-20", -1)
		a.ExternalID = "x"
		noError(t, env.Transactions.Create(ctx, a))
		b := newTransaction(env, "t2", "h1", "w1", "2026-10-20", -1)
		b.ExternalID = "x"
		wantConflict(t, env.Transactions.Create(ctx, b), domain.ConflictDuplicateExternalID)
		b.WalletID = "w2"
		noError(t, env.Transactions.Create(ctx, b))
		c := newTransaction(env, "t3", "h1", "w1", "2026-10-20", -1)
		noError(t, env.Transactions.Create(ctx, c))
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t4", "h1", "w1", "2026-10-20", -1)))
		c.ExternalID = "x"
		wantConflict(t, env.Transactions.Update(ctx, c), domain.ConflictDuplicateExternalID)
	})

	t.Run("Update replaces the fields and the splits", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p1", "h1", "Lidl")))
		tx := newTransaction(env, "t1", "h1", "w1", "2026-10-20", -1000)
		tx.Splits = []transaction.Split{{EnvelopeID: "e1", Amount: -1000, BaseAmount: -1000}}
		noError(t, env.Transactions.Create(ctx, tx))
		changed := tx
		changed.Date, changed.Amount, changed.BaseAmount, changed.RatePending = day("2026-10-01"), -2000, 0, true
		changed.PayeeID, changed.EnvelopeID, changed.Kind, changed.Status = "p1", "e2", transaction.Expense, transaction.Reconciled
		changed.Tags, changed.Note, changed.ExternalID, changed.Splits = []string{"x"}, "n", "e", []transaction.Split{}
		changed.UpdatedAt = now(env).Add(time.Hour)
		// Fields the repository keeps.
		changed.WalletID, changed.Source, changed.CreatedBy = "w2", transaction.SourceImport, domain.Actor{Kind: domain.ActorBot, ID: "b9"}
		changed.CreatedAt = now(env).Add(time.Hour)
		noError(t, env.Transactions.Update(ctx, changed))
		got, err := env.Transactions.ByID(ctx, "h1", "t1")
		noError(t, err)
		want := changed
		want.WalletID, want.Source, want.CreatedBy, want.CreatedAt = tx.WalletID, tx.Source, tx.CreatedBy, tx.CreatedAt
		wantTransaction(t, got, want)
		missing := newTransaction(env, "nope", "h1", "w1", "2026-10-20", 1)
		wantError(t, env.Transactions.Update(ctx, missing), domain.ErrNotFound)
		foreign := changed
		foreign.HouseholdID = "h2"
		wantError(t, env.Transactions.Update(ctx, foreign), domain.ErrNotFound)
	})

	t.Run("Delete and ByPair", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		for _, id := range []domain.ID{"t2", "t1"} {
			leg := newTransaction(env, id, "h1", "w1", "2026-10-20", -1)
			leg.Kind, leg.TransferPairID, leg.TransferWalletID = transaction.Transfer, "pair", "w2"
			noError(t, env.Transactions.Create(ctx, leg))
		}
		legs, err := env.Transactions.ByPair(ctx, "h1", "pair")
		noError(t, err)
		sameOrder(t, "legs", txIDs(legs), []domain.ID{"t1", "t2"})
		legs, err = env.Transactions.ByPair(ctx, "h2", "pair")
		noError(t, err)
		if len(legs) != 0 {
			t.Errorf("legs of another household = %v", legs)
		}
		wantError(t, env.Transactions.Delete(ctx, "h2", "t1"), domain.ErrNotFound)
		noError(t, env.Transactions.Delete(ctx, "h1", "t1"))
		_, err = env.Transactions.ByID(ctx, "h1", "t1")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.Transactions.Delete(ctx, "h1", "t1"), domain.ErrNotFound)
	})

	t.Run("List filters, orders and pages", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p1", "h1", "Biedronka")))
		add := func(id, wallet domain.ID, date string, change func(*transaction.Transaction)) {
			tx := newTransaction(env, id, "h1", wallet, date, -100)
			if change != nil {
				change(&tx)
			}
			noError(t, env.Transactions.Create(ctx, tx))
		}
		add("t1", "w1", "2026-10-01", func(t *transaction.Transaction) { t.PayeeID = "p1"; t.Tags = []string{"food"} })
		add("t2", "w1", "2026-10-05", func(t *transaction.Transaction) { t.Note = "Kino z 100% rabatu"; t.Status = transaction.Pending })
		add("t3", "w2", "2026-10-05", func(t *transaction.Transaction) { t.Kind = transaction.Income })
		add("t4", "w1", "2026-10-09", func(t *transaction.Transaction) {
			t.Splits = []transaction.Split{{EnvelopeID: "e1", Amount: -50, BaseAmount: -50}, {Amount: -50, BaseAmount: -50}}
		})
		add("t5", "w1", "2026-10-09", func(t *transaction.Transaction) { t.EnvelopeID = "e1"; t.Tags = []string{"food", "x"} })
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t9", "h2", "w9", "2026-10-09", -1)))

		base := usecase.TransactionFilter{HouseholdID: "h1", WalletIDs: []domain.ID{"w1", "w2"}}
		list := func(f usecase.TransactionFilter) []domain.ID {
			t.Helper()
			items, next, err := env.Transactions.List(ctx, f, firstPage(50))
			noError(t, err)
			if next != "" {
				t.Errorf("next = %q on the only page", next)
			}
			return txIDs(items)
		}
		with := func(change func(*usecase.TransactionFilter)) usecase.TransactionFilter {
			f := base
			change(&f)
			return f
		}
		sameOrder(t, "all", list(base), []domain.ID{"t5", "t4", "t3", "t2", "t1"})
		sameOrder(t, "wallet", list(with(func(f *usecase.TransactionFilter) { f.WalletIDs = []domain.ID{"w2"} })), []domain.ID{"t3"})
		sameOrder(t, "no wallets", list(with(func(f *usecase.TransactionFilter) { f.WalletIDs = nil })), []domain.ID{})
		sameOrder(t, "dates", list(with(func(f *usecase.TransactionFilter) { f.From, f.To = day("2026-10-05"), day("2026-10-05") })),
			[]domain.ID{"t3", "t2"})
		sameOrder(t, "payee", list(with(func(f *usecase.TransactionFilter) { f.PayeeID = "p1" })), []domain.ID{"t1"})
		sameOrder(t, "envelope", list(with(func(f *usecase.TransactionFilter) { f.EnvelopeID = "e1" })), []domain.ID{"t5", "t4"})
		sameOrder(t, "kind", list(with(func(f *usecase.TransactionFilter) { f.Kind = transaction.Income })), []domain.ID{"t3"})
		sameOrder(t, "status", list(with(func(f *usecase.TransactionFilter) { f.Status = transaction.Pending })), []domain.ID{"t2"})
		sameOrder(t, "tag", list(with(func(f *usecase.TransactionFilter) { f.Tag = "food" })), []domain.ID{"t5", "t1"})
		sameOrder(t, "text in payee", list(with(func(f *usecase.TransactionFilter) { f.Text = "biedr" })), []domain.ID{"t1"})
		sameOrder(t, "text in note", list(with(func(f *usecase.TransactionFilter) { f.Text = "KINO" })), []domain.ID{"t2"})
		sameOrder(t, "text with wildcards", list(with(func(f *usecase.TransactionFilter) { f.Text = "100%" })), []domain.ID{"t2"})
		sameOrder(t, "underscore is literal", list(with(func(f *usecase.TransactionFilter) { f.Text = "_" })), []domain.ID{})

		var seen []domain.ID
		cursor := ""
		for range 10 {
			items, next, err := env.Transactions.List(ctx, base, usecase.Page{Cursor: cursor, Limit: 2})
			noError(t, err)
			seen = append(seen, txIDs(items)...)
			if next == "" {
				break
			}
			cursor = next
		}
		sameOrder(t, "pages", seen, []domain.ID{"t5", "t4", "t3", "t2", "t1"})

		_, next, err := env.Transactions.List(ctx, base, usecase.Page{Limit: 1})
		noError(t, err)
		_, _, err = env.Transactions.List(ctx, with(func(f *usecase.TransactionFilter) { f.Tag = "food" }), usecase.Page{Cursor: next, Limit: 1})
		wantError(t, err, domain.ErrInvalid)
		_, _, err = env.Transactions.List(ctx, base, usecase.Page{Cursor: "garbage", Limit: 1})
		wantError(t, err, domain.ErrInvalid)
	})

	t.Run("Balances and pending rates", func(t *testing.T) {
		env := factory(t)
		seedTransactions(t, env)
		add := func(id, wallet domain.ID, amount, base int64, status transaction.Status, pending bool) {
			tx := newTransaction(env, id, "h1", wallet, "2026-10-01", amount)
			tx.BaseAmount, tx.Status, tx.RatePending = base, status, pending
			if pending {
				tx.BaseAmount = 0
			}
			noError(t, env.Transactions.Create(ctx, tx))
		}
		add("t1", "w1", 10000, 10000, transaction.Reconciled, false)
		add("t2", "w1", -2500, -2500, transaction.Pending, false)
		add("t3", "w2", -1000, -4000, transaction.Cleared, false)
		add("t4", "w2", -500, 0, transaction.Cleared, true)
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t9", "h2", "w9", "2026-10-09", -1)))
		got, err := env.Transactions.Balances(ctx, "h1", []domain.ID{"w1", "w2", "w3"})
		noError(t, err)
		want := map[domain.ID]usecase.WalletBalance{
			"w1": {Total: 7500, Cleared: 10000, Base: 7500},
			"w2": {Total: -1500, Cleared: -1500, Base: -4000, Pending: 1},
			"w3": {},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("balances = %+v, want %+v", got, want)
		}

		p := newTransaction(env, "t8", "h2", "w9", "2026-10-09", -1)
		p.RatePending, p.BaseAmount = true, 0
		noError(t, env.Transactions.Create(ctx, p))
		pending, err := env.Transactions.ListRatePending(ctx, "h1", 10)
		noError(t, err)
		sameOrder(t, "pending in h1", txIDs(pending), []domain.ID{"t4"})
		pending, err = env.Transactions.ListRatePending(ctx, "", 10)
		noError(t, err)
		sameOrder(t, "pending everywhere", txIDs(pending), []domain.ID{"t4", "t8"})
		pending, err = env.Transactions.ListRatePending(ctx, "", 1)
		noError(t, err)
		sameOrder(t, "limited", txIDs(pending), []domain.ID{"t4"})
	})
}
