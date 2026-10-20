package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"

	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// specOperation is one operation of api/openapi.yaml.
type specOperation struct {
	id     string
	method string
	path   string   // as in the contract, without the /api prefix
	public bool     // "security: []"
	codes  []string // the responses it declares
}

// specOperations reads the operations from the contract, so that a new one
// cannot be forgotten here.
func specOperations(t *testing.T) []specOperation {
	t.Helper()
	spec, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	var ops []specOperation
	for path, item := range model.Model.Paths.PathItems.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			ops = append(ops, specOperation{
				id:     op.OperationId,
				method: strings.ToUpper(method),
				path:   path,
				public: op.Security != nil && len(op.Security) == 0,
				codes:  slices.Collect(op.Responses.Codes.KeysFromOldest()),
			})
		}
	}
	if len(ops) == 0 {
		t.Fatal("the contract has no operations")
	}
	return ops
}

// conformanceCase is one way to call an operation and the status the
// contract promises for it.
type conformanceCase struct {
	status int
	as     string // a user id, "bot" for b1, "" for nobody
	target string
	body   string
	// setup changes the world or the client before the call.
	setup func(t *testing.T, w *world, h http.Handler, c *client)
}

type setupFunc = func(t *testing.T, w *world, h http.Handler, c *client)

func oidcOn(_ *testing.T, w *world, _ http.Handler, _ *client) { w.oidc.On = true }

// oidcStarted begins an OIDC login and leaves the client holding the state
// cookie, with the state as {state}.
func oidcStarted(t *testing.T, w *world, h http.Handler, c *client) {
	w.oidc.On = true
	w.oidc.Identity = usecase.OIDCIdentity{Issuer: "https://idp.test", Subject: "sub-1", Email: "grace@example.com",
		EmailVerified: ptr(true), Name: "Grace"}
	rec := (&client{handler: h}).do("GET", "/api/auth/login", "")
	state := cookie(rec, oidcCookieName)
	if rec.Code != 302 || state == nil {
		t.Fatalf("login start = %d", rec.Code)
	}
	c.cookies = []*http.Cookie{{Name: oidcCookieName, Value: state.Value}}
	w.vars["state"] = url.QueryEscape(state.Value)
}

func ptr[T any](v T) *T { return &v }

// invited creates an invitation of grace@example.com to h1 as a member:
// {invitation} is its id and {token} its token.
func invited(t *testing.T, w *world, _ http.Handler, _ *client) {
	inviteAs(t, w, "grace@example.com", "", "")
}

func inviteAs(t *testing.T, w *world, email, idVar, tokenVar string) {
	t.Helper()
	inv, err := w.deps.CreateInvitation.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "owner")}, "h1",
		usecase.CreateInvitationInput{Email: email, Role: membership.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	w.vars[cmpOr(idVar, "invitation")] = string(inv.Invitation.ID)
	w.vars[cmpOr(tokenVar, "token")] = inv.Token
}

func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func mustUser(t *testing.T, w *world, id string) usecaseUser {
	t.Helper()
	u, err := w.Users.ByID(context.Background(), domainID(id))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// invitedMember invites member@example.com, who already has an account.
func invitedMember(t *testing.T, w *world, _ http.Handler, _ *client) {
	inviteAs(t, w, "member@example.com", "", "")
}

// usedInvitation is an invitation the outsider has accepted.
func usedInvitation(t *testing.T, w *world, h http.Handler, c *client) {
	invited(t, w, h, c)
	if _, err := w.deps.AcceptInvitation.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "outsider")}, w.vars["token"]); err != nil {
		t.Fatal(err)
	}
}

// secondBot adds a bot named "Other" of member.
func secondBot(_ *testing.T, w *world, _ http.Handler, _ *client) { w.addBot("b2", "h1", "member") }

// exhaust sends n requests of method to target as the client, to use up a
// limit.
func exhaust(n int, method, target, body string) setupFunc {
	return func(t *testing.T, w *world, h http.Handler, c *client) {
		for range n {
			(&client{handler: h}).do(method, target, strings.ReplaceAll(body, "{token}", w.vars["token"]))
		}
	}
}

func then(setups ...setupFunc) setupFunc {
	return func(t *testing.T, w *world, h http.Handler, c *client) {
		for _, s := range setups {
			s(t, w, h, c)
		}
	}
}

const badLogin = `{"email":"owner@example.com","password":"not the password"}`

// conformanceCases covers, for every operation, every response the
// contract declares. The 401 of an operation that needs credentials is
// added by the test itself.
var conformanceCases = map[string][]conformanceCase{
	"getAuthConfig": {{status: 200, target: "/api/auth/config"}},
	"loginStart": {
		{status: 302, target: "/api/auth/login?return_to=/x", setup: oidcOn},
		{status: 404, target: "/api/auth/login"},
		{status: 429, target: "/api/auth/login", setup: then(oidcOn, exhaust(20, "GET", "/api/auth/login", ""))},
	},
	"loginLocal": {
		{status: 200, target: "/api/auth/login", body: `{"email":"owner@example.com","password":"password-of-owner"}`},
		{status: 401, target: "/api/auth/login", body: badLogin},
		{status: 422, target: "/api/auth/login", body: `{"email":7}`},
		{status: 429, target: "/api/auth/login", body: `{"email":"owner@example.com","password":"password-of-owner"}`,
			setup: exhaust(5, "POST", "/api/auth/login", badLogin)},
	},
	"loginCallback": {
		{status: 302, target: "/api/auth/callback?code=c&state={state}", setup: oidcStarted},
		{status: 401, target: "/api/auth/callback?code=c&state=unknown", setup: oidcOn},
	},
	"logout": {
		{status: 204, as: "owner", target: "/api/auth/logout"},
		{status: 204, target: "/api/auth/logout"},
	},
	"getMe": {
		{status: 200, as: "owner", target: "/api/me"},
		{status: 200, as: "bot", target: "/api/me"},
	},
	"listCurrencies": {{status: 200, as: "viewer", target: "/api/currencies"}},
	"listHouseholds": {{status: 200, as: "owner", target: "/api/households"}, {status: 200, as: "bot", target: "/api/households"}},
	"createHousehold": {
		{status: 201, as: "member", target: "/api/households", body: `{"name":"Dom 2","base_currency":"EUR","timezone":"Europe/Berlin","month_start_day":25}`},
		{status: 403, as: "bot", target: "/api/households", body: `{"name":"X","base_currency":"EUR","timezone":"UTC","month_start_day":1}`},
		{status: 422, as: "member", target: "/api/households", body: `{"name":"","base_currency":"XYZ","timezone":"Nowhere","month_start_day":0}`},
	},
	"getHousehold": {
		{status: 200, as: "viewer", target: "/api/households/h1"},
		{status: 200, as: "bot", target: "/api/households/h1"},
		{status: 404, as: "outsider", target: "/api/households/h1"},
		{status: 404, as: "bot", target: "/api/households/h2"},
	},
	"updateHousehold": {
		{status: 200, as: "admin", target: "/api/households/h1", body: `{"name":"Nowy dom","month_start_day":10}`},
		{status: 403, as: "member", target: "/api/households/h1", body: `{"name":"X"}`},
		{status: 404, as: "outsider", target: "/api/households/h1", body: `{"name":"X"}`},
		{status: 422, as: "owner", target: "/api/households/h1", body: `{"month_start_day":31}`},
	},
	"deleteHousehold": {
		{status: 204, as: "owner", target: "/api/households/h1"},
		{status: 403, as: "admin", target: "/api/households/h1"},
		{status: 404, as: "outsider", target: "/api/households/h1"},
	},
	"listMembers": {
		{status: 200, as: "viewer", target: "/api/households/h1/members"},
		{status: 403, as: "bot", target: "/api/households/h1/members"},
		{status: 404, as: "outsider", target: "/api/households/h1/members"},
	},
	"updateMember": {
		{status: 200, as: "owner", target: "/api/households/h1/members/member", body: `{"role":"viewer"}`},
		{status: 403, as: "admin", target: "/api/households/h1/members/owner", body: `{"role":"member"}`},
		{status: 404, as: "owner", target: "/api/households/h1/members/nobody", body: `{"role":"member"}`},
		{status: 409, as: "owner", target: "/api/households/h1/members/owner", body: `{"role":"admin"}`},
		{status: 422, as: "owner", target: "/api/households/h1/members/member", body: `{"role":"boss"}`},
	},
	"removeMember": {
		{status: 204, as: "admin", target: "/api/households/h1/members/member"},
		{status: 403, as: "viewer", target: "/api/households/h1/members/member"},
		{status: 404, as: "owner", target: "/api/households/h1/members/nobody"},
		{status: 409, as: "owner", target: "/api/households/h1/members/owner"},
	},
	"listInvitations": {
		{status: 200, as: "admin", target: "/api/households/h1/invitations", setup: invited},
		{status: 403, as: "member", target: "/api/households/h1/invitations"},
		{status: 404, as: "outsider", target: "/api/households/h1/invitations"},
	},
	"createInvitation": {
		{status: 201, as: "admin", target: "/api/households/h1/invitations", body: `{"email":"grace@example.com","role":"member"}`},
		{status: 403, as: "admin", target: "/api/households/h1/invitations", body: `{"email":"grace@example.com","role":"owner"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/invitations", body: `{"email":"grace@example.com","role":"member"}`},
		{status: 422, as: "owner", target: "/api/households/h1/invitations", body: `{"email":"nope","role":"member"}`},
	},
	"revokeInvitation": {
		{status: 204, as: "admin", target: "/api/households/h1/invitations/{invitation}", setup: invited},
		{status: 403, as: "member", target: "/api/households/h1/invitations/{invitation}", setup: invited},
		{status: 404, as: "admin", target: "/api/households/h1/invitations/nope"},
		{status: 409, as: "admin", target: "/api/households/h1/invitations/{invitation}", setup: usedInvitation},
	},
	"previewInvitation": {
		{status: 200, target: "/api/invitations/preview", body: `{"token":"{token}"}`, setup: invited},
		{status: 404, target: "/api/invitations/preview", body: `{"token":"nope"}`},
		{status: 409, target: "/api/invitations/preview", body: `{"token":"{token}"}`, setup: usedInvitation},
		{status: 429, target: "/api/invitations/preview", body: `{"token":"nope"}`, setup: exhaust(20, "POST", "/api/invitations/preview", `{"token":"x"}`)},
	},
	"acceptInvitation": {
		{status: 200, as: "outsider", target: "/api/invitations/accept", body: `{"token":"{token}"}`, setup: invited},
		{status: 403, as: "bot", target: "/api/invitations/accept", body: `{"token":"{token}"}`, setup: invited},
		{status: 404, as: "outsider", target: "/api/invitations/accept", body: `{"token":"nope"}`},
		{status: 409, as: "member", target: "/api/invitations/accept", body: `{"token":"{token}"}`, setup: invitedMember},
	},
	"registerWithInvitation": {
		{status: 201, target: "/api/invitations/register", body: `{"token":"{token}","name":"Grace","password":"a long password!"}`, setup: invited},
		{status: 404, target: "/api/invitations/register", body: `{"token":"nope","name":"Grace","password":"a long password!"}`},
		{status: 409, target: "/api/invitations/register", body: `{"token":"{token}","name":"M","password":"a long password!"}`, setup: invitedMember},
		{status: 422, target: "/api/invitations/register", body: `{"token":"{token}","name":"Grace","password":"short"}`, setup: invited},
		{status: 429, target: "/api/invitations/register", body: `{"token":"nope","name":"G","password":"a long password!"}`,
			setup: exhaust(20, "POST", "/api/invitations/register", `{"token":"x","name":"G","password":"a long password!"}`)},
	},
	"listBots": {
		{status: 200, as: "member", target: "/api/households/h1/bots"},
		{status: 403, as: "bot", target: "/api/households/h1/bots"},
		{status: 404, as: "outsider", target: "/api/households/h1/bots"},
	},
	"createBot": {
		{status: 201, as: "member", target: "/api/households/h1/bots", body: `{"name":"Claude","scopes":["transactions:write","wallets:read"],"wallet_ids":["w1"],"expires_at":"2027-01-01T00:00:00Z"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/bots", body: `{"name":"Claude","scopes":["wallets:read"]}`},
		{status: 404, as: "outsider", target: "/api/households/h1/bots", body: `{"name":"Claude","scopes":["wallets:read"]}`},
		{status: 409, as: "member", target: "/api/households/h1/bots", body: `{"name":"bot b1","scopes":["wallets:read"]}`},
		{status: 422, as: "member", target: "/api/households/h1/bots", body: `{"name":"Claude","scopes":["everything"]}`},
	},
	"getBot": {
		{status: 200, as: "member", target: "/api/households/h1/bots/b1"},
		{status: 403, as: "viewer", target: "/api/households/h1/bots/b1"},
		{status: 404, as: "member", target: "/api/households/h1/bots/nope"},
	},
	"updateBot": {
		{status: 200, as: "admin", target: "/api/households/h1/bots/b1", body: `{"name":"Renamed","disabled":true,"expires_at":null,"scopes":["reports:read"]}`},
		{status: 403, as: "viewer", target: "/api/households/h1/bots/b1", body: `{"disabled":true}`},
		{status: 404, as: "member", target: "/api/households/h1/bots/nope", body: `{"disabled":true}`},
		{status: 409, as: "member", target: "/api/households/h1/bots/b1", body: `{"name":"Bot b2"}`, setup: secondBot},
		{status: 422, as: "member", target: "/api/households/h1/bots/b1", body: `{"scopes":[]}`},
	},
	"deleteBot": {
		{status: 204, as: "member", target: "/api/households/h1/bots/b1"},
		{status: 403, as: "viewer", target: "/api/households/h1/bots/b1"},
		{status: 404, as: "member", target: "/api/households/h1/bots/nope"},
	},
	"rotateBotToken": {
		{status: 200, as: "member", target: "/api/households/h1/bots/b1/token"},
		{status: 403, as: "viewer", target: "/api/households/h1/bots/b1/token"},
		{status: 404, as: "member", target: "/api/households/h1/bots/nope/token"},
	},
	"listAudit": {
		{status: 200, as: "admin", target: "/api/households/h1/audit?limit=1&resource_type=invitation", setup: then(invited, invited)},
		{status: 200, as: "owner", target: "/api/households/h1/audit?actor_kind=user&actor_id=owner"},
		{status: 403, as: "member", target: "/api/households/h1/audit"},
		{status: 404, as: "outsider", target: "/api/households/h1/audit"},
		{status: 422, as: "owner", target: "/api/households/h1/audit?cursor=garbage"},
	},
}

// problemCodes is the code a problem carries for its status.
var problemCodes = map[int]string{
	400: "bad_request",
	401: "unauthenticated",
	403: "forbidden",
	404: "not_found",
	409: "conflict",
	422: "validation",
	429: "rate_limited",
}

// matchesPath reports whether a request path is an instance of a contract
// path such as /households/{householdId}.
func matchesPath(pattern, path string) bool {
	want, got := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if !strings.HasPrefix(want[i], "{") && want[i] != got[i] {
			return false
		}
	}
	return true
}

// run performs one case on a fresh world and checks the answer against the
// case and the contract.
func (c conformanceCase) run(t *testing.T, op specOperation) {
	w := newWorld(t)
	h := w.handler()
	cl := w.as(h, c.as)
	if c.setup != nil {
		c.setup(t, w, h, cl)
	}
	target, body := c.target, c.body
	for name, value := range w.vars {
		target = strings.ReplaceAll(target, "{"+name+"}", value)
		body = strings.ReplaceAll(body, "{"+name+"}", value)
	}
	path, _, _ := strings.Cut(strings.TrimPrefix(target, "/api"), "?")
	if !matchesPath(op.path, path) {
		t.Fatalf("target %s is not an instance of %s", target, op.path)
	}

	rec := cl.do(op.method, target, body)

	if rec.Code != c.status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, c.status, rec.Body)
	}
	wantContract(t, op.method, target, rec)
	wantProblemShape(t, rec)
	if rec.Code == 429 && rec.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
}

// wantProblemShape checks that an error answer is a problem with the code
// of its status, and that a success is not a problem.
func wantProblemShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	contentType := rec.Header().Get("Content-Type")
	if rec.Code < 400 {
		if contentType == "application/problem+json" {
			t.Errorf("status %d is answered with a problem", rec.Code)
		}
		return
	}
	if contentType != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", contentType)
	}
	var problem struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body, err)
	}
	if problem.Status != rec.Code || problem.Code != problemCodes[rec.Code] || problem.Title == "" ||
		!strings.HasPrefix(problem.Type, problemTypePrefix) {
		t.Errorf("problem = %+v for status %d", problem, rec.Code)
	}
}

// TestContractConformance calls every operation of api/openapi.yaml once
// for every response it declares and validates each answer against the
// contract (FR-050, FR-051).
func TestContractConformance(t *testing.T) {
	ops := specOperations(t)
	known := map[string]bool{}
	for _, op := range ops {
		known[op.id] = true
		t.Run(op.id, func(t *testing.T) {
			cases := conformanceCases[op.id]
			if len(cases) == 0 {
				t.Fatalf("%s %s has no conformance cases", op.method, op.path)
			}
			if !op.public {
				unauthenticated := cases[0]
				unauthenticated.status, unauthenticated.as = 401, ""
				cases = append(slices.Clone(cases), unauthenticated)
			}
			covered := map[string]bool{}
			for i, c := range cases {
				covered[strconv.Itoa(c.status)] = true
				t.Run(fmt.Sprintf("%d#%d", c.status, i), func(t *testing.T) { c.run(t, op) })
			}
			for _, code := range op.codes {
				if !covered[code] {
					t.Errorf("declared response %s is not exercised", code)
				}
			}
			for code := range covered {
				if !slices.Contains(op.codes, code) {
					t.Errorf("response %s is exercised but not declared", code)
				}
			}
		})
	}
	for id := range conformanceCases {
		if !known[id] {
			t.Errorf("conformance cases for %q, which the contract does not have", id)
		}
	}
}
