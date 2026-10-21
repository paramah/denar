package usecase_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// subscriptionFamily is budgetFamily (today 2026-10-21) with the member's
// subscription openai "ChatGPT Plus": 20.00 USD a month from 31 January,
// paid from w1 (PLN) into the envelope food. NBP knows USD on 30 October
// and 30 November too.
func subscriptionFamily(t *testing.T) *family {
	f := budgetFamily(t)
	f.Rates.Set("USD", "2026-10-30", "4.0000")
	f.Rates.Set("USD", "2026-11-30", "4.1000")
	v := f.subscribe(usecase.CreateSubscriptionInput{Name: "ChatGPT Plus", Vendor: "openai", Price: usd("20.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-31")}, WalletID: "w1", EnvelopeID: string(f.vars["food"])})
	f.vars["gpt"] = v.Subscription.ID
	return f
}

func (f *family) subscribe(in usecase.CreateSubscriptionInput) usecase.SubscriptionView {
	f.t.Helper()
	v, err := usecase.NewCreateSubscription(f.deps).Do(ctx, human(f.member), "h1", in)
	f.must(err)
	return v
}

// charges returns the charges of the subscription as "date:status:amount".
func (f *family) charges(id domain.ID) string {
	f.t.Helper()
	cs, err := f.Charges.ListBySubscription(ctx, "h1", id, domain.Date{})
	f.must(err)
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.DueDate.String() + ":" + string(c.Status) + ":" + formatAmount(c.Amount, c.Currency)
	}
	return strings.Join(out, " ")
}

func formatAmount(amount int64, currency string) string {
	return usecase.FormatMinorForTest(amount, currency)
}

// charge returns the subscription's charge due on the day.
func (f *family) charge(id domain.ID, due string) subscription.Charge {
	f.t.Helper()
	cs, err := f.Charges.ListBySubscription(ctx, "h1", id, domain.Date{})
	f.must(err)
	for _, c := range cs {
		if c.DueDate == day(due) {
			return c
		}
	}
	f.t.Fatalf("no charge on %s: %s", due, f.charges(id))
	return subscription.Charge{}
}

func (f *family) events(typ event.Type) []event.Event {
	var out []event.Event
	for _, e := range f.Events.All() {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func TestCreateSubscription(t *testing.T) {
	f := subscriptionFamily(t)
	v, err := usecase.NewGetSubscription(f.deps).Do(ctx, human(f.viewer), "h1", f.vars["gpt"])
	noErr(t, err)
	s := v.Subscription
	if s.Status != subscription.Active || s.Vendor != "openai" || v.VendorName != "OpenAI" || v.NextDue != day("2026-10-31") ||
		v.CurrentPrice.Amount != 2000 || v.CurrentPrice.Currency != "USD" || v.Cost.Monthly != 2000 || v.Cost.Yearly != 24000 ||
		!slices.Equal(s.ReminderOffsets, []int{3}) || s.Match.AmountTolerancePct != 10 || s.Match.DateWindowDays != 3 ||
		s.Prices[0].EffectiveFrom != day("2026-01-31") || s.CreatedBy.ID != "member" {
		t.Errorf("view = %+v", v)
	}
	// From today less the window (18 October) to the horizon (19 January).
	if got := f.charges(s.ID); got != "2026-10-31:expected:20.00 USD 2026-11-30:expected:20.00 USD 2026-12-31:expected:20.00 USD" {
		t.Errorf("charges = %s", got)
	}
	if !slices.Contains(f.actions(), "subscription.created") {
		t.Errorf("actions = %v", f.actions())
	}

	create := usecase.NewCreateSubscription(f.deps)
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateSubscriptionInput{Name: "chatgpt plus", Vendor: "openai", Price: usd("1"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}})
	wantConflict(t, err, domain.ConflictDuplicateName)
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateSubscriptionInput{Name: "", Vendor: "myspace", Price: usecase.MoneyInput{Amount: "0", Currency: "XXX"},
		Cycle: usecase.CycleInput{Unit: "fortnight"}, ManageURL: "javascript:alert(1)"})
	for _, field := range []string{"name", "vendor", "price.currency", "cycle.unit", "cycle.anchor", "manage_url"} {
		wantField(t, err, field)
	}
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateSubscriptionInput{Name: "X", Vendor: "other", Price: pln("-5"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}, EnvelopeID: "nope"})
	wantField(t, err, "price.amount")
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateSubscriptionInput{Name: "X", Vendor: "other", Price: pln("5"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}, EnvelopeID: "nope"})
	wantField(t, err, "envelope_id")

	// A private wallet of the owner does not exist for the member.
	f.addWallet("mine", "h1", f.owner, wallet.Private)
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateSubscriptionInput{Name: "X", Vendor: "other", Price: pln("5"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}, WalletID: "mine"})
	wantErr(t, err, domain.ErrNotFound)

	// A trial starts paying when it ends; with no paid term in the window,
	// none is expected.
	trial := f.subscribe(usecase.CreateSubscriptionInput{Name: "Claude Pro", Vendor: "anthropic", Price: usd("20.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-11-05")}, TrialEndsAt: day("2026-11-05"),
		ReminderOffsets: domain.Some([]int{1, 7, 7})})
	if trial.Subscription.Status != subscription.Trial || !slices.Equal(trial.Subscription.ReminderOffsets, []int{7, 1}) ||
		trial.NextDue != day("2026-11-05") {
		t.Errorf("trial = %+v", trial)
	}
	if got := f.charges(trial.Subscription.ID); got != "2026-11-05:expected:20.00 USD 2026-12-05:expected:20.00 USD 2027-01-05:expected:20.00 USD" {
		t.Errorf("trial charges = %s", got)
	}
}

func TestUpdateSubscriptionFollowsWithCharges(t *testing.T) {
	f := subscriptionFamily(t)
	update := usecase.NewUpdateSubscription(f.deps)
	id := f.vars["gpt"]
	// A charge already paid stays, whatever the plan becomes.
	paid := f.charge(id, "2026-11-30")
	f.recordAt("w1", "-80.00", "2026-11-30", "t-paid")
	_, err := usecase.NewConfirmChargeMatch(f.deps).Do(ctx, human(f.member), "h1", paid.ID, "t-paid")
	noErr(t, err)

	v, err := update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{
		Cycle: domain.Some(usecase.CycleInput{Unit: "month", Anchor: day("2026-01-15")}), Name: domain.Some("ChatGPT")})
	noErr(t, err)
	if v.NextDue != day("2026-11-15") || v.Subscription.Name != "ChatGPT" {
		t.Errorf("view = %+v", v)
	}
	if got := f.charges(id); got != "2026-11-15:expected:20.00 USD 2026-11-30:matched:20.00 USD 2026-12-15:expected:20.00 USD 2027-01-15:expected:20.00 USD" {
		t.Errorf("after the cycle = %s", got)
	}
	_, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{Status: domain.Some("paused")})
	noErr(t, err)
	if got := f.charges(id); got != "2026-11-30:matched:20.00 USD" {
		t.Errorf("paused = %s", got)
	}
	_, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{Status: domain.Some("active")})
	noErr(t, err)
	if got := f.charges(id); !strings.HasPrefix(got, "2026-11-15:expected") {
		t.Errorf("resumed = %s", got)
	}
	v, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{Status: domain.Some("cancelled")})
	noErr(t, err)
	s := v.Subscription
	if s.Status != subscription.Cancelled || s.EndsAt != day("2026-11-15") || s.CancelledAt.IsZero() || !v.NextDue.IsZero() {
		t.Errorf("cancelled = %+v next %s", s, v.NextDue)
	}
	if got := f.charges(id); got != "2026-11-30:matched:20.00 USD" {
		t.Errorf("cancelled = %s", got)
	}
	v, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{EndsAt: domain.Some(day("2026-12-16"))})
	noErr(t, err)
	if got := f.charges(id); got != "2026-11-15:expected:20.00 USD 2026-11-30:matched:20.00 USD 2026-12-15:expected:20.00 USD" {
		t.Errorf("paid until 16 December = %s", got)
	}
	_, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{Status: domain.Some("active"), EndsAt: domain.Some(day("2026-12-01"))})
	wantField(t, err, "ends_at")
	_, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{Status: domain.Some("trial")})
	wantField(t, err, "trial_ends_at")
	_, err = update.Do(ctx, human(f.member), "h1", id, usecase.UpdateSubscriptionInput{EnvelopeID: domain.Some("nope")})
	wantField(t, err, "envelope_id")
	if n := strings.Count(strings.Join(f.actions(), " "), "subscription.updated"); n != 5 {
		t.Errorf("updates audited = %d: %v", n, f.actions())
	}

	noErr(t, usecase.NewDeleteSubscription(f.deps).Do(ctx, human(f.member), "h1", id))
	_, err = usecase.NewGetSubscription(f.deps).Do(ctx, human(f.member), "h1", id)
	wantErr(t, err, domain.ErrNotFound)
	if got := f.charges(id); got != "" {
		t.Errorf("charges of a deleted subscription = %s", got)
	}
	if _, err := f.Transactions.ByID(ctx, "h1", "t-paid"); err != nil {
		t.Errorf("the paying transaction went: %v", err)
	}
}

// recordAt stores an expense of the wallet's currency with the id, as an
// import would.
func (f *family) recordAt(walletID domain.ID, amount, date string, id domain.ID) transaction.Transaction {
	f.t.Helper()
	w, err := f.Wallets.ByID(ctx, "h1", walletID)
	f.must(err)
	v, err := usecase.NewRecordTransaction(f.deps).Do(ctx, human(f.member), "h1", usecase.RecordTransactionInput{WalletID: string(walletID),
		Amount: usecase.MoneyInput{Amount: amount, Currency: w.Currency}, Date: day(date), PayeeName: "OPENAI *CHATGPT"})
	f.must(err)
	t := v.Transaction
	f.must(f.Transactions.Delete(ctx, "h1", t.ID))
	t.ID = id
	f.must(f.Transactions.Create(ctx, t))
	return t
}

func TestChangeSubscriptionPrice(t *testing.T) {
	f := subscriptionFamily(t)
	id := f.vars["gpt"]
	change := usecase.NewChangeSubscriptionPrice(f.deps)
	v, err := change.Do(ctx, human(f.member), "h1", id, usecase.ChangeSubscriptionPriceInput{EffectiveFrom: day("2026-12-01"), Price: usd("25.00")})
	noErr(t, err)
	if v.CurrentPrice.Amount != 2000 || len(v.Subscription.Prices) != 2 {
		t.Errorf("view = %+v", v)
	}
	if got := f.charges(id); got != "2026-10-31:expected:20.00 USD 2026-11-30:expected:20.00 USD 2026-12-31:expected:25.00 USD" {
		t.Errorf("charges = %s", got)
	}
	evs := f.events(event.SubscriptionPriceChanged)
	if len(evs) != 1 {
		t.Fatalf("events = %+v", f.Events.All())
	}
	p := evs[0].Payload
	if p["old_amount"] != "20.00" || p["old_currency"] != "USD" || p["new_amount"] != "25.00" || p["effective_from"] != "2026-12-01" ||
		p["subscription_name"] != "ChatGPT Plus" || p["vendor_name"] != "OpenAI" || p["wallet_id"] != "w1" ||
		evs[0].ResourceID != id || evs[0].DedupeKey != "price:"+p["price_id"] {
		t.Errorf("event = %+v", evs[0])
	}
	// The same day again replaces the point; the price may change currency.
	_, err = change.Do(ctx, human(f.member), "h1", id, usecase.ChangeSubscriptionPriceInput{EffectiveFrom: day("2026-12-01"), Price: pln("99.00")})
	noErr(t, err)
	prices, err := usecase.NewListSubscriptionPrices(f.deps).Do(ctx, human(f.viewer), "h1", id)
	noErr(t, err)
	if len(prices) != 2 || prices[1].Amount != 9900 || prices[1].Currency != "PLN" {
		t.Errorf("prices = %+v", prices)
	}
	if got := f.charges(id); !strings.HasSuffix(got, "2026-12-31:expected:99.00 PLN") {
		t.Errorf("charges = %s", got)
	}
	// From a past day: open charges since then take the price.
	_, err = change.Do(ctx, human(f.member), "h1", id, usecase.ChangeSubscriptionPriceInput{EffectiveFrom: day("2026-10-01"), Price: usd("21.00")})
	noErr(t, err)
	if got := f.charges(id); got != "2026-10-31:expected:21.00 USD 2026-11-30:expected:21.00 USD 2026-12-31:expected:99.00 PLN" {
		t.Errorf("charges = %s", got)
	}
	_, err = change.Do(ctx, human(f.member), "h1", id, usecase.ChangeSubscriptionPriceInput{Price: usd("0")})
	wantField(t, err, "price.amount")
	if n := len(f.events(event.SubscriptionPriceChanged)); n != 3 {
		t.Errorf("price events = %d", n)
	}
}

func TestSubscriptionJobs(t *testing.T) {
	f := subscriptionFamily(t)
	id := f.vars["gpt"]
	jobs := usecase.NewRunSubscriptionJobs(f.deps)
	r, err := jobs.Do(ctx)
	noErr(t, err)
	if r != (usecase.SubscriptionJobResult{}) {
		t.Errorf("first run on the day of creation = %+v", r)
	}
	// A week later the horizon reaches another term; three days before
	// the 31st the reminder goes out, once.
	f.Clock.Set(time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC))
	r, err = jobs.Do(ctx)
	noErr(t, err)
	if r.Created != 0 || r.Reminders != 1 {
		t.Errorf("run of 28 October = %+v", r)
	}
	evs := f.events(event.SubscriptionRenewalUpcoming)
	if len(evs) != 1 || evs[0].Payload["due_date"] != "2026-10-31" || evs[0].Payload["days_before"] != "3" ||
		evs[0].Payload["amount"] != "20.00" || evs[0].Payload["currency"] != "USD" {
		t.Errorf("reminders = %+v", evs)
	}
	r, _ = jobs.Do(ctx)
	if r != (usecase.SubscriptionJobResult{}) {
		t.Errorf("second run = %+v", r)
	}
	// Not paid by the end of the window: missed on 4 November, once.
	f.Clock.Set(time.Date(2026, 11, 3, 22, 30, 0, 0, time.UTC)) // 23:30 in Warsaw: still the 3rd
	r, _ = jobs.Do(ctx)
	if r.Missed != 0 {
		t.Errorf("missed too early: %+v", r)
	}
	f.Clock.Set(time.Date(2026, 11, 3, 23, 30, 0, 0, time.UTC)) // 00:30 of the 4th in Warsaw
	r, err = jobs.Do(ctx)
	noErr(t, err)
	if r.Missed != 1 {
		t.Errorf("run of 4 November = %+v", r)
	}
	if got := f.charges(id); !strings.HasPrefix(got, "2026-10-31:missed") {
		t.Errorf("charges = %s", got)
	}
	missed := f.events(event.SubscriptionChargeMissed)
	if len(missed) != 1 || missed[0].Payload["due_date"] != "2026-10-31" || missed[0].DedupeKey != "missed:"+missed[0].Payload["charge_id"] {
		t.Errorf("missed events = %+v", missed)
	}
	r, _ = jobs.Do(ctx)
	if r.Missed != 0 {
		t.Errorf("missed twice: %+v", r)
	}
	if !slices.Contains(f.actions(), "charge.missed") {
		t.Errorf("actions = %v", f.actions())
	}
	// A missed charge may still be paid.
	c := f.charge(id, "2026-10-31")
	f.recordAt("w1", "-80.00", "2026-11-02", "t-late")
	v, err := usecase.NewConfirmChargeMatch(f.deps).Do(ctx, human(f.member), "h1", c.ID, "t-late")
	noErr(t, err)
	if v.Charge.Status != subscription.Matched {
		t.Errorf("late match = %+v", v)
	}
	// Unlinked after the window, it is missed again.
	v, err = usecase.NewUnmatchCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID)
	noErr(t, err)
	if v.Charge.Status != subscription.Missed || v.Charge.TransactionID != "" {
		t.Errorf("unmatched late = %+v", v)
	}
}

func TestTrialEndingReminder(t *testing.T) {
	f := subscriptionFamily(t)
	trial := f.subscribe(usecase.CreateSubscriptionInput{Name: "Claude Pro", Vendor: "anthropic", Price: usd("20.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-25")}, TrialEndsAt: day("2026-10-25"),
		ReminderOffsets: domain.Some([]int{7, 1})})
	remind := usecase.NewSendSubscriptionReminders(f.deps)
	n, err := remind.Do(ctx)
	noErr(t, err)
	// 21 October: 4 days to the end of the trial; the 7-day reminder is due.
	evs := f.events(event.SubscriptionTrialEnding)
	if n != 1 || len(evs) != 1 || evs[0].Payload["trial_ends_at"] != "2026-10-25" || evs[0].Payload["days_before"] != "7" ||
		evs[0].Payload["amount"] != "20.00" || evs[0].ResourceID != trial.Subscription.ID {
		t.Errorf("trial events = %d %+v", n, evs)
	}
	f.Clock.Set(time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC))
	n, _ = remind.Do(ctx)
	if n != 1 || len(f.events(event.SubscriptionTrialEnding)) != 2 {
		t.Errorf("1-day reminder = %d", n)
	}
	n, _ = remind.Do(ctx)
	if n != 0 {
		t.Errorf("again = %d", n)
	}
}

func TestSuggestAndResolveCharges(t *testing.T) {
	f := subscriptionFamily(t)
	id := f.vars["gpt"]
	c := f.charge(id, "2026-10-31")
	// 80.40 PLN on the due day for 20.00 USD at 4.0000: 0.5% off.
	f.recordAt("w1", "-80.40", "2026-10-31", "t-good")
	f.recordAt("w1", "-79.00", "2026-11-02", "t-close")
	f.recordAt("w1", "-150.00", "2026-10-31", "t-far")
	got, err := usecase.NewSuggestTransactions(f.deps).Do(ctx, human(f.member), "h1", c.ID, domain.Optional[int]{})
	noErr(t, err)
	if len(got) != 2 || got[0].Transaction.Transaction.ID != "t-good" || got[0].Score != 39+30+20+10 || got[1].Transaction.Transaction.ID != "t-close" {
		t.Fatalf("suggestions = %+v", got)
	}
	back, err := usecase.NewSuggestCharges(f.deps).Do(ctx, human(f.member), "h1", "t-good")
	noErr(t, err)
	if len(back) != 1 || back[0].Charge.Charge.ID != c.ID || back[0].Score != 99 || back[0].Charge.SubscriptionName != "ChatGPT Plus" {
		t.Errorf("charges for the transaction = %+v", back)
	}

	confirm := usecase.NewConfirmChargeMatch(f.deps)
	v, err := confirm.Do(ctx, human(f.member), "h1", c.ID, "t-good")
	noErr(t, err)
	if v.Charge.Status != subscription.Matched || v.Charge.TransactionID != "t-good" {
		t.Errorf("matched = %+v", v)
	}
	_, err = confirm.Do(ctx, human(f.member), "h1", c.ID, "t-close")
	wantConflict(t, err, domain.ConflictChargeState)
	next := f.charge(id, "2026-11-30")
	_, err = confirm.Do(ctx, human(f.member), "h1", next.ID, "t-good")
	wantConflict(t, err, domain.ConflictTransactionLinked)
	_, err = confirm.Do(ctx, human(f.member), "h1", next.ID, "nope")
	wantField(t, err, "transaction_id")
	if back, _ := usecase.NewSuggestCharges(f.deps).Do(ctx, human(f.member), "h1", "t-good"); len(back) != 0 {
		t.Errorf("a linked transaction gets suggestions: %+v", back)
	}
	if got, _ := usecase.NewSuggestTransactions(f.deps).Do(ctx, human(f.member), "h1", next.ID, domain.Optional[int]{}); len(got) != 0 {
		t.Errorf("suggestions for November = %+v", got)
	}

	v, err = usecase.NewUnmatchCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID)
	noErr(t, err)
	if v.Charge.Status != subscription.Expected || v.Charge.TransactionID != "" {
		t.Errorf("unmatched = %+v", v)
	}
	_, err = usecase.NewUnmatchCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID)
	wantConflict(t, err, domain.ConflictChargeState)
	v, err = usecase.NewSkipCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID)
	noErr(t, err)
	if v.Charge.Status != subscription.Skipped {
		t.Errorf("skipped = %+v", v)
	}
	_, err = usecase.NewSkipCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID)
	wantConflict(t, err, domain.ConflictChargeState)
	for _, a := range []string{"charge.matched", "charge.unmatched", "charge.skipped"} {
		if !slices.Contains(f.actions(), a) {
			t.Errorf("no %s in %v", a, f.actions())
		}
	}

	page, err := usecase.NewListCharges(f.deps).Do(ctx, human(f.viewer), "h1", usecase.ListChargesInput{Statuses: []string{"expected"}})
	noErr(t, err)
	if len(page.Items) != 2 || page.Items[0].Charge.DueDate != day("2026-11-30") {
		t.Errorf("expected charges = %+v", page.Items)
	}
	_, err = usecase.NewListCharges(f.deps).Do(ctx, human(f.viewer), "h1", usecase.ListChargesInput{Statuses: []string{"lost"}})
	wantField(t, err, "status")
	_, err = usecase.NewListCharges(f.deps).Do(ctx, human(f.viewer), "h1", usecase.ListChargesInput{SubscriptionID: "nope"})
	wantErr(t, err, domain.ErrNotFound)
}

func TestPostCharge(t *testing.T) {
	f := subscriptionFamily(t)
	id := f.vars["gpt"]
	post := usecase.NewPostCharge(f.deps)
	// 20.00 USD at the rate of 30 November, 4.1000, from the PLN wallet.
	c := f.charge(id, "2026-11-30")
	out, err := post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{})
	noErr(t, err)
	tx := out.Transaction.Transaction
	if out.Charge.Charge.Status != subscription.Posted || out.Charge.Charge.TransactionID != tx.ID || tx.Amount != -8200 ||
		tx.Currency != "PLN" || tx.Date != day("2026-11-30") || tx.Source != transaction.SourceSubscription ||
		tx.EnvelopeID != f.vars["food"] || out.Transaction.PayeeName != "OpenAI" || tx.Note != "Subskrypcja: ChatGPT Plus" ||
		tx.Kind != transaction.Expense || tx.BaseAmount != -8200 {
		t.Errorf("posted = %+v", out)
	}
	_, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{})
	wantConflict(t, err, domain.ConflictChargeState)
	// In the USD wallet the charge's own amount; given explicitly in PLN.
	c = f.charge(id, "2026-10-31")
	out, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{WalletID: "w2", Date: day("2026-11-01"), PayeeName: "OpenAI LLC"})
	noErr(t, err)
	if out.Transaction.Transaction.Amount != -2000 || out.Transaction.Transaction.Currency != "USD" || out.Transaction.PayeeName != "OpenAI LLC" {
		t.Errorf("USD post = %+v", out.Transaction)
	}
	c = f.charge(id, "2026-12-31") // no USD rate in late December
	_, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{})
	wantField(t, err, "amount")
	_, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{Amount: &usecase.MoneyInput{Amount: "-1", Currency: "PLN"}})
	wantField(t, err, "amount.amount")
	out, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{Amount: &usecase.MoneyInput{Amount: "83.17", Currency: "PLN"}})
	noErr(t, err)
	if out.Transaction.Transaction.Amount != -8317 {
		t.Errorf("explicit = %+v", out.Transaction)
	}
	// Without a wallet one must be given.
	nw := f.subscribe(usecase.CreateSubscriptionInput{Name: "Netflix", Vendor: "netflix", Price: pln("60.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-25")}})
	c = f.charge(nw.Subscription.ID, "2026-10-25")
	_, err = post.Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{})
	wantField(t, err, "wallet_id")
	if !slices.Contains(f.actions(), "charge.posted") {
		t.Errorf("actions = %v", f.actions())
	}
}

func TestDeletingAPayingTransactionReopensTheCharge(t *testing.T) {
	f := subscriptionFamily(t)
	c := f.charge(f.vars["gpt"], "2026-11-30")
	out, err := usecase.NewPostCharge(f.deps).Do(ctx, human(f.member), "h1", c.ID, usecase.PostChargeInput{})
	noErr(t, err)
	noErr(t, usecase.NewDeleteTransaction(f.deps).Do(ctx, human(f.member), "h1", out.Transaction.Transaction.ID))
	if got := f.charge(f.vars["gpt"], "2026-11-30"); got.Status != subscription.Expected || got.TransactionID != "" {
		t.Errorf("charge = %+v", got)
	}
	entries := f.Audit.All()
	last := entries[len(entries)-2]
	if last.Action != "charge.unmatched" || last.Changes["via"].To != "transaction_deleted" {
		t.Errorf("audit = %+v", last)
	}
}

func TestEnvelopeOfASubscriptionCannotBeDeleted(t *testing.T) {
	f := subscriptionFamily(t)
	err := usecase.NewDeleteEnvelope(f.deps).Do(ctx, human(f.member), "h1", f.vars["food"])
	wantConflict(t, err, domain.ConflictEnvelopeInUse)
	noErr(t, usecase.NewDeleteEnvelope(f.deps).Do(ctx, human(f.member), "h1", f.vars["fun"]))
}

func TestAutoMatchTransactions(t *testing.T) {
	f := subscriptionFamily(t)
	auto := usecase.NewAutoMatchTransactions(f.deps)
	c := f.charge(f.vars["gpt"], "2026-10-31")
	f.recordAt("w1", "-80.40", "2026-11-01", "t-import")
	f.recordAt("w1", "-30.00", "2026-10-31", "t-other")
	got, err := auto.Do(ctx, human(f.member), "h1", []domain.ID{"t-other", "t-import", "nope"})
	noErr(t, err)
	if len(got) != 1 || got[0].ChargeID != c.ID || got[0].TransactionID != "t-import" || got[0].Score < 80 {
		t.Fatalf("auto = %+v", got)
	}
	if got := f.charge(f.vars["gpt"], "2026-10-31"); got.Status != subscription.Matched {
		t.Errorf("charge = %+v", got)
	}
	entries := f.Audit.All()
	if last := entries[len(entries)-1]; last.Action != "charge.matched" || last.Changes["via"].To != "auto" {
		t.Errorf("audit = %+v", last)
	}
	again, _ := auto.Do(ctx, human(f.member), "h1", []domain.ID{"t-import"})
	if len(again) != 0 {
		t.Errorf("again = %+v", again)
	}

	// Two transactions fit November equally well: no automatic choice.
	f.recordAt("w1", "-82.00", "2026-11-30", "t-a")
	f.recordAt("w1", "-82.00", "2026-11-30", "t-b")
	got, _ = auto.Do(ctx, human(f.member), "h1", []domain.ID{"t-a"})
	if len(got) != 0 {
		t.Errorf("ambiguous = %+v", got)
	}
	// Two subscriptions fit one transaction: no automatic choice either.
	twin := f.subscribe(usecase.CreateSubscriptionInput{Name: "ChatGPT Team", Vendor: "openai", Price: usd("20.00"), WalletID: "w1",
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-31")}})
	_ = twin
	f.recordAt("w1", "-84.00", "2026-12-31", "t-dec")
	f.Rates.Set("USD", "2026-12-31", "4.2000")
	got, _ = auto.Do(ctx, human(f.member), "h1", []domain.ID{"t-dec"})
	if len(got) != 0 {
		t.Errorf("two subscriptions = %+v", got)
	}
	// Too far from any term, without a rate besides.
	f.Rates.SetDown(true)
	f.recordAt("w1", "-80.00", "2027-01-15", "t-jan")
	if got, _ := auto.Do(ctx, human(f.member), "h1", []domain.ID{"t-jan"}); len(got) != 0 {
		t.Errorf("below threshold = %+v", got)
	}
}

func TestSubscriptionSummary(t *testing.T) {
	f := subscriptionFamily(t)
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Netflix", Vendor: "netflix", Price: pln("60.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-25")}})
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Domena", Vendor: "other", Price: pln("120.00"), EnvelopeID: string(f.vars["fun"]),
		Cycle: usecase.CycleInput{Unit: "year", Anchor: day("2027-03-01")}})
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Claude Pro", Vendor: "anthropic", Price: usd("20.00"),
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-30")}, TrialEndsAt: day("2026-10-30")})
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Spotify", Vendor: "spotify", Price: pln("23.99"), Status: "paused",
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}})
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Proton", Vendor: "other", Price: usecase.MoneyInput{Amount: "10.00", Currency: "CHF"},
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}})
	_, err := usecase.NewChangeSubscriptionPrice(f.deps).Do(ctx, human(f.member), "h1", f.vars["gpt"],
		usecase.ChangeSubscriptionPriceInput{EffectiveFrom: day("2026-10-01"), Price: usd("22.00")})
	noErr(t, err)

	sum, err := usecase.NewGetSubscriptionSummary(f.deps).Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	// 22 USD × 4 = 88 PLN, 60 PLN, 10 PLN (120 a year), 80 PLN (trial).
	if sum.Currency != "PLN" || sum.Total.Monthly != 8800+6000+1000+8000 || sum.Total.Yearly != (8800+6000+8000)*12+12000 {
		t.Errorf("total = %+v", sum.Total)
	}
	if !sum.Incomplete || !slices.Equal(sum.MissingCurrencies, []string{"CHF"}) {
		t.Errorf("incomplete = %v %v", sum.Incomplete, sum.MissingCurrencies)
	}
	if sum.Counts[subscription.Active] != 4 || sum.Counts[subscription.Trial] != 1 || sum.Counts[subscription.Paused] != 1 || sum.Counts[subscription.Cancelled] != 0 {
		t.Errorf("counts = %v", sum.Counts)
	}
	if len(sum.ByVendor) != 4 || sum.ByVendor[0].Key != "openai" || sum.ByVendor[0].Cost.Monthly != 8800 || sum.ByVendor[3].Key != "other" {
		t.Errorf("by vendor = %+v", sum.ByVendor)
	}
	byEnvelope := map[string]int64{}
	for _, g := range sum.ByEnvelope {
		byEnvelope[g.Key] = g.Cost.Monthly
	}
	if byEnvelope[string(f.vars["food"])] != 8800 || byEnvelope[string(f.vars["fun"])] != 1000 || byEnvelope[""] != 14000 {
		t.Errorf("by envelope = %+v", sum.ByEnvelope)
	}
	if len(sum.TrialsEnding) != 1 || sum.TrialsEnding[0].EndsAt != day("2026-10-30") || sum.TrialsEnding[0].Price.Amount != 2000 {
		t.Errorf("trials = %+v", sum.TrialsEnding)
	}
	if len(sum.PriceChanges) != 1 || sum.PriceChanges[0].Price.Amount != 2200 || sum.PriceChanges[0].Previous.Amount != 2000 {
		t.Errorf("price changes = %+v", sum.PriceChanges)
	}
	var upcoming []string
	for _, u := range sum.Upcoming {
		upcoming = append(upcoming, u.Charge.Charge.DueDate.String()+" "+u.Charge.SubscriptionName)
	}
	if strings.Join(upcoming, ", ") != "2026-10-25 Netflix, 2026-10-30 Claude Pro, 2026-10-31 ChatGPT Plus, 2026-11-01 Proton" {
		t.Errorf("upcoming = %v", upcoming)
	}

	renewals, err := usecase.NewListUpcomingRenewals(f.deps).Do(ctx, asBot(f.addBot("b1", "h1", f.member, "subscriptions:read")), "h1", domain.Some(5))
	noErr(t, err)
	if len(renewals) != 1 || renewals[0].Charge.SubscriptionName != "Netflix" || !renewals[0].BaseKnown || renewals[0].BaseAmount != 6000 {
		t.Errorf("renewals = %+v", renewals)
	}
	renewals, _ = usecase.NewListUpcomingRenewals(f.deps).Do(ctx, human(f.member), "h1", domain.Optional[int]{})
	if len(renewals) != 1 {
		t.Errorf("renewals of 7 days = %+v", renewals)
	}
	renewals, _ = usecase.NewListUpcomingRenewals(f.deps).Do(ctx, human(f.member), "h1", domain.Some(9))
	if len(renewals) != 2 || renewals[1].Charge.SubscriptionName != "Claude Pro" || renewals[1].BaseAmount != 8000 {
		t.Errorf("renewals of 9 days = %+v", renewals)
	}
	_, err = usecase.NewListUpcomingRenewals(f.deps).Do(ctx, human(f.member), "h1", domain.Some(366))
	wantField(t, err, "days")
}

func TestListSubscriptionsFilters(t *testing.T) {
	f := subscriptionFamily(t)
	f.subscribe(usecase.CreateSubscriptionInput{Name: "Netflix", Vendor: "netflix", Price: pln("60.00"), Status: "paused",
		Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-25")}})
	list := usecase.NewListSubscriptions(f.deps)
	all, err := list.Do(ctx, human(f.viewer), "h1", usecase.ListSubscriptionsInput{})
	noErr(t, err)
	if len(all) != 2 || all[0].Subscription.Name != "ChatGPT Plus" {
		t.Errorf("all = %+v", all)
	}
	for in, want := range map[usecase.ListSubscriptionsInput]int{{Status: "paused"}: 1, {Vendor: "openai"}: 1, {WalletID: "w1"}: 1,
		{Vendor: "apple"}: 0} {
		got, err := list.Do(ctx, human(f.viewer), "h1", in)
		noErr(t, err)
		if len(got) != want {
			t.Errorf("%+v = %d, want %d", in, len(got), want)
		}
	}
	_, err = list.Do(ctx, human(f.viewer), "h1", usecase.ListSubscriptionsInput{Status: "gone"})
	wantField(t, err, "status")
	if v := usecase.NewListVendors().Do(ctx, human(f.viewer)); len(v) != len(subscription.Vendors) {
		t.Errorf("vendors = %v", v)
	}
}
