package portstest

import (
	"context"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/transaction"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

func newHousehold(env Env, id, creator domain.ID) household.Household {
	return household.Household{
		ID: id, Name: "Dom " + string(id), BaseCurrency: "PLN", Timezone: "Europe/Warsaw",
		MonthStartDay: 1, CreatedBy: creator, CreatedAt: now(env),
	}
}

func wantHousehold(t *testing.T, got, want household.Household) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	got.CreatedAt = want.CreatedAt
	if got != want {
		t.Errorf("household = %+v, want %+v", got, want)
	}
}

// seedHousehold creates the users and a household h1 of u1.
func seedHousehold(t *testing.T, env Env, users ...domain.ID) household.Household {
	t.Helper()
	ctx := context.Background()
	for _, id := range append([]domain.ID{"u1"}, users...) {
		noError(t, env.Users.Create(ctx, newUser(env, id)))
	}
	h := newHousehold(env, "h1", "u1")
	noError(t, env.Households.Create(ctx, h))
	return h
}

// RunHouseholdSuite checks a HouseholdRepository. It needs Users, and for
// the cascade Memberships, Invitations, Bots and Audit.
func RunHouseholdSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back", func(t *testing.T) {
		env := factory(t)
		h := seedHousehold(t, env)
		got, err := env.Households.ByID(ctx, "h1")
		noError(t, err)
		wantHousehold(t, got, h)
		_, err = env.Households.ByID(ctx, "nope")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Update stores name, time zone and start day only", func(t *testing.T) {
		env := factory(t)
		h := seedHousehold(t, env, "u2")
		changed := h
		changed.Name, changed.Timezone, changed.MonthStartDay = "Nowy", "UTC", 25
		changed.BaseCurrency, changed.CreatedBy, changed.CreatedAt = "EUR", "u2", h.CreatedAt.Add(time.Hour)
		noError(t, env.Households.Update(ctx, changed))
		got, err := env.Households.ByID(ctx, "h1")
		noError(t, err)
		want := h
		want.Name, want.Timezone, want.MonthStartDay = "Nowy", "UTC", 25
		wantHousehold(t, got, want)
		wantError(t, env.Households.Update(ctx, newHousehold(env, "nope", "u1")), domain.ErrNotFound)
	})

	t.Run("ByIDs skips unknown ids", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		got, err := env.Households.ByIDs(ctx, []domain.ID{"h2", "nope", "h1"})
		noError(t, err)
		ids := []domain.ID{}
		for _, h := range got {
			ids = append(ids, h.ID)
		}
		sameSet(t, "ByIDs", ids, []domain.ID{"h1", "h2"})
		got, err = env.Households.ByIDs(ctx, nil)
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("ByIDs(nil) = %v", got)
		}
	})

	t.Run("Delete removes the household and what belongs to it", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		other := newHousehold(env, "h2", "u2")
		noError(t, env.Households.Create(ctx, other))
		for _, hid := range []domain.ID{"h1", "h2"} {
			noError(t, env.Memberships.Create(ctx, membership.Membership{HouseholdID: hid, UserID: "u1", Role: membership.RoleOwner, CreatedAt: now(env)}))
			noError(t, env.Invitations.Create(ctx, newInvitation(env, "i-"+hid, hid, "u1")))
			noError(t, env.Bots.Create(ctx, newBot(env, "b-"+hid, hid, "u1", "Bot")))
			noError(t, env.Audit.Append(ctx, newEntry(env, "a-"+hid, hid, domain.Actor{Kind: domain.ActorUser, ID: "u1"}, audit.ResourceHousehold)))
			if env.Wallets != nil {
				w := newWallet(env, "w-"+hid, hid, "u1", "Konto")
				w.Visibility, w.MemberIDs = wallet.Private, []domain.ID{"u1"}
				noError(t, env.Wallets.Create(ctx, w))
				noError(t, env.Payees.Create(ctx, newPayee(env, "p-"+hid, hid, "Lidl")))
				tx := newTransaction(env, "t-"+hid, hid, "w-"+hid, "2026-10-20", -1)
				tx.PayeeID, tx.Splits = "p-"+hid, []transaction.Split{{Amount: -1, BaseAmount: -1}}
				noError(t, env.Transactions.Create(ctx, tx))
				noError(t, env.FxOverrides.Put(ctx, usecase.FxOverride{HouseholdID: hid, Currency: "USD", Date: day("2026-10-20"),
					Rate: "4", CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "u1"}, CreatedAt: now(env)}))
			}
			if env.Envelopes != nil {
				noError(t, env.EnvelopeGroups.Create(ctx, newGroup(env, "g-"+hid, hid, "Grupa", 0)))
				noError(t, env.Envelopes.Create(ctx, newEnvelope(env, "e-"+hid, hid, "g-"+hid, "Koperta", 0)))
				noError(t, env.Assignments.Set(ctx, budget.Assignment{HouseholdID: hid, EnvelopeID: "e-" + hid, Month: month("2026-10"), Amount: 5, UpdatedAt: now(env)}))
			}
			if env.Subscriptions != nil {
				s := newSubscription(env, "s-"+hid, hid, "Netflix")
				s.WalletID, s.EnvelopeID = "w-"+hid, "e-"+hid
				noError(t, env.Subscriptions.Create(ctx, s))
				c := newCharge(env, "c-"+hid, hid, "s-"+hid, "2026-10-20")
				c.Status, c.TransactionID = subscription.Matched, "t-"+hid
				noError(t, env.Charges.Create(ctx, c))
				_, err := env.Events.Append(ctx, newEvent(env, "ev-"+hid, hid, "k"))
				noError(t, err)
			}
		}

		noError(t, env.Households.Delete(ctx, "h1"))
		_, err := env.Households.ByID(ctx, "h1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Memberships.Get(ctx, "h1", "u1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Invitations.ByID(ctx, "i-h1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Bots.ByID(ctx, "b-h1")
		wantError(t, err, domain.ErrNotFound)
		entries, _, err := env.Audit.List(ctx, auditFilter("h1"), firstPage(10))
		noError(t, err)
		if len(entries) != 0 {
			t.Errorf("audit of a deleted household = %v", entries)
		}

		if env.Wallets != nil {
			_, err = env.Wallets.ByID(ctx, "h1", "w-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Transactions.ByID(ctx, "h1", "t-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Payees.ByID(ctx, "h1", "p-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.FxOverrides.Get(ctx, "h1", "USD", day("2026-10-20"))
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Transactions.ByID(ctx, "h2", "t-h2")
			noError(t, err)
		}
		if env.Envelopes != nil {
			_, err = env.EnvelopeGroups.ByID(ctx, "h1", "g-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Envelopes.ByID(ctx, "h1", "e-h1")
			wantError(t, err, domain.ErrNotFound)
			left, err := env.Assignments.ListByHousehold(ctx, "h1")
			noError(t, err)
			kept, err := env.Assignments.ListByHousehold(ctx, "h2")
			noError(t, err)
			if len(left) != 0 || len(kept) != 1 {
				t.Errorf("assignments left %v, kept %v", left, kept)
			}
		}

		if env.Subscriptions != nil {
			_, err = env.Subscriptions.ByID(ctx, "h1", "s-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Charges.ByID(ctx, "h1", "c-h1")
			wantError(t, err, domain.ErrNotFound)
			_, err = env.Charges.ByID(ctx, "h2", "c-h2")
			noError(t, err)
			pending, err := env.Events.ListPending(ctx, 10)
			noError(t, err)
			if len(pending) != 1 || pending[0].HouseholdID != "h2" {
				t.Errorf("events left = %+v", pending)
			}
		}

		// The other household is untouched.
		_, err = env.Memberships.Get(ctx, "h2", "u1")
		noError(t, err)
		_, err = env.Bots.ByID(ctx, "b-h2")
		noError(t, err)
		wantError(t, env.Households.Delete(ctx, "h1"), domain.ErrNotFound)
	})
}
