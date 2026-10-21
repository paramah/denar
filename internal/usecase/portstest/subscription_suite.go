package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func newSubscription(env Env, id, householdID domain.ID, name string) subscription.Subscription {
	return subscription.Subscription{ID: id, HouseholdID: householdID, Name: name, Vendor: "openai", Status: subscription.Active,
		Cycle:           subscription.Cycle{Unit: subscription.Month, Every: 1, Anchor: day("2026-01-31")},
		ReminderOffsets: []int{3}, Match: subscription.MatchRule{AmountTolerancePct: 10, DateWindowDays: 3},
		Prices:    []subscription.PricePoint{{ID: "pp-" + id, EffectiveFrom: day("2026-01-31"), Amount: 2000, Currency: "USD", CreatedAt: now(env)}},
		CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "u1"}, CreatedAt: now(env), UpdatedAt: now(env)}
}

func newCharge(env Env, id, householdID, subscriptionID domain.ID, due string) subscription.Charge {
	return subscription.Charge{ID: id, HouseholdID: householdID, SubscriptionID: subscriptionID, DueDate: day(due), Amount: 2000,
		Currency: "USD", Status: subscription.Expected, CreatedAt: now(env), UpdatedAt: now(env)}
}

func wantSubscription(t *testing.T, got, want subscription.Subscription) {
	t.Helper()
	sameTimes(t, got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	if !got.CancelledAt.Equal(want.CancelledAt) {
		t.Errorf("cancelled at = %v, want %v", got.CancelledAt, want.CancelledAt)
	}
	got.CancelledAt = want.CancelledAt
	for i := range got.Prices {
		if i < len(want.Prices) && got.Prices[i].CreatedAt.Equal(want.Prices[i].CreatedAt) {
			got.Prices[i].CreatedAt = want.Prices[i].CreatedAt
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("subscription = %+v, want %+v", got, want)
	}
}

// seedSubscriptions adds to seedBudget the subscriptions s1 "ChatGPT" (w1,
// e1) and s2 "Netflix" in h1, and s9 in h2.
func seedSubscriptions(t *testing.T, env Env) {
	t.Helper()
	ctx := context.Background()
	seedBudget(t, env)
	s1 := newSubscription(env, "s1", "h1", "ChatGPT")
	s1.WalletID, s1.EnvelopeID = "w1", "e1"
	noError(t, env.Subscriptions.Create(ctx, s1))
	s2 := newSubscription(env, "s2", "h1", "Netflix")
	s2.Vendor = "netflix"
	noError(t, env.Subscriptions.Create(ctx, s2))
	noError(t, env.Subscriptions.Create(ctx, newSubscription(env, "s9", "h2", "Obca")))
}

// RunSubscriptionSuite checks a SubscriptionRepository. It needs Users,
// Households, Wallets, EnvelopeGroups, Envelopes and Charges.
func RunSubscriptionSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back, scoped to the household", func(t *testing.T) {
		env := factory(t)
		seedBudget(t, env)
		s := newSubscription(env, "s1", "h1", "ChatGPT Plus")
		s.ExternalRef, s.TrialEndsAt, s.Status, s.WalletID, s.EnvelopeID = "sub_123", day("2026-02-14"), subscription.Trial, "w1", "e1"
		s.ReminderOffsets, s.Note, s.ManageURL = []int{7, 1}, "konto firmowe", "https://chatgpt.com/account"
		s.Match = subscription.MatchRule{PayeePattern: "OPENAI", AmountTolerancePct: 5, DateWindowDays: 2}
		s.Prices = append(s.Prices, subscription.PricePoint{ID: "pp2", EffectiveFrom: day("2026-06-01"), Amount: 2200, Currency: "USD", CreatedAt: now(env)})
		noError(t, env.Subscriptions.Create(ctx, s))
		got, err := env.Subscriptions.ByID(ctx, "h1", "s1")
		noError(t, err)
		wantSubscription(t, got, s)
		_, err = env.Subscriptions.ByID(ctx, "h2", "s1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Subscriptions.ByID(ctx, "h1", "nope")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Names are unique per household, ignoring case", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		wantConflict(t, env.Subscriptions.Create(ctx, newSubscription(env, "s3", "h1", "chatgpt")), domain.ConflictDuplicateName)
		noError(t, env.Subscriptions.Create(ctx, newSubscription(env, "s4", "h2", "ChatGPT")))
		taken := newSubscription(env, "s2", "h1", "CHATGPT")
		wantConflict(t, env.Subscriptions.Update(ctx, taken), domain.ConflictDuplicateName)
	})

	t.Run("Update stores the mutable fields, not the prices", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		old, err := env.Subscriptions.ByID(ctx, "h1", "s1")
		noError(t, err)
		changed := old
		changed.Name, changed.Vendor, changed.ExternalRef = "GPT", "anthropic", "x"
		changed.Cycle = subscription.Cycle{Unit: subscription.Year, Every: 2, Anchor: day("2026-03-01")}
		changed.Status, changed.CancelledAt, changed.EndsAt = subscription.Cancelled, now(env).Add(time.Hour), day("2026-12-01")
		changed.WalletID, changed.EnvelopeID, changed.ReminderOffsets = "", "e2", []int{}
		changed.Match, changed.Note, changed.ManageURL = subscription.MatchRule{AmountTolerancePct: 0, DateWindowDays: 0}, "n", "http://x.test"
		changed.UpdatedAt = now(env).Add(time.Hour)
		changed.Prices = nil
		changed.CreatedBy, changed.CreatedAt = domain.Actor{Kind: domain.ActorBot, ID: "b"}, now(env).Add(time.Hour)
		noError(t, env.Subscriptions.Update(ctx, changed))
		got, err := env.Subscriptions.ByID(ctx, "h1", "s1")
		noError(t, err)
		want := changed
		want.Prices, want.CreatedBy, want.CreatedAt = old.Prices, old.CreatedBy, old.CreatedAt
		wantSubscription(t, got, want)
		wantError(t, env.Subscriptions.Update(ctx, newSubscription(env, "s9", "h1", "Inna")), domain.ErrNotFound)
	})

	t.Run("PutPrice adds a point and replaces one of the same day", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		p := subscription.PricePoint{ID: "pp-b", EffectiveFrom: day("2026-07-01"), Amount: 2500, Currency: "USD", CreatedAt: now(env)}
		noError(t, env.Subscriptions.PutPrice(ctx, "h1", "s1", p))
		p2 := subscription.PricePoint{ID: "pp-c", EffectiveFrom: day("2026-03-01"), Amount: 9900, Currency: "PLN", CreatedAt: now(env)}
		noError(t, env.Subscriptions.PutPrice(ctx, "h1", "s1", p2))
		fixed := subscription.PricePoint{ID: "pp-d", EffectiveFrom: day("2026-07-01"), Amount: 2400, Currency: "USD", CreatedAt: now(env)}
		noError(t, env.Subscriptions.PutPrice(ctx, "h1", "s1", fixed))
		got, err := env.Subscriptions.ByID(ctx, "h1", "s1")
		noError(t, err)
		var ids []domain.ID
		for _, q := range got.Prices {
			ids = append(ids, q.ID)
		}
		sameOrder(t, "prices", ids, []domain.ID{"pp-s1", "pp-c", "pp-d"})
		if got.Prices[2].Amount != 2400 || got.Prices[1].Currency != "PLN" {
			t.Errorf("prices = %+v", got.Prices)
		}
		wantError(t, env.Subscriptions.PutPrice(ctx, "h2", "s1", p), domain.ErrNotFound)
	})

	t.Run("List orders by name; all households without one", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		noError(t, env.Subscriptions.Create(ctx, newSubscription(env, "s3", "h1", "apple one")))
		list, err := env.Subscriptions.List(ctx, "h1")
		noError(t, err)
		var ids []domain.ID
		for _, s := range list {
			ids = append(ids, s.ID)
		}
		sameOrder(t, "list", ids, []domain.ID{"s3", "s1", "s2"})
		all, err := env.Subscriptions.List(ctx, "")
		noError(t, err)
		if len(all) != 4 {
			t.Errorf("all = %d", len(all))
		}
	})

	t.Run("CountByEnvelope and Delete with the charges", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		n, err := env.Subscriptions.CountByEnvelope(ctx, "h1", "e1")
		noError(t, err)
		if n != 1 {
			t.Errorf("count = %d", n)
		}
		n, _ = env.Subscriptions.CountByEnvelope(ctx, "h1", "e2")
		if n != 0 {
			t.Errorf("count e2 = %d", n)
		}
		noError(t, env.Charges.Create(ctx, newCharge(env, "c1", "h1", "s1", "2026-10-31")))
		noError(t, env.Subscriptions.Delete(ctx, "h1", "s1"))
		_, err = env.Subscriptions.ByID(ctx, "h1", "s1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Charges.ByID(ctx, "h1", "c1")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.Subscriptions.Delete(ctx, "h1", "s1"), domain.ErrNotFound)
		wantError(t, env.Subscriptions.Delete(ctx, "h1", "s9"), domain.ErrNotFound)
	})
}

// RunChargeSuite checks a ChargeRepository. It needs Users, Households,
// Wallets, Transactions, EnvelopeGroups, Envelopes and Subscriptions.
func RunChargeSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read back, one per subscription and day", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		c := newCharge(env, "c1", "h1", "s1", "2026-10-31")
		noError(t, env.Charges.Create(ctx, c))
		got, err := env.Charges.ByID(ctx, "h1", "c1")
		noError(t, err)
		sameTimes(t, got.CreatedAt, got.UpdatedAt, c.CreatedAt, c.UpdatedAt)
		got.CreatedAt, got.UpdatedAt = c.CreatedAt, c.UpdatedAt
		if got != c {
			t.Errorf("charge = %+v, want %+v", got, c)
		}
		_, err = env.Charges.ByID(ctx, "h2", "c1")
		wantError(t, err, domain.ErrNotFound)
		wantConflict(t, env.Charges.Create(ctx, newCharge(env, "c2", "h1", "s1", "2026-10-31")), domain.ConflictDuplicateCharge)
		noError(t, env.Charges.Create(ctx, newCharge(env, "c3", "h1", "s2", "2026-10-31")))
	})

	t.Run("Update links a transaction, one charge per transaction", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t1", "h1", "w1", "2026-10-31", -8000)))
		c1, c2 := newCharge(env, "c1", "h1", "s1", "2026-10-31"), newCharge(env, "c2", "h1", "s2", "2026-10-31")
		noError(t, env.Charges.Create(ctx, c1))
		noError(t, env.Charges.Create(ctx, c2))
		c1.Status, c1.TransactionID, c1.Amount, c1.Currency, c1.UpdatedAt = subscription.Matched, "t1", 2100, "EUR", now(env).Add(time.Hour)
		c1.DueDate = day("2027-01-01") // does not change
		noError(t, env.Charges.Update(ctx, c1))
		got, err := env.Charges.ByTransaction(ctx, "h1", "t1")
		noError(t, err)
		if got.ID != "c1" || got.Status != subscription.Matched || got.Amount != 2100 || got.Currency != "EUR" ||
			got.DueDate != day("2026-10-31") || !got.UpdatedAt.Equal(c1.UpdatedAt) {
			t.Errorf("charge = %+v", got)
		}
		c2.Status, c2.TransactionID = subscription.Posted, "t1"
		wantConflict(t, env.Charges.Update(ctx, c2), domain.ConflictTransactionLinked)
		_, err = env.Charges.ByTransaction(ctx, "h2", "t1")
		wantError(t, err, domain.ErrNotFound)
		c1.Status, c1.TransactionID = subscription.Expected, ""
		noError(t, env.Charges.Update(ctx, c1))
		_, err = env.Charges.ByTransaction(ctx, "h1", "t1")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.Charges.Update(ctx, newCharge(env, "nope", "h1", "s1", "2026-10-31")), domain.ErrNotFound)
	})

	t.Run("List filters and pages by due date", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		for i, due := range []string{"2026-10-31", "2026-11-30", "2026-12-31", "2027-01-31"} {
			c := newCharge(env, domain.ID("a"+string(rune('0'+i))), "h1", "s1", due)
			if i == 1 {
				c.Status = subscription.Skipped
			}
			noError(t, env.Charges.Create(ctx, c))
		}
		noError(t, env.Charges.Create(ctx, newCharge(env, "b0", "h1", "s2", "2026-11-30")))
		noError(t, env.Charges.Create(ctx, newCharge(env, "z9", "h2", "s9", "2026-11-30")))
		f := usecase.ChargeFilter{HouseholdID: "h1", SubscriptionIDs: []domain.ID{"s1", "s2"}}
		var seen []domain.ID
		cursor := ""
		for range 10 {
			page, next, err := env.Charges.List(ctx, f, usecase.Page{Cursor: cursor, Limit: 2})
			noError(t, err)
			for _, c := range page {
				seen = append(seen, c.ID)
			}
			if cursor = next; cursor == "" {
				break
			}
		}
		sameOrder(t, "pages", seen, []domain.ID{"a0", "a1", "b0", "a2", "a3"})
		f.Statuses, f.From, f.To = []subscription.ChargeStatus{subscription.Expected}, day("2026-11-01"), day("2026-12-31")
		got, _, err := env.Charges.List(ctx, f, usecase.Page{Limit: 10})
		noError(t, err)
		sameOrder(t, "filtered", ids(got), []domain.ID{"b0", "a2"})
		got, _, err = env.Charges.List(ctx, usecase.ChargeFilter{HouseholdID: "h1"}, usecase.Page{Limit: 10})
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("no subscriptions = %v", got)
		}
		_, _, err = env.Charges.List(ctx, f, usecase.Page{Cursor: "garbage", Limit: 2})
		wantError(t, err, domain.ErrInvalid)
	})

	t.Run("ListBySubscription, ListOpen and Delete", func(t *testing.T) {
		env := factory(t)
		seedSubscriptions(t, env)
		noError(t, env.Transactions.Create(ctx, newTransaction(env, "t1", "h1", "w1", "2026-09-30", -8000)))
		matched := newCharge(env, "c0", "h1", "s1", "2026-09-30")
		matched.Status, matched.TransactionID = subscription.Matched, "t1"
		noError(t, env.Charges.Create(ctx, matched))
		missed := newCharge(env, "c1", "h1", "s1", "2026-10-31")
		missed.Status = subscription.Missed
		noError(t, env.Charges.Create(ctx, missed))
		noError(t, env.Charges.Create(ctx, newCharge(env, "c2", "h1", "s1", "2026-11-30")))
		noError(t, env.Charges.Create(ctx, newCharge(env, "c9", "h2", "s9", "2026-10-31")))
		got, err := env.Charges.ListBySubscription(ctx, "h1", "s1", day("2026-10-01"))
		noError(t, err)
		sameOrder(t, "by subscription", ids(got), []domain.ID{"c1", "c2"})
		got, _ = env.Charges.ListBySubscription(ctx, "h1", "s1", domain.Date{})
		sameOrder(t, "all of it", ids(got), []domain.ID{"c0", "c1", "c2"})
		got, err = env.Charges.ListOpen(ctx, "", domain.Date{}, day("2026-11-30"), 10)
		noError(t, err)
		sameOrder(t, "open", ids(got), []domain.ID{"c1", "c9", "c2"})
		got, _ = env.Charges.ListOpen(ctx, "h1", day("2026-11-01"), domain.Date{}, 10)
		sameOrder(t, "open of h1 from", ids(got), []domain.ID{"c2"})
		got, _ = env.Charges.ListOpen(ctx, "", domain.Date{}, domain.Date{}, 1)
		sameOrder(t, "limited", ids(got), []domain.ID{"c1"})
		noError(t, env.Charges.Delete(ctx, "h1", "c2"))
		wantError(t, env.Charges.Delete(ctx, "h1", "c2"), domain.ErrNotFound)
		wantError(t, env.Charges.Delete(ctx, "h1", "c9"), domain.ErrNotFound)
	})
}

func ids(cs []subscription.Charge) []domain.ID {
	out := []domain.ID{}
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func newEvent(env Env, id, householdID domain.ID, key string) event.Event {
	return event.Event{ID: id, HouseholdID: householdID, Type: event.SubscriptionRenewalUpcoming, ResourceType: event.ResourceSubscription,
		ResourceID: "s1", DedupeKey: key, Payload: map[string]string{"due_date": "2026-10-31", "amount": "20.00"}, OccurredAt: now(env)}
}

// RunEventOutboxSuite checks an EventOutbox. It needs Users and
// Households.
func RunEventOutboxSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Append once per key and household, list pending, mark dispatched", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		e1 := newEvent(env, "e1", "h1", "renewal:c1:3")
		stored, err := env.Events.Append(ctx, e1)
		noError(t, err)
		if !stored {
			t.Error("first append not stored")
		}
		stored, err = env.Events.Append(ctx, newEvent(env, "e2", "h1", "renewal:c1:3"))
		noError(t, err)
		if stored {
			t.Error("duplicate key stored")
		}
		stored, _ = env.Events.Append(ctx, newEvent(env, "e3", "h2", "renewal:c1:3"))
		if !stored {
			t.Error("same key in another household not stored")
		}
		_, _ = env.Events.Append(ctx, newEvent(env, "e4", "h1", "renewal:c1:0"))
		pending, err := env.Events.ListPending(ctx, 10)
		noError(t, err)
		var got []domain.ID
		for _, e := range pending {
			got = append(got, e.ID)
		}
		sameOrder(t, "pending", got, []domain.ID{"e1", "e3", "e4"})
		first := pending[0]
		if !first.OccurredAt.Equal(e1.OccurredAt) || !reflect.DeepEqual(first.Payload, e1.Payload) || first.Type != e1.Type ||
			first.ResourceID != "s1" || first.ResourceType != "subscription" || first.DedupeKey != e1.DedupeKey || first.Dispatched() {
			t.Errorf("event = %+v", first)
		}
		noError(t, env.Events.MarkDispatched(ctx, "e1", now(env)))
		wantError(t, env.Events.MarkDispatched(ctx, "nope", now(env)), domain.ErrNotFound)
		pending, _ = env.Events.ListPending(ctx, 1)
		if len(pending) != 1 || pending[0].ID != "e3" {
			t.Errorf("pending after dispatch = %+v", pending)
		}
		stored, _ = env.Events.Append(ctx, newEvent(env, "e5", "h1", "renewal:c1:3"))
		if stored {
			t.Error("a dispatched key is stored again")
		}
	})
}
