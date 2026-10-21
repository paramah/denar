package oidc

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/paramah/denar/internal/usecase"
)

// requestTimeout bounds every request to the identity provider.
const requestTimeout = 10 * time.Second

// retryDelay is how long a failed discovery answers later callers before
// the provider is asked again. Without it every login attempt against an
// unreachable provider would wait for a lookup of its own.
const retryDelay = 5 * time.Second

// Config says which identity provider to use and how the server is
// registered with it. OIDC is off unless Issuer, ClientID and ClientSecret
// are all set.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string // the server's /api/auth/callback
}

// Provider implements usecase.OIDCProvider with the authorization code flow
// and PKCE. It is safe for concurrent use.
type Provider struct {
	cfg    Config
	client *http.Client
	now    func() time.Time

	mu         sync.Mutex
	discovered *discovered
	failure    error     // the last discovery error
	failedAt   time.Time // when failure happened
}

// discovered is what the provider's discovery document gives.
type discovered struct {
	endpoint oauth2.Endpoint
	verifier *gooidc.IDTokenVerifier
}

// New returns a Provider without contacting the identity provider, so an
// unreachable one does not stop the server from starting.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg, client: &http.Client{Timeout: requestTimeout}, now: time.Now}
}

func (p *Provider) Enabled() bool {
	return p.cfg.Issuer != "" && p.cfg.ClientID != "" && p.cfg.ClientSecret != ""
}

func (p *Provider) AuthURL(_ context.Context, state, nonce, codeVerifier string) (string, error) {
	d, err := p.discover()
	if err != nil {
		return "", err
	}
	return p.oauth2Config(d).AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(codeVerifier)), nil
}

// Exchange errors never contain the code, the verifier, the nonce or a
// token, so the caller may log them.
func (p *Provider) Exchange(ctx context.Context, code, nonce, codeVerifier string) (usecase.OIDCIdentity, error) {
	d, err := p.discover()
	if err != nil {
		return usecase.OIDCIdentity{}, err
	}
	ctx = gooidc.ClientContext(ctx, p.client)

	token, err := p.oauth2Config(d).Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		// A RetrieveError prints the provider's whole response; keep only
		// the error code.
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return usecase.OIDCIdentity{}, fmt.Errorf("oidc: token endpoint refused the code: %q", re.ErrorCode)
		}
		return usecase.OIDCIdentity{}, fmt.Errorf("oidc: exchanging the code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return usecase.OIDCIdentity{}, errors.New("oidc: token response has no id_token")
	}

	// Verify checks the signature, issuer, audience and expiry.
	idToken, err := d.verifier.Verify(ctx, raw)
	if err != nil {
		return usecase.OIDCIdentity{}, fmt.Errorf("oidc: verifying the ID token: %w", err)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return usecase.OIDCIdentity{}, errors.New("oidc: ID token nonce does not match")
	}
	if idToken.Subject == "" {
		return usecase.OIDCIdentity{}, errors.New("oidc: ID token has no subject")
	}

	var claims struct {
		Email             string `json:"email"`
		EmailVerified     *bool  `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return usecase.OIDCIdentity{}, errors.New("oidc: ID token claims have unexpected types")
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	return usecase.OIDCIdentity{
		Issuer:        idToken.Issuer,
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          name,
	}, nil
}

// discover fetches the provider's discovery document on first use and keeps
// it. A failure is kept for retryDelay only, then the next call tries again.
func (p *Provider) discover() (*discovered, error) {
	if !p.Enabled() {
		return nil, errors.New("oidc: not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.discovered != nil {
		return p.discovered, nil
	}
	if p.failure != nil && p.now().Sub(p.failedAt) < retryDelay {
		return nil, p.failure
	}
	// The key set keeps this context to fetch keys later, so it must
	// outlive the request; the client's timeout bounds the lookup.
	provider, err := gooidc.NewProvider(gooidc.ClientContext(context.Background(), p.client), p.cfg.Issuer)
	if err != nil {
		p.failure, p.failedAt = fmt.Errorf("oidc: discovering the provider: %w", err), p.now()
		return nil, p.failure
	}
	p.failure = nil
	p.discovered = &discovered{
		endpoint: provider.Endpoint(),
		verifier: provider.Verifier(&gooidc.Config{ClientID: p.cfg.ClientID}),
	}
	return p.discovered, nil
}

func (p *Provider) oauth2Config(d *discovered) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURL,
		Endpoint:     d.endpoint,
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}
}
