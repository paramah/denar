package usecase

import (
	"context"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// oidcStateTTL is how long a started OIDC login may take.
const oidcStateTTL = 10 * time.Minute

type LoginOIDCStartInput struct {
	// ReturnTo is where the browser should land after the login. Anything
	// but a path on this server is replaced with "/".
	ReturnTo string
}

// OIDCStart is what the browser needs to continue at the provider. State
// goes into a cookie, so that the callback can be tied to this browser.
type OIDCStart struct {
	AuthURL string
	State   string
}

// LoginOIDCStart begins an OIDC login: it makes the state, nonce and PKCE
// verifier, remembers them under the hash of the state and returns the
// provider's URL. It gives domain.ErrNotFound when OIDC is not configured.
type LoginOIDCStart struct {
	provider OIDCProvider
	states   OIDCStateRepository
	tokens   TokenGenerator
	clock    Clock
}

func NewLoginOIDCStart(provider OIDCProvider, states OIDCStateRepository, tokens TokenGenerator, clock Clock) *LoginOIDCStart {
	return &LoginOIDCStart{provider: provider, states: states, tokens: tokens, clock: clock}
}

func (uc *LoginOIDCStart) Do(ctx context.Context, in LoginOIDCStartInput) (OIDCStart, error) {
	if !uc.provider.Enabled() {
		return OIDCStart{}, domain.ErrNotFound
	}
	var secrets [3]string
	for i := range secrets {
		secret, err := uc.tokens.NewToken()
		if err != nil {
			return OIDCStart{}, err
		}
		secrets[i] = secret
	}
	state, nonce, verifier := secrets[0], secrets[1], secrets[2]

	authURL, err := uc.provider.AuthURL(ctx, state, nonce, verifier)
	if err != nil {
		return OIDCStart{}, err
	}
	err = uc.states.Create(ctx, OIDCLoginState{
		StateHash:    hashToken(state),
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnTo:     safeReturnTo(in.ReturnTo),
		ExpiresAt:    uc.clock.Now().Add(oidcStateTTL),
	})
	if err != nil {
		return OIDCStart{}, err
	}
	return OIDCStart{AuthURL: authURL, State: state}, nil
}

// maxReturnToBytes bounds the return path kept for a login in progress.
const maxReturnToBytes = 2048

// safeReturnTo returns target when it is a path on this server and "/"
// otherwise, so that a login link cannot send the user to another site.
// Browsers read "//host" and "/\host" as another host, hence the checks
// beyond the leading slash. A path longer than maxReturnToBytes is dropped
// as well: starting a login needs no session and stores the path.
func safeReturnTo(target string) string {
	if len(target) > maxReturnToBytes {
		return "/"
	}
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.Contains(target, `\`) {
		return "/"
	}
	for _, r := range target {
		if r < 0x20 || r == 0x7f {
			return "/"
		}
	}
	return target
}
