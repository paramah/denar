package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"git.eiac.dev/tools/denar/internal/usecase/fakes"
)

// bearer adds a bot token to every request of the MCP client.
type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// mcpCall runs a tool and decodes its structured content.
func mcpCall(t *testing.T, s *sdk.ClientSession, tool string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: %s", tool, raw)
	}
	return out, res.IsError
}

// TestMCPEndToEnd is the walk of stage 006: a person sets up a household
// with a card, an envelope and a subscription over REST and gives an
// assistant a bot token; the assistant connects with the MCP client of
// go-sdk over HTTP, learns where it is, records an expense by names, asks
// what renews and what subscriptions cost, and is refused a budget write
// it has no scope for. REST then shows the expense as the bot's.
func TestMCPEndToEnd(t *testing.T) {
	nbp := &fakeNBP{rates: map[string]string{}}
	nbpServer := httptest.NewServer(nbp)
	defer nbpServer.Close()
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_NBP_URL", nbpServer.URL)
	clock := fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC))
	e := startE2EWith(t, appOptions{clock: clock, noSubscriptionJobs: true})

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
	admin.must(201, "POST", api+"/budget/envelope-groups", `{"name":"Codzienne"}`, &group)
	admin.must(201, "POST", api+"/budget/envelopes", `{"group_id":"`+group.ID+`","name":"Jedzenie"}`, &envelope)
	admin.must(201, "POST", api+"/subscriptions", `{"name":"Netflix","vendor":"netflix","price":{"amount":"43.00","currency":"PLN"},
		"cycle":{"unit":"month","anchor":"2026-10-25"},"wallet_id":"`+card.ID+`"}`, nil)
	var created botWithToken
	admin.must(201, "POST", api+"/bots", `{"name":"Claude","scopes":["wallets:read","transactions:read","transactions:write","subscriptions:read"]}`, &created)

	// Without a token: the one 401.
	resp, err := http.Post(e.base+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("no token: %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "e2e-assistant", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{
		Endpoint:   e.base + "/mcp",
		HTTPClient: &http.Client{Transport: bearer{token: created.Token}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	me, isErr := mcpCall(t, session, "whoami", nil)
	household := me["household"].(map[string]any)
	wallets := me["wallets"].([]any)
	if isErr || household["id"] != h.ID || household["base_currency"] != "PLN" || me["today"] != "2026-10-21" || len(wallets) != 1 {
		t.Fatalf("whoami = %v", me)
	}

	spent, isErr := mcpCall(t, session, "record_expense", map[string]any{"wallet": "karta", "amount": "42.50", "payee": "Biedronka",
		"envelope": "jedzenie"})
	tx, _ := spent["transaction"].(map[string]any)
	if isErr || tx["source"] != "bot" || tx["envelope_id"] != envelope.ID || tx["date"] != "2026-10-21" {
		t.Fatalf("record_expense = %v", spent)
	}
	if amount := tx["amount"].(map[string]any); amount["amount"] != "-42.50" || amount["minor"] != float64(-4250) {
		t.Errorf("amount = %v", amount)
	}

	renewals, isErr := mcpCall(t, session, "upcoming_renewals", map[string]any{"days": 7})
	list, _ := renewals["renewals"].([]any)
	if isErr || len(list) != 1 {
		t.Fatalf("upcoming_renewals = %v", renewals)
	}
	if charge := list[0].(map[string]any)["charge"].(map[string]any); charge["subscription_name"] != "Netflix" || charge["due_date"] != "2026-10-25" {
		t.Errorf("renewal = %v", charge)
	}

	costs, isErr := mcpCall(t, session, "subscription_costs", nil)
	if monthly := costs["monthly"].(map[string]any); isErr || monthly["amount"] != "43.00" || monthly["currency"] != "PLN" {
		t.Fatalf("subscription_costs = %v", costs)
	}

	refusal, isErr := mcpCall(t, session, "assign_to_envelope", map[string]any{"envelope": "Jedzenie", "amount": "500.00"})
	if !isErr || refusal["code"] != "forbidden" || !strings.Contains(refusal["message"].(string), "budget:write") {
		t.Fatalf("assign_to_envelope = %v", refusal)
	}

	// The person sees the expense, made by the bot, and the audit says so.
	var page struct {
		Items []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
		} `json:"items"`
	}
	admin.must(200, "GET", api+"/transactions", "", &page)
	if len(page.Items) != 1 || page.Items[0].ID != tx["id"] || page.Items[0].Source != "bot" {
		t.Errorf("transactions = %+v", page)
	}
	var audit struct {
		Items []struct {
			Action string `json:"action"`
			Actor  struct {
				Kind string `json:"kind"`
			} `json:"actor"`
		} `json:"items"`
	}
	admin.must(200, "GET", api+"/audit?actor_kind=bot", "", &audit)
	actions := map[string]bool{}
	for _, a := range audit.Items {
		if a.Actor.Kind == "bot" {
			actions[a.Action] = true
		}
	}
	if !actions["transaction.created"] || !actions["payee.created"] {
		t.Errorf("audit by the bot = %+v", audit)
	}
}
