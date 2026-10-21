package mcp

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain/bot"
)

func TestEveryFailedAuthenticationLooksTheSame(t *testing.T) {
	w := newWorld(t)
	w.addBot("disabled", w.owner, allScopes)
	b, _ := w.Bots.ByID(ctx, "disabled")
	b.Disabled = true
	w.must(w.Bots.Update(ctx, b))
	w.addBot("expired", w.owner, allScopes)
	b, _ = w.Bots.ByID(ctx, "expired")
	b.ExpiresAt = w.now().Add(-time.Minute)
	w.must(w.Bots.Update(ctx, b))
	w.addBot("rotated", w.owner, allScopes)
	b, _ = w.Bots.ByID(ctx, "rotated")
	b.TokenHash = "something else"
	w.must(w.Bots.Update(ctx, b))

	var first string
	for name, edit := range map[string]func(*http.Request){
		"no header":      nil,
		"basic":          func(r *http.Request) { r.Header.Set("Authorization", "Basic YWRhOnB3") },
		"unknown token":  withToken("dnr_nope"),
		"no prefix":      withToken("slg_token"),
		"disabled bot":   withToken(botToken("disabled")),
		"expired bot":    withToken(botToken("expired")),
		"rotated token":  withToken(botToken("rotated")),
		"bearer, no val": func(r *http.Request) { r.Header.Set("Authorization", "Bearer") },
	} {
		res, body := w.raw(http.MethodPost, listTools, edit)
		if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s: %d %q", name, res.StatusCode, res.Header.Get("WWW-Authenticate"))
		}
		if first == "" {
			first = body
		} else if body != first {
			t.Errorf("%s: body %q differs from %q", name, body, first)
		}
	}
	// A token is never logged.
	if strings.Contains(w.log.String(), "token-of") {
		t.Errorf("the log has a token: %s", w.log.String())
	}
}

func TestTooManyWrongTokensFromOneAddress(t *testing.T) {
	w := newWorld(t)
	good := w.addBot("b1", w.owner, allScopes)
	for i := range bearerFailuresPerAddress {
		if res, _ := w.raw(http.MethodPost, listTools, withToken("dnr_wrong")); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, res.StatusCode)
		}
	}
	res, _ := w.raw(http.MethodPost, listTools, withToken("dnr_wrong"))
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Fatalf("over the limit: %d %q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	// The address is blocked as a whole until the window moves on.
	if res, _ := w.raw(http.MethodPost, listTools, withToken(good)); res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("good token while blocked: %d", res.StatusCode)
	}
	w.Clock.Advance(limitWindow + time.Second)
	if res, _ := w.raw(http.MethodPost, listTools, withToken(good)); res.StatusCode != http.StatusOK {
		t.Errorf("after the window: %d", res.StatusCode)
	}
}

func TestBotRate(t *testing.T) {
	w := newWorld(t, func(d *Deps) { d.RatePerMinute = 3 })
	a := w.addBot("a", w.owner, allScopes)
	b := w.addBot("b", w.owner, allScopes)
	for range 3 {
		if res, _ := w.raw(http.MethodPost, listTools, withToken(a)); res.StatusCode != http.StatusOK {
			t.Fatalf("within the rate: %d", res.StatusCode)
		}
	}
	res, _ := w.raw(http.MethodPost, listTools, withToken(a))
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") != "60" {
		t.Fatalf("over the rate: %d %q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	// Each bot has its own rate.
	if res, _ := w.raw(http.MethodPost, listTools, withToken(b)); res.StatusCode != http.StatusOK {
		t.Errorf("other bot: %d", res.StatusCode)
	}
	w.Clock.Advance(limitWindow)
	if res, _ := w.raw(http.MethodPost, listTools, withToken(a)); res.StatusCode != http.StatusOK {
		t.Errorf("after a minute: %d", res.StatusCode)
	}
}

func TestTransport(t *testing.T) {
	w := newWorld(t)
	token := w.addBot("b1", w.owner, allScopes)

	res, body := w.raw(http.MethodPost, listTools, withToken(token))
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") ||
		res.Header.Get("Mcp-Session-Id") != "" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("tools/list: %d %v", res.StatusCode, res.Header)
	}
	if !strings.Contains(body, `"record_expense"`) {
		t.Errorf("tools/list = %s", body)
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodOptions} {
		if res, _ := w.raw(method, "", withToken(token)); res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != "POST" {
			t.Errorf("%s: %d", method, res.StatusCode)
		}
	}
	if res, _ := w.raw(http.MethodPost, " ["+listTools+"]", withToken(token)); res.StatusCode != http.StatusBadRequest {
		t.Errorf("batch: %d", res.StatusCode)
	}
	big := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"x":"` + strings.Repeat("a", maxRequestBody) + `"}}`
	if res, _ := w.raw(http.MethodPost, big, withToken(token)); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("big body: %d", res.StatusCode)
	}
	crossSite := func(r *http.Request) {
		withToken(token)(r)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
	}
	if res, _ := w.raw(http.MethodPost, listTools, crossSite); res.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site: %d", res.StatusCode)
	}
	// One log line per request, with the bot and the route only.
	if log := w.log.String(); !strings.Contains(log, "bot_id=b1") || !strings.Contains(log, "path=/mcp") || strings.Contains(log, token) {
		t.Errorf("log = %s", log)
	}
}

// A bot whose creator left the household keeps its token but no scope.
func TestCreatorWhoLeft(t *testing.T) {
	w := newWorld(t)
	token := w.addBot("b1", w.member, []bot.Scope{bot.ScopeWalletsRead})
	w.must(w.Memberships.Delete(ctx, "h1", w.member.ID))
	s := w.connect(token)
	a := refused(t, s, "list_wallets", nil, codeForbidden)
	if !strings.Contains(a.message(), "wallets:read") {
		t.Errorf("message = %q", a.message())
	}
}
