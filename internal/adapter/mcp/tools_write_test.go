package mcp

import (
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
)

var assistantScopes = []bot.Scope{bot.ScopeTransactionsRead, bot.ScopeTransactionsWrite, bot.ScopeSubscriptionsRead,
	bot.ScopeSubscriptionsWrite, bot.ScopeBudgetRead, bot.ScopeBudgetWrite, bot.ScopeWalletsRead}

func TestRecordExpense(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, assistantScopes))
	// A subscription whose charge the expense will pay.
	ok(t, s, "create_subscription", map[string]any{"name": "Netflix", "vendor": "netflix", "amount": "43.00", "anchor": "2026-10-21",
		"wallet": "karta", "envelope": "rozrywka", "payee_pattern": "NETFLIX"})

	a := ok(t, s, "record_expense", map[string]any{"wallet": "Karta", "amount": "12,50", "payee": "Biedronka", "envelope": "jedzenie",
		"note": "zakupy", "tags": []string{"dom"}})
	tx := a.get("transaction").(map[string]any)
	if tx["date"] != "2026-10-21" || a.str("transaction", "amount", "amount") != "-12.50" || tx["source"] != "bot" ||
		tx["envelope_id"] != "e-food" || tx["payee_name"] != "Biedronka" || a.str("transaction", "created_by", "kind") != "bot" ||
		a.str("transaction", "created_by", "id") != "b1" || a.list("transaction", "tags")[0] != "dom" {
		t.Errorf("expense = %s", a.raw)
	}
	if got := a.list("charge_suggestions"); got == nil || len(got) != 0 {
		t.Errorf("suggestions for groceries = %s", a.raw)
	}
	stored, err := w.Transactions.ByID(ctx, "h1", domain.ID(tx["id"].(string)))
	w.must(err)
	if stored.Source != transaction.SourceBot || stored.CreatedBy.Kind != domain.ActorBot {
		t.Errorf("stored = %+v", stored)
	}
	var audited bool
	for _, e := range w.Audit.All() {
		if e.Action == "transaction.created" && e.ResourceID == stored.ID && e.Actor.Kind == domain.ActorBot && e.Actor.ID == "b1" {
			audited = true
		}
	}
	if !audited {
		t.Errorf("no audit entry by the bot: %+v", w.Audit.All())
	}

	// The Netflix payment gets the charge as a suggestion, and confirms it.
	a = ok(t, s, "record_expense", map[string]any{"wallet": "w-card", "amount": "43.00", "payee": "NETFLIX.COM", "date": "2026-10-21"})
	if len(a.list("charge_suggestions")) != 1 || a.str("charge_suggestions", 0, "charge", "subscription_name") != "Netflix" {
		t.Fatalf("suggestions = %s", a.raw)
	}
	charge := a.str("charge_suggestions", 0, "charge", "id")
	m := ok(t, s, "confirm_charge_match", map[string]any{"charge_id": charge, "transaction_id": a.str("transaction", "id")})
	if m.str("status") != "matched" || m.str("transaction_id") != a.str("transaction", "id") {
		t.Errorf("matched = %s", m.raw)
	}
	refused(t, s, "confirm_charge_match", map[string]any{"charge_id": charge, "transaction_id": a.str("transaction", "id")}, codeConflict)

	// Without the read scopes the expense is recorded without suggestions.
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeTransactionsWrite}))
	a = ok(t, s, "record_expense", map[string]any{"wallet": "Gotówka", "amount": "5"})
	if a.get("charge_suggestions") != nil || a.str("transaction", "amount", "amount") != "-5.00" {
		t.Errorf("without suggestions = %s", a.raw)
	}
}

func TestRecordExpenseRefusals(t *testing.T) {
	w := newWorld(t)
	before := len(w.Audit.All())
	// The viewer's bot holds every scope, but its creator only reads.
	viewer := w.connect(w.addBot("bv", w.viewer, allScopes))
	a := refused(t, viewer, "record_expense", map[string]any{"wallet": "Karta", "amount": "10.00"}, codeForbidden)
	if !strings.Contains(a.message(), "transactions:write") {
		t.Errorf("message = %q", a.message())
	}
	refused(t, viewer, "assign_to_envelope", map[string]any{"envelope": "Jedzenie", "amount": "10.00"}, codeForbidden)
	refused(t, viewer, "create_subscription", map[string]any{"name": "X", "amount": "1.00"}, codeForbidden)
	ok(t, viewer, "list_wallets", nil) // reading still works

	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeTransactionsWrite}, "w-cash"))
	// Off its wallet list, by id or name, a wallet does not exist.
	refused(t, s, "record_expense", map[string]any{"wallet": "w-card", "amount": "10.00"}, codeNotFound)
	refused(t, s, "record_expense", map[string]any{"wallet": "Karta", "amount": "10.00"}, codeNotFound)
	refused(t, s, "record_expense", map[string]any{"wallet": "Oszczędności", "amount": "10.00"}, codeNotFound)
	// Bad amounts and dates.
	for _, args := range []map[string]any{
		{"wallet": "Gotówka", "amount": "-10.00"},
		{"wallet": "Gotówka", "amount": "0"},
		{"wallet": "Gotówka", "amount": "10.001"},
		{"wallet": "Gotówka", "amount": "dziesięć"},
		{"wallet": "Gotówka", "amount": "10.00", "currency": "EUR"},
		{"wallet": "Gotówka", "amount": "10.00", "date": "jutro"},
		{"wallet": "Gotówka", "amount": 10},
		{"wallet": "Gotówka"},
		{"wallet": "", "amount": "10.00"},
		{"wallet": "Gotówka", "amount": "1.00", "unknown": true},
	} {
		refused(t, s, "record_expense", args, codeInvalid)
	}
	if n := len(w.Audit.All()); n != before {
		t.Errorf("refused calls left %d audit entries", n-before)
	}
	all := w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeTransactionsWrite}))
	a = refused(t, all, "record_expense", map[string]any{"wallet": "a", "amount": "1.00"}, codeInvalid)
	if !strings.Contains(a.message(), `"Gotówka"`) || !strings.Contains(a.message(), `"Karta"`) {
		t.Errorf("ambiguity = %q", a.message())
	}
}

func TestIncomeAndTransfer(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, assistantScopes))
	a := ok(t, s, "record_income", map[string]any{"wallet": "Karta", "amount": "5200.00", "payee": "Pracodawca"})
	if a.str("amount", "amount") != "5200.00" || a.str("kind") != "income" || a.str("source") != "bot" || a.get("envelope_id") != nil {
		t.Errorf("income = %s", a.raw)
	}
	a = ok(t, s, "transfer", map[string]any{"from_wallet": "Karta", "to_wallet": "Gotówka", "amount": "200"})
	if a.str("from", "amount", "amount") != "-200.00" || a.str("to", "amount", "amount") != "200.00" || a.str("to", "kind") != "transfer" ||
		a.str("from", "transfer_wallet_id") != "w-cash" {
		t.Errorf("transfer = %s", a.raw)
	}
	refused(t, s, "transfer", map[string]any{"from_wallet": "Karta", "to_wallet": "Oszczędności", "amount": "1"}, codeNotFound)
	refused(t, s, "transfer", map[string]any{"from_wallet": "Karta", "to_wallet": "Karta", "amount": "1"}, codeInvalid)
}

func TestEnvelopeWrites(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, assistantScopes))
	ok(t, s, "record_income", map[string]any{"wallet": "Karta", "amount": "1000.00"})
	a := ok(t, s, "assign_to_envelope", map[string]any{"envelope": "jedzenie", "amount": "800"})
	if a.str("month") != "2026-10" || a.str("envelope", "assigned", "amount") != "800.00" || a.str("ready_to_assign", "amount") != "200.00" {
		t.Errorf("assigned = %s", a.raw)
	}
	a = ok(t, s, "move_between_envelopes", map[string]any{"from_envelope": "Jedzenie", "to_envelope": "Rozrywka", "amount": "50.00"})
	if a.str("from", "assigned", "amount") != "750.00" || a.str("to", "assigned", "amount") != "50.00" {
		t.Errorf("moved = %s", a.raw)
	}
	a = ok(t, s, "assign_to_envelope", map[string]any{"envelope": "e-fun", "amount": "10", "month": "2026-11"})
	if a.str("month") != "2026-11" {
		t.Errorf("november = %s", a.raw)
	}
	refused(t, s, "assign_to_envelope", map[string]any{"envelope": "Czynsz", "amount": "1"}, codeNotFound)
	refused(t, s, "move_between_envelopes", map[string]any{"from_envelope": "Jedzenie", "to_envelope": "Jedzenie", "amount": "1"}, codeInvalid)

	// budget:read alone does not write.
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeBudgetRead}))
	if m := refused(t, s, "assign_to_envelope", map[string]any{"envelope": "Jedzenie", "amount": "1"}, codeForbidden).message(); !strings.Contains(m, "budget:write") {
		t.Errorf("message = %q", m)
	}
}

func TestSubscriptionTools(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, assistantScopes))
	a := ok(t, s, "create_subscription", map[string]any{"name": "ChatGPT Plus", "vendor": "openai", "amount": "20.00", "currency": "usd",
		"anchor": "2026-10-25", "wallet": "Karta", "reminder_offsets": []int{7, 1}})
	if a.str("price", "amount") != "20.00" || a.str("price", "currency") != "USD" || a.str("cycle", "unit") != "month" ||
		a.str("next_due") != "2026-10-25" || a.str("wallet_id") != "w-card" || a.str("status") != "active" {
		t.Errorf("created = %s", a.raw)
	}
	ok(t, s, "create_subscription", map[string]any{"name": "Spotify", "vendor": "spotify", "amount": "23.99", "anchor": "2026-11-02"})
	refused(t, s, "create_subscription", map[string]any{"name": "spotify", "amount": "1.00"}, codeConflict)
	refused(t, s, "create_subscription", map[string]any{"name": "Bad", "amount": "1.00", "wallet": "Oszczędności"}, codeNotFound)

	a = ok(t, s, "list_subscriptions", nil)
	if len(a.list("subscriptions")) != 2 || a.str("subscriptions", 0, "name") != "ChatGPT Plus" {
		t.Errorf("list = %s", a.raw)
	}
	a = ok(t, s, "list_subscriptions", map[string]any{"wallet": "Karta"})
	if len(a.list("subscriptions")) != 1 {
		t.Errorf("by wallet = %s", a.raw)
	}

	a = ok(t, s, "get_subscription", map[string]any{"subscription": "chatgpt"})
	if a.str("subscription", "name") != "ChatGPT Plus" || len(a.list("prices")) != 1 || a.str("open_charges", 0, "due_date") != "2026-10-25" {
		t.Errorf("get = %s", a.raw)
	}
	refused(t, s, "get_subscription", map[string]any{"subscription": "Disney"}, codeNotFound)

	a = ok(t, s, "upcoming_renewals", nil)
	if len(a.list("renewals")) != 2 || a.str("renewals", 0, "charge", "subscription_name") != "ChatGPT Plus" ||
		a.get("renewals", 0, "base_amount") != nil || a.str("renewals", 1, "base_amount", "amount") != "23.99" || a.str("currency") != "PLN" {
		t.Errorf("upcoming = %s", a.raw)
	}
	a = ok(t, s, "upcoming_renewals", map[string]any{"days": 7})
	if len(a.list("renewals")) != 1 {
		t.Errorf("upcoming in a week = %s", a.raw)
	}
	refused(t, s, "upcoming_renewals", map[string]any{"days": 1000}, codeInvalid)

	a = ok(t, s, "subscription_costs", nil)
	// No USD rate is known: only Spotify counts, and the summary says so.
	if a.str("monthly", "amount") != "23.99" || a.get("incomplete") != true || a.list("missing_currencies")[0] != "USD" ||
		a.get("counts", "active") != float64(2) {
		t.Errorf("costs = %s", a.raw)
	}

	a = ok(t, s, "update_subscription", map[string]any{"subscription": "Spotify", "status": "paused", "envelope": "Rozrywka", "every": 2})
	if a.str("status") != "paused" || a.str("envelope_id") != "e-fun" || a.get("cycle", "every") != float64(2) ||
		a.str("cycle", "anchor") != "2026-11-02" {
		t.Errorf("updated = %s", a.raw)
	}
	a = ok(t, s, "update_subscription", map[string]any{"subscription": "Spotify", "envelope": "", "status": "active"})
	if a.get("envelope_id") != nil {
		t.Errorf("cleared envelope = %s", a.raw)
	}
	a = ok(t, s, "change_subscription_price", map[string]any{"subscription": "ChatGPT Plus", "amount": "25.00", "effective_from": "2026-11-01"})
	if a.str("price", "amount") != "20.00" || a.str("price", "currency") != "USD" {
		t.Errorf("new price = %s", a.raw)
	}

	a = ok(t, s, "list_charges", map[string]any{"subscription": "ChatGPT Plus"})
	if len(a.list("charges")) != 3 || a.str("charges", 0, "amount", "amount") != "20.00" || a.str("charges", 1, "amount", "amount") != "25.00" {
		t.Fatalf("charges = %s", a.raw)
	}
	first := a.str("charges", 0, "id")
	second := a.str("charges", 1, "id")
	a = ok(t, s, "skip_charge", map[string]any{"charge_id": first})
	if a.str("status") != "skipped" {
		t.Errorf("skipped = %s", a.raw)
	}
	refused(t, s, "skip_charge", map[string]any{"charge_id": first}, codeConflict)
	// Posting the next one from the card: 101.25 PLN paid.
	a = ok(t, s, "post_charge", map[string]any{"charge_id": second, "amount": "101.25", "date": "2026-11-25"})
	if a.str("charge", "status") != "posted" || a.str("transaction", "amount", "amount") != "-101.25" ||
		a.str("transaction", "source") != "subscription" || a.str("transaction", "wallet_id") != "w-card" {
		t.Errorf("posted = %s", a.raw)
	}
	a = ok(t, s, "list_charges", map[string]any{"status": []string{"posted", "skipped"}})
	if len(a.list("charges")) != 2 {
		t.Errorf("by status = %s", a.raw)
	}
	refused(t, s, "skip_charge", map[string]any{"charge_id": "nope"}, codeNotFound)

	// subscriptions:read only reads.
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeSubscriptionsRead}))
	ok(t, s, "list_subscriptions", nil)
	if m := refused(t, s, "update_subscription", map[string]any{"subscription": "Spotify", "note": "x"}, codeForbidden).message(); !strings.Contains(m, "subscriptions:write") {
		t.Errorf("message = %q", m)
	}
	// A subscription paid from a wallet off the bot's list is hidden.
	s = w.connect(w.addBot("b3", w.member, []bot.Scope{bot.ScopeSubscriptionsRead}, "w-cash"))
	a = ok(t, s, "list_subscriptions", nil)
	if len(a.list("subscriptions")) != 1 || a.str("subscriptions", 0, "name") != "Spotify" {
		t.Errorf("limited list = %s", a.raw)
	}
	refused(t, s, "get_subscription", map[string]any{"subscription": "ChatGPT Plus"}, codeNotFound)
}
