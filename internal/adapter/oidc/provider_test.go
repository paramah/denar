package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc/oidctest"

	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.OIDCProvider = (*Provider)(nil)

const (
	clientID     = "denar"
	clientSecret = "client-secret-value"
	redirectURL  = "https://board.example.com/api/auth/callback"
	authCode     = "auth-code-value"
	verifier     = "code-verifier-value-code-verifier-value-0123"
	nonce        = "nonce-value"
	// providerDetail stands for whatever a provider puts into an error
	// answer; none of it may come back out of the adapter.
	providerDetail = "provider-says-user-ada-is-locked"
)

// One key pair for the whole package; generating RSA keys is slow.
var signingKey, otherKey = sync.OnceValue(newKey), sync.OnceValue(newKey)

func newKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

// fakeIdP is a local identity provider: discovery and keys come from
// oidctest, the token endpoint is ours.
type fakeIdP struct {
	*httptest.Server
	down       atomic.Bool  // answer everything with 503
	requests   atomic.Int32 // requests received, whatever the answer
	discovered atomic.Int32 // discovery requests served

	mu         sync.Mutex
	claims     map[string]any  // ID token claims; nil key removes a default
	signer     *rsa.PrivateKey // key signing the ID token
	tokenError string          // OAuth2 error the token endpoint returns
	// tokenErrorBody, when set, is answered as it is with status 500: a
	// provider failing outside the OAuth2 error format.
	tokenErrorBody string
	omitToken      bool   // leave id_token out of the token response
	challenge      string // code_challenge the code was issued for
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	idp := &fakeIdP{signer: signingKey()}
	discovery := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: signingKey().Public(), KeyID: "key-1", Algorithm: "RS256"}},
	}
	idp.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idp.requests.Add(1)
		switch {
		case idp.down.Load():
			http.Error(w, "down", http.StatusServiceUnavailable)
		case r.URL.Path == "/token":
			idp.serveToken(w, r)
		default:
			if r.URL.Path == "/.well-known/openid-configuration" {
				idp.discovered.Add(1)
			}
			discovery.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(idp.Close)
	discovery.SetIssuer(idp.URL)
	return idp
}

func (idp *fakeIdP) serveToken(w http.ResponseWriter, r *http.Request) {
	idp.mu.Lock()
	defer idp.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	id, secret, basic := r.BasicAuth()
	if !basic {
		id, secret = r.PostFormValue("client_id"), r.PostFormValue("client_secret")
	}
	sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
	switch {
	case idp.tokenErrorBody != "":
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(idp.tokenErrorBody))
		return
	case idp.tokenError != "":
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             idp.tokenError,
			"error_description": providerDetail,
			"error_uri":         "https://idp.example.com/errors/" + providerDetail,
		})
		return
	case id != clientID || secret != clientSecret:
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
		return
	case r.PostFormValue("code") != authCode ||
		r.PostFormValue("redirect_uri") != redirectURL ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != idp.challenge:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		return
	}

	claims := map[string]any{
		"iss":   idp.URL,
		"aud":   clientID,
		"sub":   "subject-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
		"email": "ada@example.com",
		"name":  "Ada Lovelace",
	}
	for k, v := range idp.claims {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	raw, _ := json.Marshal(claims)
	resp := map[string]any{"access_token": "access-token-value", "token_type": "Bearer"}
	if !idp.omitToken {
		resp["id_token"] = oidctest.SignIDToken(idp.signer, "key-1", "RS256", string(raw))
	}
	json.NewEncoder(w).Encode(resp)
}

func newProvider(idp *fakeIdP) *Provider {
	return New(Config{Issuer: idp.URL, ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL})
}

// login runs AuthURL and hands its code challenge to the fake provider, as
// a browser redirect would.
func login(t *testing.T, p *Provider, idp *fakeIdP) {
	t.Helper()
	raw, err := p.AuthURL(context.Background(), "state-value", nonce, verifier)
	if err != nil {
		t.Fatalf("AuthURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	idp.mu.Lock()
	idp.challenge = u.Query().Get("code_challenge")
	idp.mu.Unlock()
}

func TestEnabled(t *testing.T) {
	full := Config{Issuer: "https://id.example.com", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL}
	tests := []struct {
		name   string
		change func(*Config)
		want   bool
	}{
		{"complete", func(*Config) {}, true},
		{"no issuer", func(c *Config) { c.Issuer = "" }, false},
		{"no client id", func(c *Config) { c.ClientID = "" }, false},
		{"no client secret", func(c *Config) { c.ClientSecret = "" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := full
			tt.change(&cfg)
			if got := New(cfg).Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisabledProviderRefuses(t *testing.T) {
	p := New(Config{})
	if _, err := p.AuthURL(context.Background(), "s", "n", "v"); err == nil {
		t.Error("AuthURL succeeded on a disabled provider")
	}
	if _, err := p.Exchange(context.Background(), "c", "n", "v"); err == nil {
		t.Error("Exchange succeeded on a disabled provider")
	}
}

func TestAuthURL(t *testing.T) {
	idp := newFakeIdP(t)

	raw, err := newProvider(idp).AuthURL(context.Background(), "state-value", nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(raw, idp.URL+"/auth?") {
		t.Errorf("AuthURL() = %q, want the provider's authorization endpoint", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(verifier))
	want := map[string]string{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          redirectURL,
		"scope":                 "openid profile email",
		"state":                 "state-value",
		"nonce":                 nonce,
		"code_challenge_method": "S256",
		"code_challenge":        base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	for key, value := range want {
		if got := u.Query().Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
	if strings.Contains(raw, verifier) || strings.Contains(raw, clientSecret) {
		t.Errorf("AuthURL() = %q leaks the verifier or the client secret", raw)
	}
}

func TestExchange(t *testing.T) {
	idp := newFakeIdP(t)
	p := newProvider(idp)
	login(t, p, idp)

	got, err := p.Exchange(context.Background(), authCode, nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}

	want := usecase.OIDCIdentity{Issuer: idp.URL, Subject: "subject-1", Email: "ada@example.com", Name: "Ada Lovelace"}
	if got != want {
		t.Errorf("Exchange() = %+v, want %+v", got, want)
	}
}

func TestExchangeEmailVerified(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name  string
		claim any
		want  *bool
	}{
		{"absent", nil, nil},
		{"true", true, &yes},
		{"false", false, &no},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp := newFakeIdP(t)
			idp.claims = map[string]any{"email_verified": tt.claim}
			p := newProvider(idp)
			login(t, p, idp)

			got, err := p.Exchange(context.Background(), authCode, nonce, verifier)
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.want == nil && got.EmailVerified != nil:
				t.Errorf("EmailVerified = %v, want nil", *got.EmailVerified)
			case tt.want != nil && (got.EmailVerified == nil || *got.EmailVerified != *tt.want):
				t.Errorf("EmailVerified = %v, want %v", got.EmailVerified, *tt.want)
			}
		})
	}
}

func TestExchangeNameFallsBackToUsername(t *testing.T) {
	idp := newFakeIdP(t)
	idp.claims = map[string]any{"name": nil, "preferred_username": "ada"}
	p := newProvider(idp)
	login(t, p, idp)

	got, err := p.Exchange(context.Background(), authCode, nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ada" {
		t.Errorf("Name = %q, want the preferred_username", got.Name)
	}
}

func TestExchangeRejects(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*fakeIdP)
		code     string
		nonce    string
		verifier string
	}{
		{"wrong nonce", func(*fakeIdP) {}, authCode, "another-nonce", verifier},
		{"empty nonce on both sides", func(i *fakeIdP) { i.claims = map[string]any{"nonce": nil} }, authCode, "", verifier},
		{"token without nonce", func(i *fakeIdP) { i.claims = map[string]any{"nonce": nil} }, authCode, nonce, verifier},
		{"wrong audience", func(i *fakeIdP) { i.claims = map[string]any{"aud": "another-client"} }, authCode, nonce, verifier},
		{"wrong issuer", func(i *fakeIdP) { i.claims = map[string]any{"iss": "https://evil.example.com"} }, authCode, nonce, verifier},
		{"expired token", func(i *fakeIdP) { i.claims = map[string]any{"exp": time.Now().Add(-time.Hour).Unix()} }, authCode, nonce, verifier},
		{"token without subject", func(i *fakeIdP) { i.claims = map[string]any{"sub": nil} }, authCode, nonce, verifier},
		{"unknown signing key", func(i *fakeIdP) { i.signer = otherKey() }, authCode, nonce, verifier},
		{"provider error", func(i *fakeIdP) { i.tokenError = "access_denied" }, authCode, nonce, verifier},
		{"no id token", func(i *fakeIdP) { i.omitToken = true }, authCode, nonce, verifier},
		{"wrong code", func(*fakeIdP) {}, "another-code-value", nonce, verifier},
		{"wrong verifier", func(*fakeIdP) {}, authCode, nonce, "another-verifier-value-another-verifier-0123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp := newFakeIdP(t)
			tt.setup(idp)
			p := newProvider(idp)
			login(t, p, idp)

			got, err := p.Exchange(context.Background(), tt.code, tt.nonce, tt.verifier)
			if err == nil {
				t.Fatalf("Exchange() = %+v, want an error", got)
			}
			if got != (usecase.OIDCIdentity{}) {
				t.Errorf("Exchange() = %+v with an error, want the zero identity", got)
			}
			for _, secret := range []string{tt.code, tt.verifier, tt.nonce, nonce, clientSecret, "access-token-value"} {
				if secret != "" && strings.Contains(err.Error(), secret) {
					t.Errorf("error %q leaks %q", err, secret)
				}
			}
		})
	}
}

// An unreachable provider must not fail construction; the adapter looks it
// up again on the next call and then keeps the result.
func TestProviderUnavailableThenAvailable(t *testing.T) {
	idp := newFakeIdP(t)
	idp.down.Store(true)

	p := newProvider(idp)
	now := time.Now()
	p.now = func() time.Time { return now }
	if !p.Enabled() {
		t.Fatal("Enabled() = false while the provider is down")
	}
	if _, err := p.AuthURL(context.Background(), "state-value", nonce, verifier); err == nil {
		t.Fatal("AuthURL succeeded while the provider is down")
	}
	if _, err := p.Exchange(context.Background(), authCode, nonce, verifier); err == nil {
		t.Fatal("Exchange succeeded while the provider is down")
	}

	idp.down.Store(false)
	now = now.Add(retryDelay)
	login(t, p, idp)
	if _, err := p.Exchange(context.Background(), authCode, nonce, verifier); err != nil {
		t.Fatalf("Exchange after the provider came back: %v", err)
	}
	if _, err := p.AuthURL(context.Background(), "state-value", nonce, verifier); err != nil {
		t.Fatal(err)
	}
	if got := idp.discovered.Load(); got != 1 {
		t.Errorf("discovery served %d times, want 1 (cached after the first success)", got)
	}
}

func TestProviderUnreachable(t *testing.T) {
	idp := newFakeIdP(t)
	p := newProvider(idp)
	idp.Close()

	if _, err := p.AuthURL(context.Background(), "state-value", nonce, verifier); err == nil {
		t.Error("AuthURL succeeded with the provider closed")
	}
}

func TestConcurrentFirstUse(t *testing.T) {
	idp := newFakeIdP(t)
	p := newProvider(idp)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := p.AuthURL(context.Background(), "state-value", nonce, verifier); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := idp.discovered.Load(); got != 1 {
		t.Errorf("discovery served %d times, want 1", got)
	}
}

// While the provider is down, callers must not each start a lookup of their
// own: one failed attempt answers everyone until the retry delay has passed.
func TestFailedDiscoveryIsNotRepeatedAtOnce(t *testing.T) {
	idp := newFakeIdP(t)
	idp.down.Store(true)
	p := newProvider(idp)
	now := time.Now()
	p.now = func() time.Time { return now }

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := p.AuthURL(context.Background(), "state-value", nonce, verifier); err == nil {
				t.Error("AuthURL succeeded while the provider is down")
			}
		})
	}
	wg.Wait()
	if got := idp.requests.Load(); got != 1 {
		t.Errorf("provider received %d requests from 50 callers, want 1", got)
	}

	now = now.Add(retryDelay - time.Millisecond)
	p.AuthURL(context.Background(), "state-value", nonce, verifier)
	if got := idp.requests.Load(); got != 1 {
		t.Errorf("provider received %d requests before the retry delay passed, want 1", got)
	}

	now = now.Add(time.Millisecond)
	p.AuthURL(context.Background(), "state-value", nonce, verifier)
	if got := idp.requests.Load(); got != 2 {
		t.Errorf("provider received %d requests after the retry delay, want 2", got)
	}
}

// Whatever the provider answers at the token endpoint, the adapter's error
// carries the OAuth2 error code at most: providers echo request data and
// internal detail in their error answers.
func TestExchangeErrorHoldsNothingOfTheProvidersAnswer(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeIdP)
		want  string
	}{
		{"OAuth2 error with a description", func(i *fakeIdP) { i.tokenError = "invalid_grant" }, "invalid_grant"},
		{"answer outside the OAuth2 format", func(i *fakeIdP) {
			i.tokenErrorBody = "stack trace: " + providerDetail + " code=" + authCode
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp := newFakeIdP(t)
			tt.setup(idp)
			p := newProvider(idp)
			login(t, p, idp)

			_, err := p.Exchange(context.Background(), authCode, nonce, verifier)

			if err == nil {
				t.Fatal("Exchange succeeded")
			}
			for _, leak := range []string{providerDetail, "stack trace", authCode, verifier, clientSecret} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error %q holds %q from the provider's answer", err, leak)
				}
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name the code %q", err, tt.want)
			}
		})
	}
}
