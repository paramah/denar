package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/payee"
)

func newPayee(env Env, id, householdID domain.ID, name string) payee.Payee {
	return payee.Payee{ID: id, HouseholdID: householdID, Name: name, Patterns: []string{}, CreatedAt: now(env), UpdatedAt: now(env)}
}

func wantPayee(t *testing.T, got, want payee.Payee) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("times = %v %v, want %v %v", got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	}
	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("payee = %+v, want %+v", got, want)
	}
}

// RunPayeeSuite checks a PayeeRepository. It needs Users, Households,
// Wallets and Transactions.
func RunPayeeSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back, update", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		p := newPayee(env, "p1", "h1", "Lidl")
		p.Patterns, p.DefaultEnvelopeID = []string{"LIDL SP", "lidl"}, "e1"
		noError(t, env.Payees.Create(ctx, p))
		got, err := env.Payees.ByID(ctx, "h1", "p1")
		noError(t, err)
		wantPayee(t, got, p)
		_, err = env.Payees.ByID(ctx, "h2", "p1")
		wantError(t, err, domain.ErrNotFound)

		p.Name, p.Patterns, p.DefaultEnvelopeID, p.UpdatedAt = "Lidl PL", []string{}, "", now(env).Add(time.Hour)
		noError(t, env.Payees.Update(ctx, p))
		got, err = env.Payees.ByID(ctx, "h1", "p1")
		noError(t, err)
		wantPayee(t, got, p)
		wantError(t, env.Payees.Update(ctx, newPayee(env, "p1", "h2", "X")), domain.ErrNotFound)
	})

	t.Run("Names are unique per household, ignoring case", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p1", "h1", "Lidl")))
		wantConflict(t, env.Payees.Create(ctx, newPayee(env, "p2", "h1", "LIDL")), domain.ConflictDuplicateName)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p3", "h2", "Lidl")))
		noError(t, env.Payees.Create(ctx, newPayee(env, "p4", "h1", "Aldi")))
		wantConflict(t, env.Payees.Update(ctx, newPayee(env, "p4", "h1", "lidl")), domain.ConflictDuplicateName)
		list, err := env.Payees.ListByHousehold(ctx, "h1")
		noError(t, err)
		ids := []domain.ID{}
		for _, p := range list {
			ids = append(ids, p.ID)
		}
		sameOrder(t, "payees", ids, []domain.ID{"p4", "p1"})
	})

	t.Run("Delete takes the payee off its transactions", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		noError(t, env.Payees.Create(ctx, newPayee(env, "p1", "h1", "Lidl")))
		tx := newTransaction(env, "t1", "h1", "w1", "2026-10-20", -1000)
		tx.PayeeID = "p1"
		noError(t, env.Transactions.Create(ctx, tx))
		wantError(t, env.Payees.Delete(ctx, "h2", "p1"), domain.ErrNotFound)
		noError(t, env.Payees.Delete(ctx, "h1", "p1"))
		_, err := env.Payees.ByID(ctx, "h1", "p1")
		wantError(t, err, domain.ErrNotFound)
		got, err := env.Transactions.ByID(ctx, "h1", "t1")
		noError(t, err)
		if got.PayeeID != "" {
			t.Errorf("payee = %q", got.PayeeID)
		}
		wantError(t, env.Payees.Delete(ctx, "h1", "p1"), domain.ErrNotFound)
	})
}
