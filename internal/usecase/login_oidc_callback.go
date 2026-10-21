package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/user"
)

// OIDCLoginFailure names why an OIDC login was refused.
type OIDCLoginFailure string

const (
	OIDCDisabled        OIDCLoginFailure = "oidc_disabled"
	OIDCStateMismatch   OIDCLoginFailure = "state_mismatch"   // query and cookie differ or are missing
	OIDCStateUnknown    OIDCLoginFailure = "state_unknown"    // never issued or already used
	OIDCStateExpired    OIDCLoginFailure = "state_expired"    // the login took too long
	OIDCProviderRefused OIDCLoginFailure = "provider_refused" // the provider sent no code
	OIDCExchangeFailed  OIDCLoginFailure = "exchange_failed"  // code or ID token rejected
	OIDCEmailMissing    OIDCLoginFailure = "email_missing"
	OIDCEmailUnverified OIDCLoginFailure = "email_unverified"
	OIDCEmailTaken      OIDCLoginFailure = "email_taken" // another account has the email
	OIDCNotInvited      OIDCLoginFailure = "not_invited" // no account, no invitation, no auto-create
	OIDCUserInactive    OIDCLoginFailure = "user_inactive"
)

// OIDCLoginError is a refused OIDC login. It matches
// domain.ErrUnauthenticated, so the client gets the same answer for every
// reason; Reason and CauseType are for the server log. It never holds
// codes, tokens or the provider's error text.
type OIDCLoginError struct {
	Reason OIDCLoginFailure
	// CauseType is the Go type of the provider's error, if there was one.
	CauseType string
}

func (e OIDCLoginError) Error() string {
	text := "unauthenticated: oidc login refused: " + string(e.Reason)
	if e.CauseType != "" {
		text += " (" + e.CauseType + ")"
	}
	return text
}

func (e OIDCLoginError) Is(target error) bool { return target == domain.ErrUnauthenticated }

type LoginOIDCCallbackInput struct {
	// Code is empty when the provider reported an error instead.
	Code string
	// State is the value from the query, CookieState the one from the
	// cookie set when the login started.
	State       string
	CookieState string
	// PreviousToken is the session token the browser sent, if any.
	PreviousToken string
}

// OIDCLogin is a finished OIDC login and where to send the browser.
type OIDCLogin struct {
	Login
	ReturnTo string
}

// LoginOIDCCallbackConfig holds the settings of OIDC sign-in.
type LoginOIDCCallbackConfig struct {
	SessionTTL time.Duration
	// AutoCreate gives every user of the provider an account on first
	// login (DENAR_OIDC_AUTO_CREATE). Off, a new account needs a pending
	// invitation for the verified email (decision D-005).
	AutoCreate bool
}

// LoginOIDCCallback finishes an OIDC login. The state works once, whatever
// the outcome. An account is found by issuer and subject only, never
// linked by email.
type LoginOIDCCallback struct {
	provider    OIDCProvider
	states      OIDCStateRepository
	users       UserRepository
	invitations InvitationRepository
	sessions    sessionStarter
	ids         IDGenerator
	clock       Clock
	tx          TxManager
	autoCreate  bool
}

func NewLoginOIDCCallback(provider OIDCProvider, states OIDCStateRepository, users UserRepository,
	invitations InvitationRepository, sessions SessionRepository, tokens TokenGenerator, ids IDGenerator,
	clock Clock, tx TxManager, cfg LoginOIDCCallbackConfig) *LoginOIDCCallback {
	return &LoginOIDCCallback{
		provider:    provider,
		states:      states,
		users:       users,
		invitations: invitations,
		sessions:    sessionStarter{sessions: sessions, tokens: tokens, clock: clock, ttl: cfg.SessionTTL},
		ids:         ids,
		clock:       clock,
		tx:          tx,
		autoCreate:  cfg.AutoCreate,
	}
}

func (uc *LoginOIDCCallback) Do(ctx context.Context, in LoginOIDCCallbackInput) (OIDCLogin, error) {
	if !uc.provider.Enabled() {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCDisabled}
	}
	if in.State == "" || subtle.ConstantTimeCompare([]byte(in.State), []byte(in.CookieState)) != 1 {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCStateMismatch}
	}

	// The state is consumed outside the transaction below, so that it
	// stays used when the login fails later.
	state, err := uc.states.Consume(ctx, hashToken(in.State))
	if found, err := ignoreNotFound(err); err != nil {
		return OIDCLogin{}, err
	} else if !found {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCStateUnknown}
	}
	if !state.ExpiresAt.After(uc.clock.Now()) {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCStateExpired}
	}
	if in.Code == "" {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCProviderRefused}
	}

	identity, err := uc.provider.Exchange(ctx, in.Code, state.Nonce, state.CodeVerifier)
	if err != nil {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCExchangeFailed, CauseType: fmt.Sprintf("%T", err)}
	}
	if identity.Issuer == "" || identity.Subject == "" {
		return OIDCLogin{}, OIDCLoginError{Reason: OIDCExchangeFailed}
	}

	var login Login
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		u, err := uc.resolveUser(ctx, identity)
		if err != nil {
			return err
		}
		login, err = uc.sessions.start(ctx, u, in.PreviousToken)
		return err
	})
	if err != nil {
		return OIDCLogin{}, err
	}
	return OIDCLogin{Login: login, ReturnTo: safeReturnTo(state.ReturnTo)}, nil
}

// resolveUser finds the account of the identity or creates it.
func (uc *LoginOIDCCallback) resolveUser(ctx context.Context, identity OIDCIdentity) (user.User, error) {
	u, err := uc.users.ByOIDCSubject(ctx, identity.Issuer, identity.Subject)
	found, err := ignoreNotFound(err)
	if err != nil {
		return user.User{}, err
	}
	if found {
		if !u.Active {
			return user.User{}, OIDCLoginError{Reason: OIDCUserInactive}
		}
		return u, nil
	}

	email := user.NormalizeEmail(identity.Email)
	if email == "" {
		return user.User{}, OIDCLoginError{Reason: OIDCEmailMissing}
	}
	if identity.EmailVerified == nil || !*identity.EmailVerified {
		return user.User{}, OIDCLoginError{Reason: OIDCEmailUnverified}
	}
	// An account with this email is never linked: whoever controls the
	// provider could otherwise take over a local account.
	_, err = uc.users.ByEmail(ctx, email)
	if taken, err := ignoreNotFound(err); err != nil {
		return user.User{}, err
	} else if taken {
		return user.User{}, OIDCLoginError{Reason: OIDCEmailTaken}
	}
	if !uc.autoCreate {
		pending, err := uc.invitations.PendingByEmail(ctx, email, uc.clock.Now())
		if err != nil {
			return user.User{}, err
		}
		if len(pending) == 0 {
			return user.User{}, OIDCLoginError{Reason: OIDCNotInvited}
		}
	}

	name := strings.TrimSpace(identity.Name)
	if name == "" || user.ValidateName("name", name) != nil {
		name = email
	}
	u = user.User{
		ID:          uc.ids.NewID(),
		Email:       email,
		Name:        name,
		OIDCIssuer:  identity.Issuer,
		OIDCSubject: identity.Subject,
		Active:      true,
		CreatedAt:   uc.clock.Now(),
	}
	if err := uc.users.Create(ctx, u); err != nil {
		var conflict domain.ConflictError
		if errors.As(err, &conflict) && conflict.Reason == domain.ConflictDuplicateEmail {
			return user.User{}, OIDCLoginError{Reason: OIDCEmailTaken}
		}
		return user.User{}, err
	}
	return u, nil
}
