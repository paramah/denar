package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

func newWallet(env Env, id, householdID, creator domain.ID, name string) wallet.Wallet {
	return wallet.Wallet{ID: id, HouseholdID: householdID, Name: name, Type: wallet.TypeChecking, Currency: "PLN",
		OnBudget: true, Visibility: wallet.Shared, MemberIDs: []domain.ID{}, CreatedBy: creator, CreatedAt: now(env)}
}

func wantWallet(t *testing.T, got, want wallet.Wallet) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.ArchivedAt.Equal(want.ArchivedAt) {
		t.Errorf("times = %v %v, want %v %v", got.CreatedAt, got.ArchivedAt, want.CreatedAt, want.ArchivedAt)
	}
	got.CreatedAt, got.ArchivedAt = want.CreatedAt, want.ArchivedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wallet = %+v, want %+v", got, want)
	}
}

// seedWallets creates users u1, u2, households h1 and h2 of u1 and the
// wallet w1 "Konto" in h1.
func seedWallets(t *testing.T, env Env) wallet.Wallet {
	t.Helper()
	ctx := context.Background()
	seedHousehold(t, env, "u2")
	noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
	w := newWallet(env, "w1", "h1", "u1", "Konto")
	noError(t, env.Wallets.Create(ctx, w))
	return w
}

// RunWalletSuite checks a WalletRepository. It needs Users and Households.
func RunWalletSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back, scoped to the household", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		p := newWallet(env, "w2", "h1", "u1", "Prywatny")
		p.Visibility, p.MemberIDs, p.Currency, p.Type, p.OnBudget = wallet.Private, []domain.ID{"u2", "u1"}, "USD", wallet.TypeCash, false
		noError(t, env.Wallets.Create(ctx, p))
		got, err := env.Wallets.ByID(ctx, "h1", "w2")
		noError(t, err)
		p.MemberIDs = []domain.ID{"u1", "u2"}
		wantWallet(t, got, p)
		_, err = env.Wallets.ByID(ctx, "h2", "w2")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Wallets.ByID(ctx, "h1", "nope")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Names are unique per household, ignoring case", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		wantConflict(t, env.Wallets.Create(ctx, newWallet(env, "w2", "h1", "u1", "KONTO")), domain.ConflictDuplicateName)
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w3", "h2", "u1", "Konto")))
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w4", "h1", "u1", "Gotówka")))
		taken := newWallet(env, "w4", "h1", "u1", "konto")
		wantConflict(t, env.Wallets.Update(ctx, taken), domain.ConflictDuplicateName)
	})

	t.Run("Update stores the mutable fields only", func(t *testing.T) {
		env := factory(t)
		w := seedWallets(t, env)
		changed := w
		changed.Name, changed.Type, changed.OnBudget, changed.Visibility, changed.MemberIDs = "Nowe", wallet.TypeSavings, false, wallet.Private, []domain.ID{"u1", "u2"}
		changed.ArchivedAt = now(env).Add(time.Hour)
		changed.Currency, changed.CreatedBy, changed.CreatedAt = "EUR", "u2", now(env).Add(time.Hour)
		noError(t, env.Wallets.Update(ctx, changed))
		got, err := env.Wallets.ByID(ctx, "h1", "w1")
		noError(t, err)
		want := changed
		want.Currency, want.CreatedBy, want.CreatedAt = w.Currency, w.CreatedBy, w.CreatedAt
		wantWallet(t, got, want)

		// Back to shared with no members, and out of the archive.
		changed.Visibility, changed.MemberIDs, changed.ArchivedAt = wallet.Shared, []domain.ID{}, time.Time{}
		noError(t, env.Wallets.Update(ctx, changed))
		got, err = env.Wallets.ByID(ctx, "h1", "w1")
		noError(t, err)
		if len(got.MemberIDs) != 0 || got.Archived() {
			t.Errorf("wallet = %+v", got)
		}
		other := newWallet(env, "w1", "h2", "u1", "X")
		wantError(t, env.Wallets.Update(ctx, other), domain.ErrNotFound)
		wantError(t, env.Wallets.Update(ctx, newWallet(env, "nope", "h1", "u1", "Y")), domain.ErrNotFound)
	})

	t.Run("ListByHousehold orders by name and keeps archived wallets", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		b := newWallet(env, "w2", "h1", "u1", "bank")
		b.ArchivedAt = now(env)
		noError(t, env.Wallets.Create(ctx, b))
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w3", "h1", "u1", "Alior")))
		noError(t, env.Wallets.Create(ctx, newWallet(env, "w4", "h2", "u1", "Inny")))
		list, err := env.Wallets.ListByHousehold(ctx, "h1")
		noError(t, err)
		sameOrder(t, "wallets", walletIDsOf(list), []domain.ID{"w3", "w2", "w1"})
		list, err = env.Wallets.ListByHousehold(ctx, "nope")
		noError(t, err)
		if list == nil || len(list) != 0 {
			t.Errorf("empty list = %#v", list)
		}
	})
}

func walletIDsOf(ws []wallet.Wallet) []domain.ID {
	ids := []domain.ID{}
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	return ids
}
