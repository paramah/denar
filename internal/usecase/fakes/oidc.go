package fakes

import (
	"context"
	"net/url"
	"slices"
	"sync"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// OIDCAuthCall is the arguments of one AuthURL call.
type OIDCAuthCall struct {
	State, Nonce, CodeVerifier string
}

// OIDCExchangeCall is the arguments of one Exchange call.
type OIDCExchangeCall struct {
	Code, Nonce, CodeVerifier string
}

// OIDCProvider answers as the test programs it and records its calls.
// Set the exported fields before the code under test runs.
type OIDCProvider struct {
	On          bool                 // what Enabled reports
	AuthURLErr  error                // returned by AuthURL when set
	Identity    usecase.OIDCIdentity // returned by Exchange
	ExchangeErr error                // returned by Exchange when set

	mu            sync.Mutex
	authCalls     []OIDCAuthCall
	exchangeCalls []OIDCExchangeCall
}

// NewOIDCProvider returns a provider that is turned off.
func NewOIDCProvider() *OIDCProvider { return &OIDCProvider{} }

func (p *OIDCProvider) Enabled() bool { return p.On }

func (p *OIDCProvider) AuthURL(_ context.Context, state, nonce, codeVerifier string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authCalls = append(p.authCalls, OIDCAuthCall{State: state, Nonce: nonce, CodeVerifier: codeVerifier})
	if p.AuthURLErr != nil {
		return "", p.AuthURLErr
	}
	return "https://idp.test/authorize?state=" + url.QueryEscape(state), nil
}

func (p *OIDCProvider) Exchange(_ context.Context, code, nonce, codeVerifier string) (usecase.OIDCIdentity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exchangeCalls = append(p.exchangeCalls, OIDCExchangeCall{Code: code, Nonce: nonce, CodeVerifier: codeVerifier})
	if p.ExchangeErr != nil {
		return usecase.OIDCIdentity{}, p.ExchangeErr
	}
	return p.Identity, nil
}

// AuthCalls returns the AuthURL calls made so far, in order.
func (p *OIDCProvider) AuthCalls() []OIDCAuthCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.authCalls)
}

// ExchangeCalls returns the Exchange calls made so far, in order.
func (p *OIDCProvider) ExchangeCalls() []OIDCExchangeCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.exchangeCalls)
}
