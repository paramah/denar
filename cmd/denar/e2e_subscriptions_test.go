package main

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/usecase/fakes"
)

type subscriptionJSON struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	NextDue *string `json:"next_due"`
	Price   money   `json:"price"`
	Monthly money   `json:"monthly"`
}

type chargeJSON struct {
	ID               string  `json:"id"`
	SubscriptionName string  `json:"subscription_name"`
	DueDate          string  `json:"due_date"`
	Amount           money   `json:"amount"`
	Status           string  `json:"status"`
	TransactionID    *string `json:"transaction_id"`
}

// TestSubscriptionsEndToEnd is the walk of stage 004 on a clock of its
// own: OpenAI billed 20.00 USD on the 31st, its charges at the ends of the
// shorter months, a card payment in PLN found by the suggestions and
// confirmed, a price rise, the summary in PLN at the NBP rate, a trial
// with its reminder in the outbox, a charge the job finds missed, and a
// bot asking what renews soon.
func TestSubscriptionsEndToEnd(t *testing.T) {
	nbp := &fakeNBP{rates: map[string]string{}}
	nbp.set("usd", "2026-10-20", "4.0000")
	nbp.set("usd", "2026-10-30", "4.0500")
	nbpServer := httptest.NewServer(nbp)
	defer nbpServer.Close()
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_NBP_URL", nbpServer.URL)
	clock := fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC))
	e := startE2EWith(t, appOptions{clock: clock, noSubscriptionJobs: true})
	ctx := context.Background()

	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	api := "/api/households/" + h.ID
	var card wallet
	admin.must(201, "POST", api+"/wallets", `{"name":"Karta","type":"card","currency":"PLN"}`, &card)
	var group, envelope struct {
		ID string `json:"id"`
	}
	admin.must(201, "POST", api+"/budget/envelope-groups", `{"name":"Stałe"}`, &group)
	admin.must(201, "POST", api+"/budget/envelopes", `{"group_id":"`+group.ID+`","name":"Subskrypcje"}`, &envelope)

	var vendors struct {
		Items []struct{ Key, Name string } `json:"items"`
	}
	admin.must(200, "GET", "/api/vendors", "", &vendors)
	if len(vendors.Items) < 16 || vendors.Items[0].Key != "openai" || vendors.Items[0].Name != "OpenAI" {
		t.Fatalf("vendors = %+v", vendors)
	}

	var gpt subscriptionJSON
	admin.must(201, "POST", api+"/subscriptions", `{"name":"ChatGPT Plus","vendor":"openai","price":{"amount":"20.00","currency":"USD"},
		"cycle":{"unit":"month","anchor":"2026-01-31"},"wallet_id":"`+card.ID+`","envelope_id":"`+envelope.ID+`",
		"manage_url":"https://chatgpt.com/#settings"}`, &gpt)
	if gpt.Status != "active" || gpt.NextDue == nil || *gpt.NextDue != "2026-10-31" || gpt.Monthly.Amount != "20.00" || gpt.Monthly.Currency != "USD" {
		t.Fatalf("subscription = %+v", gpt)
	}
	charges := func(query string) []chargeJSON {
		t.Helper()
		var page struct {
			Items []chargeJSON `json:"items"`
		}
		admin.must(200, "GET", api+"/charges?"+query, "", &page)
		return page.Items
	}
	got := charges("subscription_id=" + gpt.ID)
	if len(got) != 3 || got[0].DueDate != "2026-10-31" || got[1].DueDate != "2026-11-30" || got[2].DueDate != "2026-12-31" ||
		got[1].Amount.Amount != "20.00" || got[1].Amount.Currency != "USD" || got[0].Status != "expected" {
		t.Fatalf("charges = %+v", got)
	}
	october := got[0].ID
	if r, err := e.app.runSubscriptions(ctx); err != nil || r.Created != 0 || r.Reminders != 0 {
		t.Fatalf("jobs on the first day = %+v %v", r, err)
	}

	// The card pays 81.00 PLN on the 31st: 20.00 USD at 4.05.
	var paid txn
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+card.ID+`","date":"2026-10-31",
		"amount":{"amount":"-81.00","currency":"PLN"},"payee_name":"OPENAI *CHATGPT SUBSCR"}`, &paid)
	var forTx struct {
		Items []struct {
			Score  int        `json:"score"`
			Charge chargeJSON `json:"charge"`
		} `json:"items"`
	}
	admin.must(200, "GET", api+"/transactions/"+paid.ID+"/charge-suggestions", "", &forTx)
	if len(forTx.Items) != 1 || forTx.Items[0].Charge.ID != october || forTx.Items[0].Score != 100 {
		t.Fatalf("charges for the payment = %+v", forTx)
	}
	var forCharge struct {
		Items []struct {
			Score       int `json:"score"`
			Transaction txn `json:"transaction"`
		} `json:"items"`
	}
	admin.must(200, "GET", api+"/charges/"+october+"/suggestions", "", &forCharge)
	if len(forCharge.Items) != 1 || forCharge.Items[0].Transaction.ID != paid.ID {
		t.Fatalf("payments for the charge = %+v", forCharge)
	}
	var matched chargeJSON
	admin.must(200, "POST", api+"/charges/"+october+"/match", `{"transaction_id":"`+paid.ID+`"}`, &matched)
	if matched.Status != "matched" || matched.TransactionID == nil || *matched.TransactionID != paid.ID {
		t.Fatalf("matched = %+v", matched)
	}

	// The price rises from today; the paid charge keeps its amount.
	admin.must(201, "POST", api+"/subscriptions/"+gpt.ID+"/prices", `{"effective_from":"2026-10-21","price":{"amount":"25.00","currency":"USD"}}`, &gpt)
	if gpt.Price.Amount != "25.00" {
		t.Fatalf("price = %+v", gpt.Price)
	}
	got = charges("subscription_id=" + gpt.ID)
	if got[0].Amount.Amount != "20.00" || got[0].Status != "matched" || got[1].Amount.Amount != "25.00" || got[2].Amount.Amount != "25.00" {
		t.Fatalf("charges after the rise = %+v", got)
	}

	var summary struct {
		Currency     string `json:"currency"`
		Monthly      money  `json:"monthly"`
		Yearly       money  `json:"yearly"`
		Incomplete   bool   `json:"incomplete"`
		PriceChanges []struct {
			OldPrice money `json:"old_price"`
			NewPrice money `json:"new_price"`
		} `json:"price_changes"`
		ByEnvelope []struct {
			EnvelopeID *string `json:"envelope_id"`
			Monthly    money   `json:"monthly"`
		} `json:"by_envelope"`
	}
	admin.must(200, "GET", api+"/subscriptions/summary", "", &summary)
	// 25.00 USD at 4.0000 (NBP of 20 October, the last before today).
	if summary.Currency != "PLN" || summary.Monthly.Amount != "100.00" || summary.Yearly.Amount != "1200.00" || summary.Incomplete ||
		len(summary.PriceChanges) != 1 || summary.PriceChanges[0].OldPrice.Amount != "20.00" || summary.PriceChanges[0].NewPrice.Amount != "25.00" ||
		len(summary.ByEnvelope) != 1 || *summary.ByEnvelope[0].EnvelopeID != envelope.ID {
		t.Fatalf("summary = %+v", summary)
	}

	// A trial ending on the 25th: a week's notice is due today.
	var claude subscriptionJSON
	admin.must(201, "POST", api+"/subscriptions", `{"name":"Claude Pro","vendor":"anthropic","price":{"amount":"20.00","currency":"USD"},
		"cycle":{"unit":"month","anchor":"2026-10-25"},"trial_ends_at":"2026-10-25","reminder_offsets":[7]}`, &claude)
	if claude.Status != "trial" {
		t.Fatalf("trial = %+v", claude)
	}
	r, err := e.app.runSubscriptions(ctx)
	if err != nil || r.Reminders != 1 {
		t.Fatalf("jobs = %+v %v", r, err)
	}
	pending, err := e.app.deps.Events.ListPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	types := map[event.Type]int{}
	for _, ev := range pending {
		types[ev.Type]++
		if ev.Type == event.SubscriptionTrialEnding && (ev.Payload["trial_ends_at"] != "2026-10-25" || ev.Payload["days_before"] != "7" ||
			ev.Payload["subscription_name"] != "Claude Pro") {
			t.Errorf("trial event = %+v", ev)
		}
	}
	if types[event.SubscriptionTrialEnding] != 1 || types[event.SubscriptionPriceChanged] != 1 || len(pending) != 2 {
		t.Fatalf("outbox = %+v", pending)
	}

	// Nobody paid the trial's first term by the end of its window.
	clock.Set(time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC))
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	nbp.set("usd", "2026-10-28", "4.0500")
	if r, err = e.app.runSubscriptions(ctx); err != nil || r.Missed != 1 {
		t.Fatalf("jobs of 29 October = %+v %v", r, err)
	}
	missed := charges("status=missed")
	if len(missed) != 1 || missed[0].SubscriptionName != "Claude Pro" || missed[0].DueDate != "2026-10-25" {
		t.Fatalf("missed = %+v", missed)
	}
	if r, _ = e.app.runSubscriptions(ctx); r.Missed != 0 {
		t.Errorf("missed again: %+v", r)
	}

	// A bot with subscriptions:read asks what renews soon.
	var created botWithToken
	admin.must(201, "POST", api+"/bots", `{"name":"Asystent","scopes":["subscriptions:read"]}`, &created)
	assistant := e.bot(created.Token)
	var upcoming struct {
		Items []struct {
			Charge     chargeJSON `json:"charge"`
			BaseAmount *money     `json:"base_amount"`
		} `json:"items"`
	}
	assistant.must(200, "GET", api+"/subscriptions/upcoming?days=40", "", &upcoming)
	if len(upcoming.Items) != 2 || upcoming.Items[0].Charge.SubscriptionName != "Claude Pro" || upcoming.Items[1].Charge.DueDate != "2026-11-30" ||
		upcoming.Items[1].BaseAmount == nil || upcoming.Items[1].BaseAmount.Amount != "101.25" {
		t.Fatalf("upcoming = %+v", upcoming)
	}
	assistant.must(403, "POST", api+"/charges/"+october+"/unmatch", "", nil)
}
