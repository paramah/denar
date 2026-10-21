package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeNBP serves table A rates like api.nbp.pl:
// /api/exchangerates/rates/a/{code}/{start}/{end}/?format=json.
type fakeNBP struct {
	mu    sync.Mutex
	down  bool
	rates map[string]string // "usd/2026-10-20" → mid
	calls int
}

func (f *fakeNBP) set(code, date, mid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rates[code+"/"+date] = mid
}

func (f *fakeNBP) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

func (f *fakeNBP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if f.down || len(parts) != 7 || r.URL.Query().Get("format") != "json" {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	code, start, end := parts[4], parts[5], parts[6]
	var items []string
	for key, mid := range f.rates {
		c, date, _ := strings.Cut(key, "/")
		if c == code && date >= start && date <= end {
			items = append(items, fmt.Sprintf(`{"no":"x","effectiveDate":%q,"mid":%s}`, date, mid))
		}
	}
	if len(items) == 0 {
		http.Error(w, "404 NotFound - Not Found - Brak danych", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"table":"A","code":%q,"rates":[%s]}`, strings.ToUpper(code), strings.Join(items, ","))
}

type money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Minor    int64  `json:"minor"`
}

type wallet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Balance     money  `json:"balance"`
	BaseBalance money  `json:"base_balance"`
	RatePending bool   `json:"rate_pending"`
}

type txn struct {
	ID             string  `json:"id"`
	Amount         money   `json:"amount"`
	BaseAmount     *money  `json:"base_amount"`
	FxRate         *string `json:"fx_rate"`
	RatePending    bool    `json:"rate_pending"`
	Source         string  `json:"source"`
	Kind           string  `json:"kind"`
	TransferPairID *string `json:"transfer_pair_id"`
}

// TestFinancesEndToEnd is the walk of stage 002: a PLN and a USD wallet, a
// USD expense converted at the rate of a fake NBP, a transfer between the
// two, the balances; a bot limited to one wallet; an expense recorded
// offline that the job converts later.
func TestFinancesEndToEnd(t *testing.T) {
	nbp := &fakeNBP{rates: map[string]string{}}
	nbp.set("usd", "2026-10-20", "4.0000")
	nbpServer := httptest.NewServer(nbp)
	defer nbpServer.Close()
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_NBP_URL", nbpServer.URL)
	e := startE2E(t)

	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	api := "/api/households/" + h.ID

	var pln, usd wallet
	admin.must(201, "POST", api+"/wallets", `{"name":"Konto","type":"checking","currency":"PLN",
		"opening_balance":{"amount":"1000.00","currency":"PLN"},"opened_on":"2026-10-01"}`, &pln)
	admin.must(201, "POST", api+"/wallets", `{"name":"Dolary","type":"cash","currency":"USD"}`, &usd)
	if pln.Balance.Amount != "1000.00" || pln.BaseBalance.Amount != "1000.00" || usd.Balance.Amount != "0.00" {
		t.Fatalf("wallets = %+v %+v", pln, usd)
	}
	if nbp.calls != 0 {
		t.Errorf("PLN asked NBP %d times", nbp.calls)
	}

	var spent txn
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+usd.ID+`","date":"2026-10-20",
		"amount":{"amount":"-10.00","currency":"USD"},"payee_name":"Amazon","tags":["Books"]}`, &spent)
	if spent.BaseAmount == nil || spent.BaseAmount.Amount != "-40.00" || spent.BaseAmount.Currency != "PLN" ||
		spent.FxRate == nil || *spent.FxRate != "4" || spent.RatePending || spent.Source != "manual" {
		t.Fatalf("USD expense = %+v", spent)
	}

	var transfer struct {
		From txn `json:"from"`
		To   txn `json:"to"`
	}
	admin.must(201, "POST", api+"/transfers", `{"from_wallet_id":"`+pln.ID+`","to_wallet_id":"`+usd.ID+`","date":"2026-10-20",
		"amount":{"amount":"400.00","currency":"PLN"},"to_amount":{"amount":"100.00","currency":"USD"}}`, &transfer)
	if transfer.From.Kind != "transfer" || transfer.From.TransferPairID == nil || transfer.To.TransferPairID == nil ||
		*transfer.From.TransferPairID != *transfer.To.TransferPairID || transfer.To.BaseAmount.Amount != "400.00" {
		t.Fatalf("transfer = %+v", transfer)
	}

	balances := func() map[string]wallet {
		t.Helper()
		var list struct {
			Items []wallet `json:"items"`
		}
		admin.must(200, "GET", api+"/wallets", "", &list)
		out := map[string]wallet{}
		for _, w := range list.Items {
			out[w.Name] = w
		}
		return out
	}
	ws := balances()
	if ws["Konto"].Balance.Amount != "600.00" || ws["Dolary"].Balance.Amount != "90.00" || ws["Dolary"].Balance.Currency != "USD" ||
		ws["Dolary"].BaseBalance.Amount != "360.00" || ws["Dolary"].BaseBalance.Currency != "PLN" {
		t.Errorf("balances = %+v", ws)
	}

	// A bot limited to the PLN wallet records there, as a bot, and does
	// not see the other wallet.
	var created botWithToken
	admin.must(201, "POST", api+"/bots", `{"name":"Claude","scopes":["wallets:read","transactions:read","transactions:write"],
		"wallet_ids":["`+pln.ID+`"]}`, &created)
	bot := e.bot(created.Token)
	var byBot txn
	bot.must(201, "POST", api+"/transactions", `{"wallet_id":"`+pln.ID+`","amount":{"amount":"-25.50","currency":"PLN"},"payee_name":"Biedronka"}`, &byBot)
	if byBot.Source != "bot" {
		t.Errorf("bot's transaction = %+v", byBot)
	}
	bot.must(404, "POST", api+"/transactions", `{"wallet_id":"`+usd.ID+`","amount":{"amount":"-1","currency":"USD"}}`, nil)
	bot.must(404, "GET", api+"/wallets/"+usd.ID, "", nil)
	bot.must(403, "POST", api+"/wallets", `{"name":"X","type":"cash","currency":"PLN"}`, nil)
	var botWallets struct {
		Items []wallet `json:"items"`
	}
	bot.must(200, "GET", api+"/wallets", "", &botWallets)
	if len(botWallets.Items) != 1 || botWallets.Items[0].ID != pln.ID {
		t.Errorf("bot sees %+v", botWallets.Items)
	}
	admin.must(422, "POST", api+"/bots", `{"name":"Zły","scopes":["wallets:read"],"wallet_ids":["nope"]}`, nil)

	// Offline: the expense is saved without a base amount; the job fills it
	// in once NBP answers again.
	nbp.setDown(true)
	var offline txn
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+usd.ID+`","date":"2026-10-10","amount":{"amount":"-10.00","currency":"USD"}}`, &offline)
	if !offline.RatePending || offline.BaseAmount != nil {
		t.Fatalf("offline = %+v", offline)
	}
	if !balances()["Dolary"].RatePending {
		t.Error("wallet does not say a rate is pending")
	}
	nbp.setDown(false)
	nbp.set("usd", "2026-10-09", "3.8000")
	n, err := e.app.fillRates(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("job = %d, %v", n, err)
	}
	var filled txn
	admin.must(200, "GET", api+"/transactions/"+offline.ID, "", &filled)
	if filled.RatePending || filled.BaseAmount == nil || filled.BaseAmount.Amount != "-38.00" {
		t.Errorf("filled = %+v", filled)
	}
	ws = balances()
	if ws["Dolary"].RatePending || ws["Dolary"].BaseBalance.Amount != "322.00" || ws["Dolary"].Balance.Amount != "80.00" {
		t.Errorf("balances after the job = %+v", ws["Dolary"])
	}

	// A rate set by hand works without NBP.
	nbp.setDown(true)
	admin.must(200, "PUT", api+"/fx-rates/overrides", `{"currency":"USD","date":"2026-10-05","rate":"4.1"}`, nil)
	var manual txn
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+usd.ID+`","date":"2026-10-05","amount":{"amount":"-1.00","currency":"USD"}}`, &manual)
	if manual.BaseAmount == nil || manual.BaseAmount.Amount != "-4.10" {
		t.Errorf("manual rate = %+v", manual)
	}

	var page struct {
		Items []txn `json:"items"`
	}
	admin.must(200, "GET", api+"/transactions?wallet_id="+usd.ID+"&from=2026-10-01&to=2026-10-31", "", &page)
	if len(page.Items) != 4 {
		t.Errorf("USD wallet has %d transactions", len(page.Items))
	}
	admin.must(200, "GET", api+"/transactions?q=amaz", "", &page)
	if len(page.Items) != 1 || page.Items[0].ID != spent.ID {
		t.Errorf("search = %+v", page.Items)
	}

	// Deleting one leg deletes the transfer.
	admin.must(204, "DELETE", api+"/transactions/"+transfer.To.ID, "", nil)
	admin.must(404, "GET", api+"/transactions/"+transfer.From.ID, "", nil)
	if got := balances()["Konto"].Balance.Amount; got != "974.50" {
		t.Errorf("Konto after deleting the transfer = %s", got)
	}
}
