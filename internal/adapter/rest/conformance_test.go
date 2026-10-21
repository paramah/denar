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

	"git.eiac.dev/tools/denar/internal/domain/bot"
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

// secondChannel gives member a second channel "Telefon".
func secondChannel(t *testing.T, w *world, _ http.Handler, _ *client) {
	t.Helper()
	_, err := w.deps.CreateChannel.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "member")},
		usecase.CreateChannelInput{Type: "ntfy", Name: "Telefon", Config: map[string]string{"topic": "t"}})
	if err != nil {
		t.Fatal(err)
	}
}

// income records 1000 PLN of income in w1, so that auto-assign has money
// to give.
func income(t *testing.T, w *world, _ http.Handler, _ *client) {
	t.Helper()
	_, err := w.deps.RecordTransaction.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "owner")}, "h1",
		usecase.RecordTransactionInput{WalletID: "w1", Amount: usecase.MoneyInput{Amount: "1000", Currency: "PLN"}, Kind: "income"})
	if err != nil {
		t.Fatal(err)
	}
}

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

// botScopes gives b1 these scopes instead of wallets:read.
func botScopes(scopes ...bot.Scope) setupFunc {
	return func(t *testing.T, w *world, _ http.Handler, _ *client) {
		b, err := w.Bots.ByID(context.Background(), "b1")
		if err != nil {
			t.Fatal(err)
		}
		b.Scopes = scopes
		w.must(w.Bots.Update(context.Background(), b))
	}
}

// archived archives the wallet w1.
func archived(t *testing.T, w *world, _ http.Handler, _ *client) {
	wl, err := w.Wallets.ByID(context.Background(), "h1", "w1")
	if err != nil {
		t.Fatal(err)
	}
	wl.ArchivedAt = w.Clock.Now()
	w.must(w.Wallets.Update(context.Background(), wl))
}

const subscriptionBody = `{"name":"X","vendor":"other","price":{"amount":"1.00","currency":"PLN"},"cycle":{"unit":"month","anchor":"2026-01-01"}}`

const badLogin = `{"email":"owner@example.com","password":"not the password"}`

// conformanceCases covers, for every operation, every response the
// contract declares. The 401 of an operation that needs credentials is
// added by the test itself.
var conformanceCases = map[string][]conformanceCase{
	"listNotifications": {
		{status: 200, as: "member", target: "/api/me/notifications?household_id=h1&unread=true&limit=1"},
		{status: 200, as: "bot", target: "/api/me/notifications", setup: botScopes(bot.ScopeNotificationsRead)},
		{status: 403, as: "bot", target: "/api/me/notifications"},
		{status: 404, as: "outsider", target: "/api/me/notifications?household_id=h1"},
		{status: 422, as: "member", target: "/api/me/notifications?limit=0"},
	},
	"countUnreadNotifications": {
		{status: 200, as: "member", target: "/api/me/notifications/unread-count?household_id=h1"},
		{status: 403, as: "bot", target: "/api/me/notifications/unread-count"},
		{status: 404, as: "bot", target: "/api/me/notifications/unread-count?household_id=h2", setup: botScopes(bot.ScopeNotificationsRead)},
	},
	"markAllNotificationsRead": {
		{status: 200, as: "member", target: "/api/me/notifications/read-all", body: `{"household_id":"h1"}`},
		{status: 403, as: "bot", target: "/api/me/notifications/read-all", body: `{}`},
		{status: 404, as: "outsider", target: "/api/me/notifications/read-all", body: `{"household_id":"h1"}`},
	},
	"markNotificationRead": {
		{status: 200, as: "member", target: "/api/me/notifications/n1/read"},
		{status: 403, as: "bot", target: "/api/me/notifications/n1/read"},
		{status: 404, as: "owner", target: "/api/me/notifications/n1/read"},
	},
	"listChannelTypes": {
		{status: 200, as: "viewer", target: "/api/me/channel-types"},
		{status: 403, as: "bot", target: "/api/me/channel-types"},
	},
	"listChannels": {
		{status: 200, as: "member", target: "/api/me/channels"},
		{status: 403, as: "bot", target: "/api/me/channels"},
	},
	"createChannel": {
		{status: 201, as: "member", target: "/api/me/channels", body: `{"type":"ntfy","name":"Telefon","config":{"topic":"denar-dom"},"secrets":{"token":"tk_x"}}`},
		{status: 403, as: "bot", target: "/api/me/channels", body: `{"type":"ntfy","name":"Telefon","config":{"topic":"denar-dom"}}`},
		{status: 409, as: "member", target: "/api/me/channels", body: `{"type":"email","name":"Poczta"}`},
		{status: 422, as: "member", target: "/api/me/channels", body: `{"type":"webhook","name":"X","config":{"url":"ftp://x"}}`},
	},
	"getChannel": {
		{status: 200, as: "member", target: "/api/me/channels/ch1"},
		{status: 403, as: "bot", target: "/api/me/channels/ch1"},
		{status: 404, as: "owner", target: "/api/me/channels/ch1"},
	},
	"updateChannel": {
		{status: 200, as: "member", target: "/api/me/channels/ch1", body: `{"name":"Nowy","secrets":null,"enabled":false,"config":{"url":"https://x.example.com"}}`},
		{status: 403, as: "bot", target: "/api/me/channels/ch1", body: `{"name":"X"}`},
		{status: 404, as: "owner", target: "/api/me/channels/ch1", body: `{"name":"X"}`},
		{status: 409, as: "member", target: "/api/me/channels/ch1", body: `{"name":"Telefon"}`, setup: secondChannel},
		{status: 422, as: "member", target: "/api/me/channels/ch1", body: `{"name":""}`},
	},
	"deleteChannel": {
		{status: 204, as: "member", target: "/api/me/channels/ch1"},
		{status: 403, as: "bot", target: "/api/me/channels/ch1"},
		{status: 404, as: "owner", target: "/api/me/channels/ch1"},
	},
	"testChannel": {
		{status: 200, as: "member", target: "/api/me/channels/ch1/test"},
		{status: 403, as: "bot", target: "/api/me/channels/ch1/test"},
		{status: 404, as: "owner", target: "/api/me/channels/ch1/test"},
	},
	"getNotificationPreferences": {
		{status: 200, as: "viewer", target: "/api/me/households/h1/notification-preferences"},
		{status: 403, as: "bot", target: "/api/me/households/h1/notification-preferences"},
		{status: 404, as: "outsider", target: "/api/me/households/h1/notification-preferences"},
	},
	"setNotificationPreferences": {
		{status: 200, as: "member", target: "/api/me/households/h1/notification-preferences",
			body: `{"items":[{"type":"envelope.overspent","in_app":false,"channel_ids":["ch1"]}]}`},
		{status: 403, as: "bot", target: "/api/me/households/h1/notification-preferences", body: `{"items":[]}`},
		{status: 404, as: "outsider", target: "/api/me/households/h1/notification-preferences", body: `{"items":[]}`},
		{status: 422, as: "owner", target: "/api/me/households/h1/notification-preferences",
			body: `{"items":[{"type":"envelope.overspent","in_app":true,"channel_ids":["ch1"]}]}`},
	},
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
		{status: 409, as: "owner", target: "/api/households/h1/invitations", body: `{"email":"grace@example.com","role":"member","send_email":true}`},
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
	"listWallets": {
		{status: 200, as: "viewer", target: "/api/households/h1/wallets?include_archived=true"},
		{status: 200, as: "bot", target: "/api/households/h1/wallets"},
		{status: 403, as: "bot", target: "/api/households/h1/wallets", setup: botScopes(bot.ScopeReportsRead)},
		{status: 404, as: "outsider", target: "/api/households/h1/wallets"},
	},
	"createWallet": {
		{status: 201, as: "member", target: "/api/households/h1/wallets",
			body: `{"name":"Karta","type":"card","currency":"EUR","on_budget":false,"visibility":"private","member_ids":["viewer"],"opening_balance":{"amount":"-120.50","currency":"EUR"},"opened_on":"2026-10-01"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/wallets", body: `{"name":"X","type":"cash","currency":"PLN"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/wallets", body: `{"name":"X","type":"cash","currency":"PLN"}`},
		{status: 409, as: "member", target: "/api/households/h1/wallets", body: `{"name":"konto","type":"cash","currency":"PLN"}`},
		{status: 422, as: "member", target: "/api/households/h1/wallets", body: `{"name":"X","type":"piggy","currency":"PLN","opening_balance":{"amount":"1.234","currency":"PLN"}}`},
	},
	"getWallet": {
		{status: 200, as: "viewer", target: "/api/households/h1/wallets/w1"},
		{status: 200, as: "owner", target: "/api/households/h1/wallets/w2"},
		{status: 403, as: "bot", target: "/api/households/h1/wallets/w1", setup: botScopes(bot.ScopeReportsRead)},
		{status: 404, as: "member", target: "/api/households/h1/wallets/w2"},
	},
	"updateWallet": {
		{status: 200, as: "member", target: "/api/households/h1/wallets/w1", body: `{"name":"Konto główne","archived":true,"on_budget":false,"type":"savings"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/wallets/w1", body: `{"name":"X"}`},
		{status: 404, as: "member", target: "/api/households/h1/wallets/w2", body: `{"name":"X"}`},
		{status: 409, as: "owner", target: "/api/households/h1/wallets/w1", body: `{"name":"Dolary"}`},
		{status: 422, as: "owner", target: "/api/households/h1/wallets/w1", body: `{"type":"piggy"}`},
	},
	"searchTransactions": {
		{status: 200, as: "viewer", target: "/api/households/h1/transactions?wallet_id=w1&from=2026-10-01&to=2026-10-31&q=zak&tag=food&kind=expense&status=cleared&payee_id=p1&envelope_id=e1&limit=1"},
		{status: 200, as: "owner", target: "/api/households/h1/transactions"},
		{status: 403, as: "bot", target: "/api/households/h1/transactions"},
		{status: 404, as: "member", target: "/api/households/h1/transactions?wallet_id=w2"},
		{status: 422, as: "member", target: "/api/households/h1/transactions?kind=gift"},
		{status: 422, as: "member", target: "/api/households/h1/transactions?from=yesterday"},
	},
	"createTransaction": {
		{status: 201, as: "member", target: "/api/households/h1/transactions",
			body: `{"wallet_id":"w1","date":"2026-10-20","amount":{"amount":"-30.00","currency":"PLN"},"payee_name":"Biedronka","tags":["food"],"note":"x","external_id":"ext-2","status":"pending","splits":[{"envelope_id":"e1","amount":{"amount":"-10.00","currency":"PLN"}},{"amount":{"amount":"-20.00","currency":"PLN"},"note":"reszta"}]}`},
		{status: 201, as: "owner", target: "/api/households/h1/transactions",
			body: `{"wallet_id":"w2","date":"2026-10-20","amount":{"amount":"-10.00","currency":"USD"},"kind":"income"}`},
		{status: 201, as: "bot", target: "/api/households/h1/transactions", setup: botScopes(bot.ScopeTransactionsWrite),
			body: `{"wallet_id":"w1","amount":{"amount":"-1","currency":"PLN"}}`},
		{status: 403, as: "viewer", target: "/api/households/h1/transactions", body: `{"wallet_id":"w1","amount":{"amount":"-1","currency":"PLN"}}`},
		{status: 404, as: "member", target: "/api/households/h1/transactions", body: `{"wallet_id":"w2","amount":{"amount":"-1","currency":"USD"}}`},
		{status: 409, as: "member", target: "/api/households/h1/transactions", body: `{"wallet_id":"w1","amount":{"amount":"-1","currency":"PLN"},"external_id":"ext-1"}`},
		{status: 409, as: "member", target: "/api/households/h1/transactions", body: `{"wallet_id":"w1","amount":{"amount":"-1","currency":"PLN"}}`, setup: archived},
		{status: 422, as: "member", target: "/api/households/h1/transactions", body: `{"wallet_id":"w1","amount":{"amount":"-1","currency":"USD"}}`},
	},
	"getTransaction": {
		{status: 200, as: "viewer", target: "/api/households/h1/transactions/t1"},
		{status: 403, as: "bot", target: "/api/households/h1/transactions/t1"},
		{status: 404, as: "member", target: "/api/households/h1/transactions/nope"},
	},
	"updateTransaction": {
		{status: 200, as: "member", target: "/api/households/h1/transactions/t1",
			body: `{"date":"2026-10-19","amount":{"amount":"-5","currency":"PLN"},"payee_id":null,"envelope_id":null,"kind":"expense","status":"reconciled","tags":[],"note":"","external_id":null,"splits":[]}`},
		{status: 200, as: "member", target: "/api/households/h1/transactions/t1", body: `{"payee_name":"Żabka"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/transactions/t1", body: `{"note":"x"}`},
		{status: 404, as: "member", target: "/api/households/h1/transactions/nope", body: `{"note":"x"}`},
		{status: 409, as: "member", target: "/api/households/h1/transactions/t1", body: `{"note":"x"}`, setup: archived},
		{status: 422, as: "member", target: "/api/households/h1/transactions/t1", body: `{"kind":"transfer"}`},
	},
	"deleteTransaction": {
		{status: 204, as: "member", target: "/api/households/h1/transactions/t1"},
		{status: 403, as: "viewer", target: "/api/households/h1/transactions/t1"},
		{status: 404, as: "member", target: "/api/households/h1/transactions/nope"},
		{status: 409, as: "member", target: "/api/households/h1/transactions/t1", setup: archived},
	},
	"createTransfer": {
		{status: 201, as: "owner", target: "/api/households/h1/transfers",
			body: `{"from_wallet_id":"w1","to_wallet_id":"w2","date":"2026-10-20","amount":{"amount":"400.00","currency":"PLN"},"to_amount":{"amount":"100.00","currency":"USD"},"note":"wymiana","tags":["fx"],"status":"cleared"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/transfers", body: `{"from_wallet_id":"w1","to_wallet_id":"w2","amount":{"amount":"1","currency":"PLN"}}`},
		{status: 404, as: "member", target: "/api/households/h1/transfers", body: `{"from_wallet_id":"w1","to_wallet_id":"w2","amount":{"amount":"1","currency":"PLN"}}`},
		{status: 409, as: "owner", target: "/api/households/h1/transfers", body: `{"from_wallet_id":"w1","to_wallet_id":"w2","amount":{"amount":"1","currency":"PLN"},"to_amount":{"amount":"1","currency":"USD"}}`, setup: archived},
		{status: 422, as: "owner", target: "/api/households/h1/transfers", body: `{"from_wallet_id":"w1","to_wallet_id":"w2","amount":{"amount":"1","currency":"PLN"}}`},
	},
	"listPayees": {
		{status: 200, as: "viewer", target: "/api/households/h1/payees"},
		{status: 403, as: "bot", target: "/api/households/h1/payees"},
		{status: 404, as: "outsider", target: "/api/households/h1/payees"},
	},
	"createPayee": {
		{status: 201, as: "member", target: "/api/households/h1/payees", body: `{"name":"Biedronka","patterns":["JMP S.A."],"default_envelope_id":"groceries"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/payees", body: `{"name":"X"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/payees", body: `{"name":"X"}`},
		{status: 409, as: "member", target: "/api/households/h1/payees", body: `{"name":"lidl"}`},
		{status: 422, as: "member", target: "/api/households/h1/payees", body: `{"name":" "}`},
	},
	"updatePayee": {
		{status: 200, as: "member", target: "/api/households/h1/payees/p1", body: `{"name":"Lidl PL","patterns":[],"default_envelope_id":null}`},
		{status: 403, as: "viewer", target: "/api/households/h1/payees/p1", body: `{"name":"X"}`},
		{status: 404, as: "member", target: "/api/households/h1/payees/nope", body: `{"name":"X"}`},
		{status: 409, as: "member", target: "/api/households/h1/payees/p1", body: `{"name":"ALDI"}`},
		{status: 422, as: "member", target: "/api/households/h1/payees/p1", body: `{"name":""}`},
	},
	"deletePayee": {
		{status: 204, as: "member", target: "/api/households/h1/payees/p1"},
		{status: 403, as: "viewer", target: "/api/households/h1/payees/p1"},
		{status: 404, as: "member", target: "/api/households/h1/payees/nope"},
	},
	"quoteFxRate": {
		{status: 200, as: "viewer", target: "/api/households/h1/fx-rates?currency=USD&date=2026-10-20"},
		{status: 200, as: "viewer", target: "/api/households/h1/fx-rates?currency=EUR"},
		{status: 403, as: "bot", target: "/api/households/h1/fx-rates?currency=USD"},
		{status: 404, as: "outsider", target: "/api/households/h1/fx-rates?currency=USD"},
		{status: 422, as: "viewer", target: "/api/households/h1/fx-rates?currency=XYZ"},
		{status: 422, as: "viewer", target: "/api/households/h1/fx-rates"},
	},
	"listFxOverrides": {
		{status: 200, as: "viewer", target: "/api/households/h1/fx-rates/overrides"},
		{status: 403, as: "bot", target: "/api/households/h1/fx-rates/overrides"},
		{status: 404, as: "outsider", target: "/api/households/h1/fx-rates/overrides"},
	},
	"setFxOverride": {
		{status: 200, as: "admin", target: "/api/households/h1/fx-rates/overrides", body: `{"currency":"USD","date":"2026-10-19","rate":"4.2512"}`},
		{status: 403, as: "member", target: "/api/households/h1/fx-rates/overrides", body: `{"currency":"USD","date":"2026-10-19","rate":"4"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/fx-rates/overrides", body: `{"currency":"USD","date":"2026-10-19","rate":"4"}`},
		{status: 422, as: "admin", target: "/api/households/h1/fx-rates/overrides", body: `{"currency":"USD","date":"2026-10-19","rate":"-4"}`},
	},
	"getBudgetMonth": {
		{status: 200, as: "viewer", target: "/api/households/h1/budget/months/2026-10"},
		{status: 200, as: "bot", target: "/api/households/h1/budget/months/2026-11", setup: botScopes(bot.ScopeBudgetRead)},
		{status: 403, as: "bot", target: "/api/households/h1/budget/months/2026-10"},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/months/2026-10"},
		{status: 422, as: "member", target: "/api/households/h1/budget/months/2026-13"},
	},
	"listEnvelopeActivity": {
		{status: 200, as: "viewer", target: "/api/households/h1/budget/months/2026-10/envelopes/e1/transactions?limit=1"},
		{status: 403, as: "bot", target: "/api/households/h1/budget/months/2026-10/envelopes/e1/transactions", setup: botScopes(bot.ScopeBudgetRead)},
		{status: 404, as: "member", target: "/api/households/h1/budget/months/2026-10/envelopes/nope/transactions"},
		{status: 422, as: "member", target: "/api/households/h1/budget/months/2026-10/envelopes/e1/transactions?cursor=garbage"},
	},
	"setAssignment": {
		{status: 200, as: "member", target: "/api/households/h1/budget/months/2026-10/assignments/e1", body: `{"amount":{"amount":"100.00","currency":"PLN"}}`},
		{status: 200, as: "bot", target: "/api/households/h1/budget/months/2026-10/assignments/e1", body: `{"amount":{"amount":"0","currency":"PLN"}}`,
			setup: botScopes(bot.ScopeBudgetWrite)},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/months/2026-10/assignments/e1", body: `{"amount":{"amount":"1","currency":"PLN"}}`},
		{status: 404, as: "member", target: "/api/households/h1/budget/months/2026-10/assignments/nope", body: `{"amount":{"amount":"1","currency":"PLN"}}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/months/2026-10/assignments/e1", body: `{"amount":{"amount":"1","currency":"USD"}}`},
	},
	"moveBetweenEnvelopes": {
		{status: 200, as: "member", target: "/api/households/h1/budget/months/2026-10/moves",
			body: `{"from_envelope_id":"e1","to_envelope_id":"groceries","amount":{"amount":"10.00","currency":"PLN"}}`},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/months/2026-10/moves",
			body: `{"from_envelope_id":"e1","to_envelope_id":"groceries","amount":{"amount":"1","currency":"PLN"}}`},
		{status: 404, as: "member", target: "/api/households/h1/budget/months/2026-10/moves",
			body: `{"from_envelope_id":"e1","to_envelope_id":"nope","amount":{"amount":"1","currency":"PLN"}}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/months/2026-10/moves",
			body: `{"from_envelope_id":"e1","to_envelope_id":"e1","amount":{"amount":"1","currency":"PLN"}}`},
	},
	"autoAssign": {
		{status: 200, as: "member", target: "/api/households/h1/budget/months/2026-10/auto-assign", setup: income},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/months/2026-10/auto-assign"},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/months/2026-10/auto-assign"},
		{status: 422, as: "member", target: "/api/households/h1/budget/months/2026-13/auto-assign"},
	},
	"listEnvelopeGroups": {
		{status: 200, as: "viewer", target: "/api/households/h1/budget/envelope-groups"},
		{status: 403, as: "bot", target: "/api/households/h1/budget/envelope-groups"},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/envelope-groups"},
	},
	"createEnvelopeGroup": {
		{status: 201, as: "member", target: "/api/households/h1/budget/envelope-groups", body: `{"name":"Rachunki","position":3}`},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelope-groups", body: `{"name":"X"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/envelope-groups", body: `{"name":"X"}`},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelope-groups", body: `{"name":"codzienne"}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/envelope-groups", body: `{"name":" "}`},
	},
	"updateEnvelopeGroup": {
		{status: 200, as: "member", target: "/api/households/h1/budget/envelope-groups/g1", body: `{"name":"Stałe","position":2}`},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelope-groups/g1", body: `{"name":"X"}`},
		{status: 404, as: "member", target: "/api/households/h1/budget/envelope-groups/nope", body: `{"name":"X"}`},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelope-groups/g1", body: `{"name":"CELE"}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/envelope-groups/g1", body: `{"position":-1}`},
	},
	"deleteEnvelopeGroup": {
		{status: 204, as: "member", target: "/api/households/h1/budget/envelope-groups/g2"},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelope-groups/g2"},
		{status: 404, as: "member", target: "/api/households/h1/budget/envelope-groups/nope"},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelope-groups/g1"},
	},
	"listEnvelopes": {
		{status: 200, as: "viewer", target: "/api/households/h1/budget/envelopes?include_hidden=true"},
		{status: 200, as: "bot", target: "/api/households/h1/budget/envelopes", setup: botScopes(bot.ScopeBudgetRead)},
		{status: 403, as: "bot", target: "/api/households/h1/budget/envelopes"},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/envelopes"},
	},
	"createEnvelope": {
		{status: 201, as: "member", target: "/api/households/h1/budget/envelopes",
			body: `{"group_id":"g2","name":"Wakacje","position":1,"hidden":false,"goal":{"type":"target_by_date","amount":{"amount":"1200.00","currency":"PLN"},"target_date":"2027-03-15"}}`},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelopes", body: `{"group_id":"g1","name":"X"}`},
		{status: 404, as: "outsider", target: "/api/households/h1/budget/envelopes", body: `{"group_id":"g1","name":"X"}`},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelopes", body: `{"group_id":"g2","name":"jedzenie"}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/envelopes", body: `{"group_id":"nope","name":"X"}`},
	},
	"getEnvelope": {
		{status: 200, as: "viewer", target: "/api/households/h1/budget/envelopes/e1"},
		{status: 403, as: "bot", target: "/api/households/h1/budget/envelopes/e1"},
		{status: 404, as: "member", target: "/api/households/h1/budget/envelopes/nope"},
	},
	"updateEnvelope": {
		{status: 200, as: "member", target: "/api/households/h1/budget/envelopes/e1", body: `{"goal":null,"hidden":true,"group_id":"g2","name":"Jedzenie i chemia","position":4}`},
		{status: 200, as: "member", target: "/api/households/h1/budget/envelopes/e2", body: `{"goal":{"type":"monthly","amount":{"amount":"50","currency":"PLN"}}}`},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelopes/e1", body: `{"hidden":true}`},
		{status: 404, as: "member", target: "/api/households/h1/budget/envelopes/nope", body: `{"hidden":true}`},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelopes/e1", body: `{"name":"ZAKUPY"}`},
		{status: 422, as: "member", target: "/api/households/h1/budget/envelopes/e1", body: `{"goal":{"type":"monthly","amount":{"amount":"5","currency":"USD"}}}`},
	},
	"deleteEnvelope": {
		{status: 204, as: "member", target: "/api/households/h1/budget/envelopes/e2"},
		{status: 403, as: "viewer", target: "/api/households/h1/budget/envelopes/e2"},
		{status: 404, as: "member", target: "/api/households/h1/budget/envelopes/nope"},
		{status: 409, as: "member", target: "/api/households/h1/budget/envelopes/e1"},
	},
	"listVendors": {{status: 200, as: "viewer", target: "/api/vendors"}, {status: 200, as: "bot", target: "/api/vendors"}},
	"listSubscriptions": {
		{status: 200, as: "viewer", target: "/api/households/h1/subscriptions?vendor=openai&status=active"},
		{status: 200, as: "bot", target: "/api/households/h1/subscriptions", setup: botScopes(bot.ScopeSubscriptionsRead)},
		{status: 403, as: "bot", target: "/api/households/h1/subscriptions"},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions?wallet_id=w2"},
		{status: 422, as: "member", target: "/api/households/h1/subscriptions?status=gone"},
	},
	"createSubscription": {
		{status: 201, as: "member", target: "/api/households/h1/subscriptions", body: `{"name":"Claude Pro","vendor":"anthropic",
			"external_ref":"sub_1","price":{"amount":"20.00","currency":"USD"},"cycle":{"unit":"month","every":1,"anchor":"2026-11-05"},
			"trial_ends_at":"2026-11-05","wallet_id":"w1","envelope_id":"groceries","reminder_offsets":[7,1],
			"match_rule":{"payee_pattern":"ANTHROPIC","amount_tolerance_pct":5,"date_window_days":2},"note":"n",
			"manage_url":"https://claude.ai/settings"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/subscriptions", body: subscriptionBody},
		{status: 404, as: "outsider", target: "/api/households/h1/subscriptions", body: subscriptionBody},
		{status: 409, as: "member", target: "/api/households/h1/subscriptions", body: `{"name":"chatgpt plus","vendor":"openai",
			"price":{"amount":"20.00","currency":"USD"},"cycle":{"unit":"month","anchor":"2026-01-31"}}`},
		{status: 422, as: "member", target: "/api/households/h1/subscriptions", body: `{"name":"","vendor":"myspace",
			"price":{"amount":"0","currency":"USD"},"cycle":{"unit":"fortnight","anchor":"2026-01-31"}}`},
	},
	"getSubscriptionSummary": {
		{status: 200, as: "viewer", target: "/api/households/h1/subscriptions/summary"},
		{status: 403, as: "bot", target: "/api/households/h1/subscriptions/summary"},
		{status: 404, as: "outsider", target: "/api/households/h1/subscriptions/summary"},
	},
	"listUpcomingRenewals": {
		{status: 200, as: "viewer", target: "/api/households/h1/subscriptions/upcoming?days=30"},
		{status: 403, as: "bot", target: "/api/households/h1/subscriptions/upcoming"},
		{status: 404, as: "outsider", target: "/api/households/h1/subscriptions/upcoming"},
		{status: 422, as: "viewer", target: "/api/households/h1/subscriptions/upcoming?days=0"},
	},
	"getSubscription": {
		{status: 200, as: "viewer", target: "/api/households/h1/subscriptions/s1"},
		{status: 403, as: "bot", target: "/api/households/h1/subscriptions/s1"},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions/s2"},
	},
	"updateSubscription": {
		{status: 200, as: "member", target: "/api/households/h1/subscriptions/s1", body: `{"name":"ChatGPT","status":"cancelled",
			"ends_at":"2026-12-01","envelope_id":null,"wallet_id":"w1","cycle":{"unit":"month","anchor":"2026-01-31"},
			"match_rule":{"date_window_days":5},"reminder_offsets":[],"manage_url":null,"external_ref":null,"trial_ends_at":null}`},
		{status: 403, as: "viewer", target: "/api/households/h1/subscriptions/s1", body: `{"note":"x"}`},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions/s2", body: `{"note":"x"}`},
		{status: 409, as: "member", target: "/api/households/h1/subscriptions/s1", body: `{"name":"netflix"}`},
		{status: 422, as: "member", target: "/api/households/h1/subscriptions/s1", body: `{"status":"gone"}`},
	},
	"deleteSubscription": {
		{status: 204, as: "member", target: "/api/households/h1/subscriptions/s1"},
		{status: 403, as: "viewer", target: "/api/households/h1/subscriptions/s1"},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions/s2"},
	},
	"listSubscriptionPrices": {
		{status: 200, as: "viewer", target: "/api/households/h1/subscriptions/s1/prices"},
		{status: 403, as: "bot", target: "/api/households/h1/subscriptions/s1/prices"},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions/s2/prices"},
	},
	"changeSubscriptionPrice": {
		{status: 201, as: "member", target: "/api/households/h1/subscriptions/s1/prices", body: `{"effective_from":"2026-12-01","price":{"amount":"25.00","currency":"USD"}}`},
		{status: 403, as: "viewer", target: "/api/households/h1/subscriptions/s1/prices", body: `{"price":{"amount":"25.00","currency":"USD"}}`},
		{status: 404, as: "member", target: "/api/households/h1/subscriptions/s2/prices", body: `{"price":{"amount":"25.00","currency":"USD"}}`},
		{status: 422, as: "member", target: "/api/households/h1/subscriptions/s1/prices", body: `{"price":{"amount":"2.555","currency":"USD"}}`},
	},
	"listCharges": {
		{status: 200, as: "viewer", target: "/api/households/h1/charges?status=expected&status=matched&from=2026-10-01&to=2026-12-31&limit=1"},
		{status: 403, as: "bot", target: "/api/households/h1/charges"},
		{status: 404, as: "member", target: "/api/households/h1/charges?subscription_id=s2"},
		{status: 422, as: "member", target: "/api/households/h1/charges?cursor=garbage"},
	},
	"getCharge": {
		{status: 200, as: "viewer", target: "/api/households/h1/charges/c1"},
		{status: 403, as: "bot", target: "/api/households/h1/charges/c1"},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9"},
	},
	"suggestChargeMatches": {
		{status: 200, as: "member", target: "/api/households/h1/charges/c2/suggestions?limit=3"},
		{status: 403, as: "bot", target: "/api/households/h1/charges/c2/suggestions", setup: botScopes(bot.ScopeSubscriptionsRead)},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9/suggestions"},
		{status: 422, as: "member", target: "/api/households/h1/charges/c2/suggestions?limit=99"},
	},
	"matchCharge": {
		{status: 200, as: "member", target: "/api/households/h1/charges/c2/match", body: `{"transaction_id":"t2"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/charges/c2/match", body: `{"transaction_id":"t2"}`},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9/match", body: `{"transaction_id":"t2"}`},
		{status: 409, as: "member", target: "/api/households/h1/charges/c2/match", body: `{"transaction_id":"t1"}`},
		{status: 422, as: "member", target: "/api/households/h1/charges/c2/match", body: `{"transaction_id":"nope"}`},
	},
	"unmatchCharge": {
		{status: 200, as: "member", target: "/api/households/h1/charges/c1/unmatch"},
		{status: 403, as: "viewer", target: "/api/households/h1/charges/c1/unmatch"},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9/unmatch"},
		{status: 409, as: "member", target: "/api/households/h1/charges/c2/unmatch"},
	},
	"postCharge": {
		{status: 201, as: "member", target: "/api/households/h1/charges/c2/post", body: `{"amount":{"amount":"80.00","currency":"PLN"},"payee_name":"OpenAI","note":"karta"}`},
		{status: 403, as: "viewer", target: "/api/households/h1/charges/c2/post"},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9/post"},
		{status: 409, as: "member", target: "/api/households/h1/charges/c1/post"},
		{status: 422, as: "member", target: "/api/households/h1/charges/c2/post"},
	},
	"skipCharge": {
		{status: 200, as: "member", target: "/api/households/h1/charges/c2/skip"},
		{status: 403, as: "viewer", target: "/api/households/h1/charges/c2/skip"},
		{status: 404, as: "member", target: "/api/households/h1/charges/c9/skip"},
		{status: 409, as: "member", target: "/api/households/h1/charges/c1/skip"},
	},
	"suggestChargesForTransaction": {
		{status: 200, as: "member", target: "/api/households/h1/transactions/t2/charge-suggestions"},
		{status: 403, as: "bot", target: "/api/households/h1/transactions/t2/charge-suggestions", setup: botScopes(bot.ScopeSubscriptionsRead)},
		{status: 404, as: "member", target: "/api/households/h1/transactions/nope/charge-suggestions"},
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
