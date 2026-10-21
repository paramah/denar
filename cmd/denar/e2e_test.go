package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coreos/go-oidc/v3/oidc/oidctest"

	"github.com/paramah/denar/internal/platform/config"
)

const (
	adminEmail    = "admin@example.com"
	adminPassword = "an admin password for e2e"
	oidcClientID  = "denar"
	oidcSecret    = "e2e-client-secret-value"
	oidcCode      = "e2e-authorization-code"
)

// syncBuffer collects the server log while requests are being served.
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

// setEnv points the configuration at a fresh database and the bootstrap
// admin; extra pairs override.
func setEnv(t *testing.T, extra ...string) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "denar.db")
	vars := map[string]string{
		"DENAR_DB": db, "DENAR_ADDR": "127.0.0.1:0", "DENAR_ADMIN_EMAIL": adminEmail, "DENAR_ADMIN_PASSWORD": adminPassword,
		"DENAR_AUTO_MIGRATE": "", "DENAR_BASE_URL": "", "DENAR_OIDC_ISSUER": "", "DENAR_OIDC_CLIENT_ID": "",
		"DENAR_OIDC_CLIENT_SECRET": "", "DENAR_OIDC_AUTO_CREATE": "", "DENAR_LOCAL_LOGIN": "", "DENAR_SESSION_TTL": "",
		"DENAR_INVITE_TTL": "", "DENAR_TRUSTED_PROXIES": "", "DENAR_INSTANCE_NAME": "", "DENAR_BOT_RATE": "",
		"DENAR_ADMIN_PASSWORD_FILE": "", "DENAR_ADMIN_NAME": "",
		// Nothing listens there: tests never reach the real NBP.
		"DENAR_NBP_URL": "http://127.0.0.1:1", "DENAR_FX_INTERVAL": "",
		"DENAR_CHARGE_HORIZON": "", "DENAR_SUBSCRIPTION_INTERVAL": "",
		"DENAR_MASTER_KEY": "", "DENAR_MASTER_KEY_FILE": "", "DENAR_SMTP_HOST": "", "DENAR_SMTP_PORT": "", "DENAR_SMTP_USERNAME": "",
		"DENAR_SMTP_PASSWORD": "", "DENAR_SMTP_PASSWORD_FILE": "", "DENAR_SMTP_FROM": "", "DENAR_SMTP_TLS": "",
		"DENAR_NOTIFY_INTERVAL": "", "DENAR_BUDGET_CHECK_INTERVAL": "", "DENAR_GOAL_DUE_DAYS": "", "DENAR_TELEGRAM_API_URL": "",
		"DENAR_NOTIFY_ALLOW_PRIVATE": "", "DENAR_NOTIFY_ALLOWED_CIDRS": "", "DENAR_DEV_FAKE_CONNECTOR": "",
	}
	for i := 0; i+1 < len(extra); i += 2 {
		vars[extra[i]] = extra[i+1]
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
	return db
}

func mustConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func testPanel() fstest.MapFS {
	return fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Denar test</title>")}}
}

// e2e is a running server on a real socket and a SQLite file.
type e2e struct {
	t      *testing.T
	base   string
	logs   *syncBuffer
	server *http.Server
	app    *app
}

func startE2E(t *testing.T) *e2e {
	t.Helper()
	return startE2EWith(t, appOptions{})
}

// startE2EWith starts the server with parts replaced.
func startE2EWith(t *testing.T, opts appOptions) *e2e {
	t.Helper()
	logs := &syncBuffer{}
	cfg := mustConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a, err := newAppWith(context.Background(), cfg, slog.New(slog.NewJSONHandler(logs, nil)), testPanel(), opts)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	e := &e2e{t: t, base: "http://" + listener.Addr().String(), logs: logs, app: a, server: &http.Server{Handler: a.handler}}
	go e.server.Serve(listener)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		e.server.Shutdown(ctx)
		e.app.Close()
	})
	return e
}

// client is an HTTP client with a cookie jar that does not follow
// redirects, or a bot with a token.
type client struct {
	e      *e2e
	http   *http.Client
	bearer string
}

func (e *e2e) browser() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

func (e *e2e) bot(token string) *client {
	return &client{e: e, http: &http.Client{}, bearer: token}
}

func (c *client) do(method, path, body string, out any) (int, http.Header) {
	c.e.t.Helper()
	req, err := http.NewRequest(method, c.e.base+path, strings.NewReader(body))
	if err != nil {
		c.e.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.e.t.Fatalf("%s %s: body %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode, resp.Header
}

func (c *client) must(status int, method, path, body string, out any) http.Header {
	c.e.t.Helper()
	got, header := c.do(method, path, body, out)
	if got != status {
		c.e.t.Fatalf("%s %s = %d, want %d", method, path, got, status)
	}
	return header
}

type household struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type botWithToken struct {
	Bot struct {
		ID string `json:"id"`
	} `json:"bot"`
	Token string `json:"token"`
}

// TestEndToEnd is the walk of stage 001: the admin signs in, creates a
// household and a bot, the bot calls REST with its token; a family member
// joins by invitation; the token stops working when rotated.
func TestEndToEnd(t *testing.T) {
	setEnv(t, "DENAR_AUTO_MIGRATE", "true")
	e := startE2E(t)

	if code, _ := e.browser().do("GET", "/healthz", "", nil); code != 200 {
		t.Fatalf("healthz = %d", code)
	}
	if code, _ := e.browser().do("GET", "/", "", nil); code != 200 {
		t.Fatalf("panel = %d", code)
	}

	admin := e.browser()
	admin.must(401, "GET", "/api/me", "", nil)
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)

	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	if h.Role != "owner" {
		t.Fatalf("household = %+v", h)
	}

	var created botWithToken
	header := admin.must(201, "POST", "/api/households/"+h.ID+"/bots", `{"name":"Claude","scopes":["wallets:read","transactions:write"]}`, &created)
	if !strings.HasPrefix(created.Token, "dnr_") || header.Get("Cache-Control") != "no-store" {
		t.Fatalf("bot = %+v", created)
	}

	bot := e.bot(created.Token)
	var me struct {
		Kind string `json:"kind"`
		Bot  struct {
			EffectiveScopes []string `json:"effective_scopes"`
		} `json:"bot"`
	}
	bot.must(200, "GET", "/api/me", "", &me)
	if me.Kind != "bot" || len(me.Bot.EffectiveScopes) != 2 {
		t.Errorf("bot me = %+v", me)
	}
	bot.must(200, "GET", "/api/households/"+h.ID, "", nil)
	bot.must(403, "GET", "/api/households/"+h.ID+"/members", "", nil)
	// A browser that also sends a bot token is refused outright.
	both := &client{e: e, http: admin.http, bearer: created.Token}
	both.must(400, "GET", "/api/me", "", nil)

	// A family member joins by invitation and registers.
	var inv struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	admin.must(201, "POST", "/api/households/"+h.ID+"/invitations", `{"email":"ola@example.com","role":"viewer"}`, &inv)
	if !strings.Contains(inv.URL, "/invite#"+inv.Token) {
		t.Errorf("invitation url = %q", inv.URL)
	}
	ola := e.browser()
	var preview struct {
		HouseholdName string `json:"household_name"`
		AccountExists bool   `json:"account_exists"`
	}
	ola.must(200, "POST", "/api/invitations/preview", `{"token":"`+inv.Token+`"}`, &preview)
	if preview.HouseholdName != "Dom" || preview.AccountExists {
		t.Errorf("preview = %+v", preview)
	}
	ola.must(201, "POST", "/api/invitations/register", `{"token":"`+inv.Token+`","name":"Ola","password":"a password for ola"}`, nil)
	var mine struct {
		Households []household `json:"households"`
	}
	ola.must(200, "GET", "/api/me", "", &mine)
	if len(mine.Households) != 1 || mine.Households[0].Role != "viewer" {
		t.Errorf("ola = %+v", mine)
	}
	ola.must(403, "POST", "/api/households/"+h.ID+"/bots", `{"name":"x","scopes":["wallets:read"]}`, nil)
	ola.must(409, "POST", "/api/invitations/register", `{"token":"`+inv.Token+`","name":"Ola","password":"a password for ola"}`, nil)

	// The audit log shows who did what, newest first.
	var audit struct {
		Items []struct {
			Action string `json:"action"`
			Actor  struct {
				Kind string `json:"kind"`
			} `json:"actor"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/households/"+h.ID+"/audit?limit=10", "", &audit)
	var actions []string
	for _, it := range audit.Items {
		actions = append(actions, it.Action)
	}
	if strings.Join(actions, ",") != "membership.joined,invitation.created,bot.created,household.created" {
		t.Errorf("audit = %v", actions)
	}

	// Rotation ends the old token at once.
	var rotated botWithToken
	admin.must(200, "POST", "/api/households/"+h.ID+"/bots/"+created.Bot.ID+"/token", "", &rotated)
	bot.must(401, "GET", "/api/me", "", nil)
	e.bot(rotated.Token).must(200, "GET", "/api/me", "", nil)

	admin.must(204, "POST", "/api/auth/logout", "", nil)
	admin.must(401, "GET", "/api/me", "", nil)

	// Nothing secret reaches the log.
	for _, secret := range []string{adminPassword, created.Token, rotated.Token, inv.Token, "a password for ola"} {
		if strings.Contains(e.logs.String(), secret) {
			t.Errorf("the log contains a secret %q", secret[:6])
		}
	}
}

func TestServeRefusesAnOldSchema(t *testing.T) {
	setEnv(t)
	_, err := newApp(context.Background(), mustConfig(t), slog.New(slog.DiscardHandler), testPanel())
	if err == nil || !strings.Contains(err.Error(), "denar migrate") {
		t.Fatalf("err = %v, want a hint to run denar migrate", err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"migrate"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "schema is up to date") {
		t.Errorf("migrate log = %s", out.String())
	}
	a, err := newApp(context.Background(), mustConfig(t), slog.New(slog.DiscardHandler), testPanel())
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
}

func TestCommands(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"version"}, &out, io.Discard); err != nil || strings.TrimSpace(out.String()) != "dev" {
		t.Errorf("version = %q, %v", out.String(), err)
	}
	for _, args := range [][]string{nil, {"nope"}} {
		if err := run(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
	setEnv(t, "DENAR_SESSION_TTL", "never")
	if err := run(context.Background(), []string{"migrate"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "DENAR_SESSION_TTL") {
		t.Errorf("bad config: %v", err)
	}
}

func TestTokenCreate(t *testing.T) {
	setEnv(t, "DENAR_AUTO_MIGRATE", "true")
	e := startE2E(t)
	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"UTC","month_start_day":1}`, &h)

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"token", "create", "--household", h.ID, "--name", "cron", "--scopes", "reports:read, wallets:read"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("token create: %v\n%s", err, stderr.String())
	}
	token := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(token, "dnr_") || strings.Contains(stderr.String(), token) {
		t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
	e.bot(token).must(200, "GET", "/api/households/"+h.ID, "", nil)

	var audit struct {
		Items []struct {
			Actor struct {
				Kind string `json:"kind"`
			} `json:"actor"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/households/"+h.ID+"/audit?resource_type=bot", "", &audit)
	if len(audit.Items) != 1 || audit.Items[0].Actor.Kind != "system" {
		t.Errorf("audit = %+v", audit)
	}

	for _, args := range [][]string{
		{"token"},
		{"token", "create", "--household", h.ID},
		{"token", "create", "--household", "nope", "--name", "x", "--scopes", "reports:read"},
		{"token", "create", "--household", h.ID, "--name", "y", "--scopes", "root"},
		{"token", "create", "--household", h.ID, "--name", "z", "--scopes", "reports:read", "--expires", "tomorrow"},
	} {
		if err := run(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}

// identityProvider is a local OIDC provider. It signs in whoever asks as
// grace@example.com, echoing the nonce of the last authorization.
type identityProvider struct {
	*httptest.Server
	mu    sync.Mutex
	nonce string
}

func newIdentityProvider(t *testing.T) *identityProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &identityProvider{}
	discovery := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "key-1", Algorithm: "RS256"}},
	}
	idp.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			discovery.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		id, secret, _ := r.BasicAuth()
		if id != oidcClientID || secret != oidcSecret || r.PostFormValue("code") != oidcCode || r.PostFormValue("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		idp.mu.Lock()
		nonce := idp.nonce
		idp.mu.Unlock()
		claims, _ := json.Marshal(map[string]any{
			"iss": idp.URL, "aud": oidcClientID, "sub": "grace-subject", "nonce": nonce,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": "grace@example.com", "email_verified": true, "name": "Grace Hopper",
		})
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "e2e-access-token", "token_type": "Bearer",
			"id_token": oidctest.SignIDToken(key, "key-1", "RS256", string(claims)),
		})
	}))
	t.Cleanup(idp.Close)
	discovery.SetIssuer(idp.URL)
	return idp
}

// TestOIDCLoginOfAnInvitedPerson: an invited email signs in through the
// provider with PKCE and a nonce, then accepts the invitation.
func TestOIDCLoginOfAnInvitedPerson(t *testing.T) {
	idp := newIdentityProvider(t)
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_BASE_URL", "http://localhost:8080",
		"DENAR_OIDC_ISSUER", idp.URL, "DENAR_OIDC_CLIENT_ID", oidcClientID, "DENAR_OIDC_CLIENT_SECRET", oidcSecret)
	e := startE2E(t)

	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"UTC","month_start_day":1}`, &h)

	grace := e.browser()
	login := func() int {
		header := grace.must(302, "GET", "/api/auth/login?return_to=/invite", "", nil)
		authURL, err := url.Parse(header.Get("Location"))
		if err != nil || !strings.HasPrefix(header.Get("Location"), idp.URL) {
			t.Fatalf("login redirects to %q", header.Get("Location"))
		}
		q := authURL.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("nonce") == "" {
			t.Fatalf("authorization request %v lacks PKCE or a nonce", q)
		}
		idp.mu.Lock()
		idp.nonce = q.Get("nonce")
		idp.mu.Unlock()
		code, _ := grace.do("GET", "/api/auth/callback?code="+oidcCode+"&state="+url.QueryEscape(q.Get("state")), "", nil)
		return code
	}
	// Not invited yet: refused.
	if code := login(); code != 401 {
		t.Fatalf("uninvited login = %d", code)
	}
	var inv struct {
		Token string `json:"token"`
	}
	admin.must(201, "POST", "/api/households/"+h.ID+"/invitations", `{"email":"grace@example.com","role":"member"}`, &inv)
	if code := login(); code != 302 {
		t.Fatalf("invited login = %d", code)
	}
	grace.must(200, "POST", "/api/invitations/accept", `{"token":"`+inv.Token+`"}`, nil)
	var me struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
		Households []household `json:"households"`
	}
	grace.must(200, "GET", "/api/me", "", &me)
	if me.User.Email != "grace@example.com" || len(me.Households) != 1 || me.Households[0].Role != "member" {
		t.Errorf("grace = %+v", me)
	}
}
