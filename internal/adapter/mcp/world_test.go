package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
	"github.com/paramah/denar/internal/usecase/fakes"
)

var ctx = context.Background()

// world is the adapter on the real use cases over fake ports, behind an
// HTTP test server: the household h1 (PLN, Europe/Warsaw) of the owner
// Ada, with the member Bob and the viewer Cyd; the shared wallets Karta
// and Gotówka, Ada's private wallet Oszczędności; the envelopes Jedzenie
// and Rozrywka; and the household h2 of an outsider.
type world struct {
	t *testing.T
	*fakes.Repos
	deps usecase.Deps
	log  *syncBuffer

	owner, member, viewer, outsider user.User
	srv                             *httptest.Server
}

// syncBuffer collects the log of a server that writes from its own goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newWorld starts the server; edit may change the dependencies first.
func newWorld(t *testing.T, edit ...func(*Deps)) *world {
	t.Helper()
	r := fakes.NewRepos()
	w := &world{t: t, Repos: r, deps: r.Deps(), log: &syncBuffer{}}
	w.owner, w.member, w.viewer, w.outsider = w.addUser("ada"), w.addUser("bob"), w.addUser("cyd"), w.addUser("out")
	w.addHousehold("h1", w.owner)
	w.addMember("h1", w.member, membership.RoleMember)
	w.addMember("h1", w.viewer, membership.RoleViewer)
	w.addHousehold("h2", w.outsider)
	w.addWallet("w-card", "Karta", "PLN", wallet.Shared)
	w.addWallet("w-cash", "Gotówka", "PLN", wallet.Shared)
	w.addWallet("w-priv", "Oszczędności", "PLN", wallet.Private)
	w.must(r.EnvelopeGroups.Create(ctx, budget.Group{ID: "g1", HouseholdID: "h1", Name: "Codzienne", CreatedAt: w.now(), UpdatedAt: w.now()}))
	w.must(r.Envelopes.Create(ctx, budget.Envelope{ID: "e-food", HouseholdID: "h1", GroupID: "g1", Name: "Jedzenie", CreatedAt: w.now(), UpdatedAt: w.now()}))
	w.must(r.Envelopes.Create(ctx, budget.Envelope{ID: "e-fun", HouseholdID: "h1", GroupID: "g1", Name: "Rozrywka", CreatedAt: w.now(), UpdatedAt: w.now()}))

	deps := Deps{
		Logger:       slog.New(slog.NewTextHandler(w.log, nil)),
		Authenticate: usecase.NewAuthenticateBot(r.Bots, r.Clock, nil),
		UseCases:     w.deps,
	}
	for _, change := range edit {
		change(&deps)
	}
	w.srv = httptest.NewServer(NewHandler(deps))
	t.Cleanup(w.srv.Close)
	return w
}

func (w *world) now() time.Time { return w.Clock.Now() }

func (w *world) must(err error) {
	w.t.Helper()
	if err != nil {
		w.t.Fatalf("setup: %v", err)
	}
}

func (w *world) addUser(id domain.ID) user.User {
	w.t.Helper()
	u := user.User{ID: id, Email: string(id) + "@example.com", Name: string(id), Active: true, CreatedAt: w.now()}
	w.must(w.Users.Create(ctx, u))
	return u
}

func (w *world) addHousehold(id domain.ID, owner user.User) {
	w.t.Helper()
	w.must(w.Households.Create(ctx, household.Household{ID: id, Name: "Dom " + string(id), BaseCurrency: "PLN", Timezone: "Europe/Warsaw",
		MonthStartDay: 1, CreatedBy: owner.ID, CreatedAt: w.now()}))
	w.addMember(id, owner, membership.RoleOwner)
}

func (w *world) addMember(householdID domain.ID, u user.User, role membership.Role) {
	w.t.Helper()
	w.must(w.Memberships.Create(ctx, membership.Membership{HouseholdID: householdID, UserID: u.ID, Role: role, CreatedAt: w.now()}))
}

// addWallet adds a wallet of h1 created by the owner; a private one is the
// owner's alone.
func (w *world) addWallet(id domain.ID, name, currency string, v wallet.Visibility) {
	w.t.Helper()
	w.must(w.Wallets.Create(ctx, wallet.Wallet{ID: id, HouseholdID: "h1", Name: name, Type: wallet.TypeCard, Currency: currency,
		OnBudget: true, Visibility: v, CreatedBy: w.owner.ID, CreatedAt: w.now()}.Normalize()))
}

func botToken(id domain.ID) string { return "dnr_token-of-" + string(id) }

// addBot stores a bot of h1 and returns its token; wallets limit it.
func (w *world) addBot(id domain.ID, creator user.User, scopes []bot.Scope, wallets ...domain.ID) string {
	w.t.Helper()
	if wallets == nil {
		wallets = []domain.ID{}
	}
	household := domain.ID("h1")
	if creator.ID == w.outsider.ID {
		household = "h2"
	}
	b := bot.Bot{ID: id, HouseholdID: household, Name: "Bot " + string(id), Scopes: scopes, WalletIDs: wallets, CreatedBy: creator.ID,
		CreatedAt: w.now(), TokenHash: usecase.HashToken(botToken(id)), TokenPrefix: "dnr_toke"}
	w.must(w.Bots.Create(ctx, b))
	return botToken(id)
}

// allScopes is every scope there is.
var allScopes = bot.Scopes

// connect opens a session of the SDK's client with the bot token.
func (w *world) connect(token string) *sdk.ClientSession {
	w.t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "test-assistant", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint:   w.srv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: token}},
	}, nil)
	if err != nil {
		w.t.Fatalf("connect: %v", err)
	}
	w.t.Cleanup(func() { _ = session.Close() })
	return session
}

// answer is the result of a tool call, read the way a model reads it.
type answer struct {
	isError bool
	raw     string
	data    map[string]any
}

func (a answer) code() string    { s, _ := a.data["code"].(string); return s }
func (a answer) message() string { s, _ := a.data["message"].(string); return s }

// get walks the answer by keys and indexes, such as get("transaction", "amount", "amount").
func (a answer) get(path ...any) any {
	var v any = a.data
	for _, k := range path {
		switch key := k.(type) {
		case string:
			m, _ := v.(map[string]any)
			v = m[key]
		case int:
			l, _ := v.([]any)
			if key >= len(l) {
				return nil
			}
			v = l[key]
		}
	}
	return v
}

func (a answer) str(path ...any) string { s, _ := a.get(path...).(string); return s }

func (a answer) list(path ...any) []any { l, _ := a.get(path...).([]any); return l }

// call runs a tool and checks what the contract promises of every answer:
// the structured content and the same JSON as the only text.
func call(t *testing.T, session *sdk.ClientSession, tool string, arguments map[string]any) answer {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: arguments})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	a := answer{isError: res.IsError, raw: string(raw)}
	if err := json.Unmarshal(raw, &a.data); err != nil {
		t.Fatalf("%s: the structured content is not an object: %s", tool, raw)
	}
	if len(res.Content) != 1 {
		t.Fatalf("%s: %d content blocks, want the one text", tool, len(res.Content))
	}
	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("%s: content is %T, want text", tool, res.Content[0])
	}
	var fromText any
	if err := json.Unmarshal([]byte(text.Text), &fromText); err != nil {
		t.Fatalf("%s: the text is not JSON: %s", tool, text.Text)
	}
	again, _ := json.Marshal(fromText)
	if string(again) != string(raw) {
		t.Fatalf("%s: text and structured content differ:\n text: %s\n structured: %s", tool, text.Text, raw)
	}
	return a
}

// ok runs a tool that must succeed.
func ok(t *testing.T, session *sdk.ClientSession, tool string, arguments map[string]any) answer {
	t.Helper()
	a := call(t, session, tool, arguments)
	if a.isError {
		t.Fatalf("%s: unexpected error %s", tool, a.raw)
	}
	return a
}

// refused runs a tool that must end in the error code.
func refused(t *testing.T, session *sdk.ClientSession, tool string, arguments map[string]any, code string) answer {
	t.Helper()
	a := call(t, session, tool, arguments)
	if !a.isError || a.code() != code {
		t.Fatalf("%s: want the error %q, got isError=%v %s", tool, code, a.isError, a.raw)
	}
	if a.message() == "" || len(a.data) != 2 {
		t.Fatalf("%s: an error is {code, message}, got %s", tool, a.raw)
	}
	return a
}

// raw sends one request without the SDK's client and returns the response
// with its body read.
func (w *world) raw(method, body string, edit func(*http.Request)) (*http.Response, string) {
	w.t.Helper()
	req, err := http.NewRequest(method, w.srv.URL+"/mcp", bytes.NewReader([]byte(body)))
	w.must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if edit != nil {
		edit(req)
	}
	res, err := http.DefaultClient.Do(req)
	w.must(err)
	defer res.Body.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(res.Body)
	w.must(err)
	return res, buf.String()
}

func withToken(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

const listTools = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
