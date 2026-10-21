package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

func newBatch(env Env, id, householdID, walletID domain.ID) imports.Batch {
	return imports.Batch{ID: id, HouseholdID: householdID, WalletID: walletID, Source: imports.SourceCSV, SourceRef: "wyciag.csv",
		Preset: "mbank", Status: imports.Completed, Rows: 5, Created: 2, Duplicates: 2, Failed: 1,
		Errors:    []imports.RowError{{Row: 7, Field: "amount", Message: "is not a number"}},
		CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "u1"}, CreatedAt: now(env)}
}

func wantBatch(t *testing.T, got, want imports.Batch) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.RevertedAt.Equal(want.RevertedAt) {
		t.Errorf("times = %v %v, want %v %v", got.CreatedAt, got.RevertedAt, want.CreatedAt, want.RevertedAt)
	}
	got.CreatedAt, got.RevertedAt = want.CreatedAt, want.RevertedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("batch =\n %+v\nwant\n %+v", got, want)
	}
}

func batchIDs(items []imports.Batch) []domain.ID {
	out := []domain.ID{}
	for _, b := range items {
		out = append(out, b.ID)
	}
	return out
}

// RunImportBatchSuite checks an ImportBatchRepository and
// TransactionRepository.ExternalIDs. It needs Users, Households, Wallets
// and Transactions.
func RunImportBatchSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back, update", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		b := newBatch(env, "b1", "h1", "w1")
		noError(t, env.ImportBatches.Create(ctx, b))
		got, err := env.ImportBatches.ByID(ctx, "h1", "b1")
		noError(t, err)
		wantBatch(t, got, b)
		_, err = env.ImportBatches.ByID(ctx, "h2", "b1")
		wantError(t, err, domain.ErrNotFound)

		b.Status, b.Matched, b.Reverted = imports.Reverted, 1, 2
		b.RevertedBy, b.RevertedAt = domain.Actor{Kind: domain.ActorBot, ID: "bot1"}, now(env).Add(time.Hour)
		changed := b
		changed.Rows, changed.SourceRef = 99, "inny.csv" // not stored by Update
		noError(t, env.ImportBatches.Update(ctx, changed))
		got, err = env.ImportBatches.ByID(ctx, "h1", "b1")
		noError(t, err)
		wantBatch(t, got, b)
		other := b
		other.HouseholdID = "h2"
		wantError(t, env.ImportBatches.Update(ctx, other), domain.ErrNotFound)
	})

	t.Run("A batch without errors reads back an empty list", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		b := newBatch(env, "b1", "h1", "w1")
		b.Errors, b.Preset, b.Source, b.SourceRef = nil, "", imports.SourceConnection, "conn1"
		noError(t, env.ImportBatches.Create(ctx, b))
		got, err := env.ImportBatches.ByID(ctx, "h1", "b1")
		noError(t, err)
		b.Errors = []imports.RowError{}
		wantBatch(t, got, b)
	})

	t.Run("List pages newest first within the wallets", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w2", "h1", "u1", "Drugi")))
		for _, b := range []imports.Batch{newBatch(env, "b1", "h1", "w1"), newBatch(env, "b2", "h1", "w2"), newBatch(env, "b3", "h1", "w1"),
			newBatch(env, "b4", "h1", "w1")} {
			noError(t, env.ImportBatches.Create(ctx, b))
		}
		f := usecase.ImportBatchFilter{HouseholdID: "h1", WalletIDs: []domain.ID{"w1"}}
		page, next, err := env.ImportBatches.List(ctx, f, usecase.Page{Limit: 2})
		noError(t, err)
		sameOrder(t, "first page", batchIDs(page), []domain.ID{"b4", "b3"})
		if next == "" {
			t.Fatal("no next cursor")
		}
		page, next, err = env.ImportBatches.List(ctx, f, usecase.Page{Cursor: next, Limit: 2})
		noError(t, err)
		sameOrder(t, "second page", batchIDs(page), []domain.ID{"b1"})
		if next != "" {
			t.Errorf("next = %q on the last page", next)
		}
		all, _, err := env.ImportBatches.List(ctx, usecase.ImportBatchFilter{HouseholdID: "h1", WalletIDs: []domain.ID{"w1", "w2"}}, usecase.Page{Limit: 10})
		noError(t, err)
		sameOrder(t, "both wallets", batchIDs(all), []domain.ID{"b4", "b3", "b2", "b1"})
		none, _, err := env.ImportBatches.List(ctx, usecase.ImportBatchFilter{HouseholdID: "h1"}, usecase.Page{Limit: 10})
		noError(t, err)
		sameOrder(t, "no wallets", batchIDs(none), []domain.ID{})
		_, _, err = env.ImportBatches.List(ctx, usecase.ImportBatchFilter{HouseholdID: "h1", WalletIDs: []domain.ID{"w2"}}, usecase.Page{Cursor: "garbage", Limit: 2})
		wantError(t, err, domain.ErrInvalid)
	})

	t.Run("Transactions of a batch: linked, deleted, cascaded", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.ImportBatches.Create(ctx, newBatch(env, "b1", "h1", "w1")))
		for _, id := range []domain.ID{"t2", "t1", "t3"} {
			noError(t, env.Transactions.Create(ctx, newTransaction(env, id, "h1", "w1", "2026-10-20", -100)))
		}
		noError(t, env.ImportBatches.AddTransactions(ctx, "b1", []domain.ID{"t2", "t1", "t3"}))
		ids, err := env.ImportBatches.TransactionIDs(ctx, "h1", "b1")
		noError(t, err)
		sameOrder(t, "linked", ids, []domain.ID{"t1", "t2", "t3"})
		_, err = env.ImportBatches.TransactionIDs(ctx, "h2", "b1")
		wantError(t, err, domain.ErrNotFound)

		noError(t, env.Transactions.Delete(ctx, "h1", "t2"))
		ids, err = env.ImportBatches.TransactionIDs(ctx, "h1", "b1")
		noError(t, err)
		sameOrder(t, "after a deletion", ids, []domain.ID{"t1", "t3"})

		noError(t, env.Households.Delete(ctx, "h1"))
		_, err = env.ImportBatches.ByID(ctx, "h1", "b1")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("ExternalIDs finds the wallet's ids only", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w2", "h1", "u1", "Drugi")))
		for id, w := range map[domain.ID]domain.ID{"t1": "w1", "t2": "w1", "t3": "w2"} {
			tx := newTransaction(env, id, "h1", w, "2026-10-20", -100)
			tx.ExternalID = "ext-" + string(id)
			noError(t, env.Transactions.Create(ctx, tx))
		}
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t4", "h1", "w1", "2026-10-20", -100)))
		got, err := env.Transactions.ExternalIDs(ctx, "h1", "w1", []string{"ext-t1", "ext-t3", "nope", ""})
		noError(t, err)
		if !reflect.DeepEqual(got, map[string]domain.ID{"ext-t1": "t1"}) {
			t.Errorf("ExternalIDs = %v", got)
		}
		got, err = env.Transactions.ExternalIDs(ctx, "h2", "w1", []string{"ext-t1"})
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("another household's = %v", got)
		}
		got, err = env.Transactions.ExternalIDs(ctx, "h1", "w1", nil)
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("no ids = %v", got)
		}
	})
}

func newConnection(env Env, id, householdID domain.ID, name string) connection.Connection {
	return connection.Connection{ID: id, HouseholdID: householdID, Provider: "fake", Kind: connection.Bank, Status: connection.Active,
		DisplayName: name, SealedCredentials: []byte("sealed"), CreatedBy: "u1", CreatedAt: now(env), UpdatedAt: now(env)}
}

func wantConnection(t *testing.T, got, want connection.Connection) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) || !got.LastSyncAt.Equal(want.LastSyncAt) {
		t.Errorf("times = %v %v %v, want %v %v %v", got.CreatedAt, got.UpdatedAt, got.LastSyncAt, want.CreatedAt, want.UpdatedAt, want.LastSyncAt)
	}
	got.CreatedAt, got.UpdatedAt, got.LastSyncAt = want.CreatedAt, want.UpdatedAt, want.LastSyncAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("connection =\n %+v\nwant\n %+v", got, want)
	}
}

// RunConnectionSuite checks a ConnectionRepository. It needs Users,
// Households and Wallets.
func RunConnectionSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back, update, list", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		c := newConnection(env, "c1", "h1", "mBank")
		noError(t, env.Connections.Create(ctx, c))
		got, err := env.Connections.ByID(ctx, "h1", "c1")
		noError(t, err)
		wantConnection(t, got, c)
		_, err = env.Connections.ByID(ctx, "h2", "c1")
		wantError(t, err, domain.ErrNotFound)

		c.Status, c.DisplayName, c.SealedCredentials = connection.Error, "Konto mBank", nil
		c.LastSyncAt, c.LastError, c.UpdatedAt = now(env).Add(time.Hour), "timeout", now(env).Add(2*time.Hour)
		noError(t, env.Connections.Update(ctx, c))
		got, err = env.Connections.ByID(ctx, "h1", "c1")
		noError(t, err)
		wantConnection(t, got, c)
		other := c
		other.HouseholdID = "h2"
		wantError(t, env.Connections.Update(ctx, other), domain.ErrNotFound)

		noError(t, env.Connections.Create(ctx, newConnection(env, "c2", "h1", "allegro")))
		noError(t, env.Connections.Create(ctx, newConnection(env, "c3", "h2", "Obce")))
		list, err := env.Connections.ListByHousehold(ctx, "h1")
		noError(t, err)
		ids := []domain.ID{}
		for _, c := range list {
			ids = append(ids, c.ID)
		}
		sameOrder(t, "connections", ids, []domain.ID{"c2", "c1"})
	})

	t.Run("Accounts: upsert, list, scoped, cascaded", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.Connections.Create(ctx, newConnection(env, "c1", "h1", "mBank")))
		noError(t, env.Connections.PutAccount(ctx, "h1", connection.Account{ConnectionID: "c1", ExternalID: "b", Name: "Oszczędności"}))
		noError(t, env.Connections.PutAccount(ctx, "h1", connection.Account{ConnectionID: "c1", ExternalID: "a", Name: "eKonto", Currency: "PLN"}))
		noError(t, env.Connections.PutAccount(ctx, "h1", connection.Account{ConnectionID: "c1", ExternalID: "a", Name: "eKonto", Currency: "PLN", WalletID: "w1"}))
		wantError(t, env.Connections.PutAccount(ctx, "h2", connection.Account{ConnectionID: "c1", ExternalID: "x"}), domain.ErrNotFound)
		got, err := env.Connections.ListAccounts(ctx, "h1", "c1")
		noError(t, err)
		want := []connection.Account{{ConnectionID: "c1", ExternalID: "a", Name: "eKonto", Currency: "PLN", WalletID: "w1"},
			{ConnectionID: "c1", ExternalID: "b", Name: "Oszczędności"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("accounts = %+v", got)
		}
		_, err = env.Connections.ListAccounts(ctx, "h2", "c1")
		wantError(t, err, domain.ErrNotFound)

		noError(t, env.Households.Delete(ctx, "h1"))
		_, err = env.Connections.ByID(ctx, "h1", "c1")
		wantError(t, err, domain.ErrNotFound)
	})
}
