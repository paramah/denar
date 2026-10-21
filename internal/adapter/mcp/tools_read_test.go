package mcp

import (
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/imports"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// spend records an expense as the owner, through the use case.
func (w *world) spend(walletID, amount, date, payeeName string, envelopeID domain.ID) domain.ID {
	w.t.Helper()
	d, err := domain.ParseDate(date)
	w.must(err)
	v, err := usecase.NewRecordTransaction(w.deps).Do(ctx, usecase.Principal{User: w.owner}, "h1", usecase.RecordTransactionInput{
		WalletID: walletID, Date: d, Amount: usecase.MoneyInput{Amount: amount, Currency: "PLN"}, PayeeName: payeeName,
		EnvelopeID: string(envelopeID)})
	w.must(err)
	return v.Transaction.ID
}

func TestWhoami(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeTransactionsWrite, bot.ScopeSubscriptionsRead}, "w-cash"))
	a := ok(t, s, "whoami", nil)
	if a.str("bot", "id") != "b1" || a.str("household", "base_currency") != "PLN" || a.str("household", "timezone") != "Europe/Warsaw" ||
		a.str("today") != "2026-10-21" || a.str("current_month") != "2026-10" {
		t.Errorf("whoami = %s", a.raw)
	}
	if got := a.list("bot", "scopes"); len(got) != 2 || got[0] != "transactions:write" {
		t.Errorf("scopes = %v", got)
	}
	if got := a.list("wallets"); len(got) != 1 || a.str("wallets", 0, "name") != "Gotówka" || a.list("bot", "wallet_ids")[0] != "w-cash" {
		t.Errorf("wallets = %s", a.raw)
	}
	if a.get("bot", "expires_at") != nil {
		t.Errorf("expires_at = %v", a.get("bot", "expires_at"))
	}
}

func TestWalletsAndBalances(t *testing.T) {
	w := newWorld(t)
	w.spend("w-card", "-42.50", "2026-10-20", "Biedronka", "e-food")
	w.spend("w-priv", "-1000.00", "2026-10-20", "", "")
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeWalletsRead}))

	a := ok(t, s, "list_wallets", nil)
	if names := a.list("wallets"); len(names) != 2 || a.str("wallets", 0, "name") != "Gotówka" || a.str("wallets", 1, "name") != "Karta" {
		t.Errorf("wallets = %s", a.raw)
	}
	a = ok(t, s, "get_balances", nil)
	if a.str("wallets", 1, "balance", "amount") != "-42.50" || a.get("wallets", 1, "balance", "minor") != float64(-4250) ||
		a.str("total_base", "amount") != "-42.50" || a.str("total_base", "currency") != "PLN" || a.get("base_pending") != false {
		t.Errorf("balances = %s", a.raw)
	}

	// Without the scope.
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeTransactionsRead}))
	m := refused(t, s, "get_balances", nil, codeForbidden).message()
	if !strings.Contains(m, "wallets:read") {
		t.Errorf("message = %q", m)
	}
}

func TestSearchTransactions(t *testing.T) {
	w := newWorld(t)
	w.spend("w-card", "-42.50", "2026-10-20", "Biedronka", "e-food")
	w.spend("w-card", "-15.00", "2026-10-19", "Kino", "e-fun")
	w.spend("w-cash", "-8.00", "2026-10-18", "Biedronka", "e-food")
	w.spend("w-priv", "-1000.00", "2026-10-20", "Biedronka", "")
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeTransactionsRead}))

	a := ok(t, s, "search_transactions", map[string]any{"payee": "biedronka"})
	if got := a.list("transactions"); len(got) != 2 || a.str("transactions", 0, "amount", "amount") != "-42.50" ||
		a.str("transactions", 0, "payee_name") != "Biedronka" || a.str("transactions", 0, "envelope_id") != "e-food" ||
		a.str("transactions", 0, "base_amount", "amount") != "-42.50" || a.get("next_cursor") != nil {
		t.Errorf("by payee = %s", a.raw)
	}
	a = ok(t, s, "search_transactions", map[string]any{"wallet": "Karta", "envelope": "rozrywka"})
	if got := a.list("transactions"); len(got) != 1 || a.str("transactions", 0, "payee_name") != "Kino" {
		t.Errorf("by wallet and envelope = %s", a.raw)
	}
	a = ok(t, s, "search_transactions", map[string]any{"limit": 1})
	cursor := a.str("next_cursor")
	if len(a.list("transactions")) != 1 || cursor == "" {
		t.Fatalf("first page = %s", a.raw)
	}
	a = ok(t, s, "search_transactions", map[string]any{"limit": 1, "cursor": cursor})
	if a.str("transactions", 0, "payee_name") != "Kino" {
		t.Errorf("second page = %s", a.raw)
	}
	refused(t, s, "search_transactions", map[string]any{"wallet": "Oszczędności"}, codeNotFound)
	refused(t, s, "search_transactions", map[string]any{"from": "21.10.2026"}, codeInvalid)
	refused(t, s, "search_transactions", map[string]any{"limit": "ten"}, codeInvalid)

	// A bot limited to the cash wallet sees nothing of the card.
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeTransactionsRead}, "w-cash"))
	a = ok(t, s, "search_transactions", nil)
	if len(a.list("transactions")) != 1 {
		t.Errorf("limited bot sees %s", a.raw)
	}
	refused(t, s, "search_transactions", map[string]any{"wallet": "w-card"}, codeNotFound)
	refused(t, s, "search_transactions", map[string]any{"wallet": "Karta"}, codeNotFound)

	a = ok(t, s, "list_payees", nil)
	if len(a.list("payees")) != 2 || a.str("payees", 0, "name") != "Biedronka" {
		t.Errorf("payees = %s", a.raw)
	}
}

func TestBudgetTools(t *testing.T) {
	w := newWorld(t)
	w.spend("w-card", "-42.50", "2026-10-20", "Biedronka", "e-food")
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeBudgetRead}))
	a := ok(t, s, "list_envelopes", nil)
	if len(a.list("envelopes")) != 2 || a.str("envelopes", 0, "name") != "Jedzenie" || a.str("envelopes", 0, "group_name") != "Codzienne" ||
		a.get("envelopes", 0, "goal") != nil || a.str("currency") != "PLN" {
		t.Errorf("envelopes = %s", a.raw)
	}
	a = ok(t, s, "budget_overview", nil)
	if a.str("month") != "2026-10" || a.str("groups", 0, "envelopes", 0, "activity", "amount") != "-42.50" ||
		a.get("groups", 0, "envelopes", 0, "overspent") != true || a.str("ready_to_assign", "amount") != "0.00" {
		t.Errorf("overview = %s", a.raw)
	}
	a = ok(t, s, "budget_overview", map[string]any{"month": "2026-09"})
	if a.str("month") != "2026-09" {
		t.Errorf("september = %s", a.raw)
	}
	refused(t, s, "budget_overview", map[string]any{"month": "październik"}, codeInvalid)
}

func TestSpendingBy(t *testing.T) {
	w := newWorld(t)
	w.spend("w-card", "-42.50", "2026-10-20", "Biedronka", "e-food")
	w.spend("w-card", "-15.00", "2026-10-19", "Kino", "e-fun")
	w.spend("w-cash", "-8.00", "2026-10-18", "Biedronka", "e-food")
	w.spend("w-priv", "-1000.00", "2026-10-20", "Biedronka", "")
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeReportsRead}))
	a := ok(t, s, "spending_by", map[string]any{"group_by": "envelope"})
	if a.str("from") != "2026-10-01" || a.str("to") != "2026-10-21" || a.str("total", "amount") != "65.50" ||
		a.str("groups", 0, "name") != "Jedzenie" || a.str("groups", 0, "total", "amount") != "50.50" || a.get("groups", 0, "count") != float64(2) {
		t.Errorf("by envelope = %s", a.raw)
	}
	a = ok(t, s, "spending_by", map[string]any{"group_by": "payee", "wallet": "gotówka", "from": "2026-10-01", "to": "2026-10-31"})
	if len(a.list("groups")) != 1 || a.str("groups", 0, "name") != "Biedronka" || a.str("total", "amount") != "8.00" {
		t.Errorf("by payee in cash = %s", a.raw)
	}
	a = ok(t, s, "spending_by", map[string]any{"group_by": "vendor"})
	if a.get("groups", 0, "key") != nil || a.get("groups", 0, "name") != nil {
		t.Errorf("by vendor = %s", a.raw)
	}
	refused(t, s, "spending_by", map[string]any{"group_by": "category"}, codeInvalid)
	refused(t, s, "spending_by", map[string]any{}, codeInvalid)
	refused(t, s, "spending_by", map[string]any{"group_by": "wallet", "wallet": "Oszczędności"}, codeNotFound)

	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeTransactionsRead}))
	if m := refused(t, s, "spending_by", map[string]any{"group_by": "wallet"}, codeForbidden).message(); !strings.Contains(m, "reports:read") {
		t.Errorf("message = %q", m)
	}
}

func TestListVendors(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.member, nil))
	a := ok(t, s, "list_vendors", nil)
	if a.str("vendors", 0, "key") != "openai" || a.str("vendors", 0, "name") != "OpenAI" {
		t.Errorf("vendors = %s", a.raw)
	}
}

func TestListImportsTool(t *testing.T) {
	w := newWorld(t)
	_, err := usecase.NewImportTransactions(w.deps).Do(ctx, usecase.Principal{User: w.member}, "h1", usecase.ImportInput{
		WalletID: "w-card", Source: imports.SourceCSV, SourceRef: "wyciag.csv",
		Rows: []usecase.ImportedTransaction{{Row: 1, Date: mustDate("2026-10-20"), Amount: "-5.00", PayeeRaw: "Kiosk"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeTransactionsRead}))
	a := ok(t, s, "list_imports", map[string]any{"wallet": "Karta"})
	if len(a.list("imports")) != 1 || a.str("imports", 0, "source_ref") != "wyciag.csv" || a.get("imports", 0, "created") != float64(1) ||
		a.get("imports", 0, "reverted_at") != nil || a.get("next_cursor") != nil {
		t.Errorf("imports = %s", a.raw)
	}
	a = ok(t, s, "list_imports", map[string]any{"wallet": "Gotówka"})
	if len(a.list("imports")) != 0 {
		t.Errorf("cash imports = %s", a.raw)
	}
	refused(t, s, "list_imports", map[string]any{"wallet": "Oszczędności"}, codeNotFound)
	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeWalletsRead}))
	refused(t, s, "list_imports", nil, codeForbidden)
}

func mustDate(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}
