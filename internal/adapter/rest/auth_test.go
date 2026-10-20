package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestCookieAndBearerTogetherIsABadRequest(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	c := w.as(h, "owner")
	c.bearer = botToken("b1")
	rec := c.do("GET", "/api/me", "")
	if rec.Code != 400 {
		t.Fatalf("status = %d", rec.Code)
	}
	wantProblemShape(t, rec)
}

func TestBotCallsRESTWithBearer(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	rec := w.as(h, "bot").do("GET", "/api/me", "")
	if rec.Code != 200 {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
	var me struct {
		Kind string `json:"kind"`
		Bot  struct {
			ID              string   `json:"id"`
			EffectiveScopes []string `json:"effective_scopes"`
		} `json:"bot"`
	}
	json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Kind != "bot" || me.Bot.ID != "b1" || len(me.Bot.EffectiveScopes) != 1 {
		t.Errorf("me = %+v", me)
	}

	// The creator drops to viewer: still reads; loses write scopes at once.
	_, err := w.deps.UpdateMemberRole.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "owner")}, "h1", "member", membership.RoleViewer)
	w.must(err)
	rec = w.as(h, "bot").do("GET", "/api/households/h1", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"role":"viewer"`) {
		t.Errorf("after demotion = %d %s", rec.Code, rec.Body)
	}

	// Administration stays closed.
	rec = w.as(h, "bot").do("POST", "/api/households/h1/bots", `{"name":"x","scopes":["wallets:read"]}`)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "human-only") {
		t.Errorf("bot creates bot = %d %s", rec.Code, rec.Body)
	}

	// Removing the creator disables the bot: its token stops working.
	w.must(w.deps.RemoveMember.Do(context.Background(), usecase.Principal{User: mustUser(t, w, "owner")}, "h1", "member"))
	if rec := w.as(h, "bot").do("GET", "/api/me", ""); rec.Code != 401 {
		t.Errorf("bot of removed member = %d", rec.Code)
	}
}

func TestBearerLimits(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	bad := &client{handler: h, bearer: "dnr_wrong"}
	for i := range bearerFailuresPerAddress {
		if rec := bad.do("GET", "/api/me", ""); rec.Code != 401 {
			t.Fatalf("attempt %d = %d", i, rec.Code)
		}
	}
	rec := bad.do("GET", "/api/me", "")
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Errorf("after the limit = %d %v", rec.Code, rec.Header())
	}
	// A malformed Authorization header counts as a bot attempt too.
	if rec := (&client{handler: w.handler()}).do("GET", "/api/me", "", "Authorization", "Basic abc"); rec.Code != 401 {
		t.Errorf("basic auth = %d", rec.Code)
	}

	w2 := newWorld(t)
	w2.deps.BotRate = 10
	h2 := w2.handler()
	for i := range 10 {
		if rec := w2.as(h2, "bot").do("GET", "/api/me", ""); rec.Code != 200 {
			t.Fatalf("request %d = %d", i, rec.Code)
		}
	}
	if rec := w2.as(h2, "bot").do("GET", "/api/me", ""); rec.Code != 429 {
		t.Errorf("over the bot rate = %d", rec.Code)
	}
}

func TestLoginLimitCountsFailuresOnly(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	c := &client{handler: h}
	good := `{"email":"owner@example.com","password":"password-of-owner"}`
	for range 10 {
		if rec := c.do("POST", "/api/auth/login", good); rec.Code != 200 {
			t.Fatalf("good login = %d", rec.Code)
		}
	}
	for range loginFailuresPerAccount {
		c.do("POST", "/api/auth/login", badLogin)
	}
	if rec := c.do("POST", "/api/auth/login", good); rec.Code != 429 {
		t.Errorf("after failures = %d", rec.Code)
	}
	// Another account from the same address is still allowed.
	if rec := c.do("POST", "/api/auth/login", `{"email":"admin@example.com","password":"password-of-admin"}`); rec.Code != 200 {
		t.Errorf("other account = %d", rec.Code)
	}
}

func TestSessionCookie(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	rec := (&client{handler: h}).do("POST", "/api/auth/login", `{"email":"owner@example.com","password":"password-of-owner"}`)
	c := cookie(rec, sessionCookieName)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie = %+v", c)
	}
	if strings.Contains(w.logs.String(), c.Value) {
		t.Error("session token in the log")
	}
	in := &client{handler: h, cookies: []*http.Cookie{{Name: sessionCookieName, Value: c.Value}}}
	if rec := in.do("GET", "/api/me", ""); rec.Code != 200 {
		t.Errorf("me = %d", rec.Code)
	}
	out := in.do("POST", "/api/auth/logout", "")
	if gone := cookie(out, sessionCookieName); gone == nil || gone.MaxAge >= 0 {
		t.Errorf("logout cookie = %+v", gone)
	}
	if rec := in.do("GET", "/api/me", ""); rec.Code != 401 {
		t.Errorf("after logout = %d", rec.Code)
	}
}

func TestSecretsAreNotCached(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	for _, call := range []struct{ method, target, body string }{
		{"POST", "/api/households/h1/bots", `{"name":"Claude","scopes":["wallets:read"]}`},
		{"POST", "/api/households/h1/bots/b1/token", ""},
	} {
		rec := w.as(h, "member").do(call.method, call.target, call.body)
		if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"token":"dnr_`) {
			t.Errorf("%s = %d %v %s", call.target, rec.Code, rec.Header(), rec.Body)
		}
	}
	rec := w.as(h, "owner").do("POST", "/api/households/h1/invitations", `{"email":"g@example.com","role":"viewer"}`)
	var created struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if rec.Header().Get("Cache-Control") != "no-store" || created.URL != baseURL+"/invite#"+created.Token {
		t.Errorf("invitation = %v %s", rec.Header(), rec.Body)
	}
	// Reads give the prefix only.
	rec = w.as(h, "member").do("GET", "/api/households/h1/bots", "")
	if strings.Contains(rec.Body.String(), `"token"`) || strings.Contains(rec.Body.String(), "hash") {
		t.Errorf("list leaks: %s", rec.Body)
	}
}

func TestCrossOriginWritesAreRefused(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	for _, op := range specOperations(t) {
		if op.method == "GET" {
			continue
		}
		c := conformanceCases[op.id][0]
		rec := w.as(h, c.as).do(op.method, c.target, c.body, "Sec-Fetch-Site", "cross-site")
		if rec.Code != 403 {
			t.Errorf("%s from another site = %d, want 403", op.id, rec.Code)
			continue
		}
		wantProblemShape(t, rec)
	}
	// The instance's own origin passes.
	rec := w.as(h, "member").do("POST", "/api/households", `{"name":"D","base_currency":"PLN","timezone":"UTC","month_start_day":1}`,
		"Origin", baseURL, "Sec-Fetch-Site", "same-origin")
	if rec.Code != 201 {
		t.Errorf("same origin = %d", rec.Code)
	}
}

func TestHealthAndUnknownPaths(t *testing.T) {
	w := newWorld(t)
	if rec := (&client{handler: w.handler()}).do("GET", "/healthz", ""); rec.Code != 200 {
		t.Errorf("healthz = %d", rec.Code)
	}
	w.deps.Health = func(context.Context) error { return errors.New("database is gone") }
	if rec := (&client{handler: w.handler()}).do("GET", "/healthz", ""); rec.Code != 503 {
		t.Errorf("healthz down = %d", rec.Code)
	}
	rec := w.as(w.handler(), "owner").do("GET", "/api/nothing/here", "")
	if rec.Code != 404 {
		t.Errorf("unknown = %d", rec.Code)
	}
	wantProblemShape(t, rec)
	rec = w.as(w.handler(), "owner").do("POST", "/api/households", "name=x", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != 422 {
		t.Errorf("form body = %d", rec.Code)
	}
}

func TestPanicIsAnInternalProblem(t *testing.T) {
	w := newWorld(t)
	w.deps.GetMe = nil
	rec := w.as(w.handler(), "owner").do("GET", "/api/me", "")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Errorf("panic = %d %s", rec.Code, rec.Body)
	}
}
