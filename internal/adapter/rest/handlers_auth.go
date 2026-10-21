package rest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	"github.com/paramah/denar/internal/adapter/rest/api"
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/usecase"
)

// The contract declares no Set-Cookie or Location headers, so the
// responses below set them themselves instead of using the generated types.

// redirect answers 302 and sets cookies on the way.
type redirect struct {
	location string
	cookies  []*http.Cookie
}

func (r redirect) write(w http.ResponseWriter) error {
	for _, c := range r.cookies {
		http.SetCookie(w, c)
	}
	w.Header().Set("Location", r.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

func (r redirect) VisitLoginStartResponse(w http.ResponseWriter) error    { return r.write(w) }
func (r redirect) VisitLoginCallbackResponse(w http.ResponseWriter) error { return r.write(w) }

// signedIn answers a local login with the user and the session cookie.
type signedIn struct {
	user   api.User
	cookie *http.Cookie
}

func (r signedIn) VisitLoginLocalResponse(w http.ResponseWriter) error {
	http.SetCookie(w, r.cookie)
	w.Header().Set("Cache-Control", "no-store")
	return api.LoginLocal200JSONResponse(r.user).VisitLoginLocalResponse(w)
}

// signedOut answers 204 and drops the session cookie.
type signedOut struct {
	cookie *http.Cookie
}

func (r signedOut) VisitLogoutResponse(w http.ResponseWriter) error {
	http.SetCookie(w, r.cookie)
	return api.Logout204Response{}.VisitLogoutResponse(w)
}

func (s *Server) logger() *slog.Logger {
	if s.deps.Logger == nil {
		return slog.Default()
	}
	return s.deps.Logger
}

func (s *Server) LoginStart(ctx context.Context, request api.LoginStartRequestObject) (api.LoginStartResponseObject, error) {
	start, err := s.deps.LoginOIDCStart.Do(ctx, usecase.LoginOIDCStartInput{ReturnTo: deref(request.Params.ReturnTo)})
	if err != nil {
		return nil, err
	}
	return redirect{location: start.AuthURL, cookies: []*http.Cookie{oidcCookie(s.deps, start.State)}}, nil
}

func (s *Server) LoginCallback(ctx context.Context, request api.LoginCallbackRequestObject) (api.LoginCallbackResponseObject, error) {
	c := callerFrom(ctx)
	in := usecase.LoginOIDCCallbackInput{
		State:         deref(request.Params.State),
		CookieState:   c.oidcState,
		PreviousToken: c.sessionToken,
	}
	// A provider that reports an error has not signed anyone in, whatever
	// else the query holds.
	if request.Params.Error == nil {
		in.Code = deref(request.Params.Code)
	}
	login, err := s.deps.LoginOIDCCallback.Do(ctx, in)
	if err != nil {
		var refused usecase.OIDCLoginError
		if errors.As(err, &refused) {
			s.logger().LogAttrs(ctx, slog.LevelWarn, "oidc login refused",
				slog.String("request_id", requestID(ctx)),
				slog.String("reason", string(refused.Reason)),
				slog.String("cause_type", refused.CauseType),
			)
		}
		return nil, err
	}
	return redirect{location: login.ReturnTo, cookies: []*http.Cookie{
		sessionCookie(s.deps, login.Token),
		expired(oidcCookie(s.deps, "")),
	}}, nil
}

func (s *Server) LoginLocal(ctx context.Context, request api.LoginLocalRequestObject) (api.LoginLocalResponseObject, error) {
	login, err := s.deps.LoginLocal.Do(ctx, usecase.LoginLocalInput{
		Email:         request.Body.Email,
		Password:      request.Body.Password,
		PreviousToken: callerFrom(ctx).sessionToken,
	})
	if err != nil {
		if errors.Is(err, domain.ErrUnauthenticated) {
			// The email is logged as a hash: enough to see one account
			// being tried again and again, without storing who.
			sum := sha256.Sum256([]byte(user.NormalizeEmail(request.Body.Email)))
			s.logger().LogAttrs(ctx, slog.LevelWarn, "local login refused",
				slog.String("request_id", requestID(ctx)),
				slog.String("email_hash", hex.EncodeToString(sum[:8])),
			)
		}
		return nil, err
	}
	return signedIn{user: apiUser(login.User), cookie: sessionCookie(s.deps, login.Token)}, nil
}

func (s *Server) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	if err := s.deps.Logout.Do(ctx, callerFrom(ctx).sessionToken); err != nil {
		return nil, err
	}
	return signedOut{cookie: expired(sessionCookie(s.deps, ""))}, nil
}

// authConfig answers with the login configuration and tells caches to ask
// again each time.
type authConfig api.AuthConfig

func (c authConfig) VisitGetAuthConfigResponse(w http.ResponseWriter) error {
	w.Header().Set("Cache-Control", "no-cache")
	return api.GetAuthConfig200JSONResponse(c).VisitGetAuthConfigResponse(w)
}

func (s *Server) GetAuthConfig(context.Context, api.GetAuthConfigRequestObject) (api.GetAuthConfigResponseObject, error) {
	cfg := s.deps.GetAuthConfig.Do()
	return authConfig{OidcEnabled: cfg.OIDCEnabled, LocalLogin: cfg.LocalLogin, InstanceName: cfg.InstanceName, Version: cfg.Version}, nil
}

func (s *Server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	me, err := s.deps.GetMe.Do(ctx, principalFrom(ctx))
	if err != nil {
		return nil, err
	}
	out := api.GetMe200JSONResponse{Households: apiHouseholds(me.Households)}
	if me.Bot != nil {
		b := apiBot(*me.Bot)
		out.Kind, out.Bot = api.MeKindBot, &b
	} else {
		u := apiUser(*me.User)
		out.Kind, out.User = api.MeKindUser, &u
	}
	return out, nil
}

func (s *Server) ListCurrencies(context.Context, api.ListCurrenciesRequestObject) (api.ListCurrenciesResponseObject, error) {
	all := money.Currencies()
	items := make([]api.Currency, len(all))
	for i, c := range all {
		items[i] = api.Currency{Code: c.Code, Decimals: c.Decimals}
	}
	return api.ListCurrencies200JSONResponse{Items: items}, nil
}

func (s *Server) UpdateMe(ctx context.Context, request api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	p := principalFrom(ctx)
	if p.IsBot() {
		return nil, domain.ForbiddenError{Reason: domain.ForbiddenHumanOnly}
	}
	if request.Body == nil || !request.Body.Locale.IsSpecified() {
		return nil, domain.ValidationError{Field: "locale", Message: "is required"}
	}
	var in usecase.UpdateMeInput
	if !request.Body.Locale.IsNull() {
		l := string(request.Body.Locale.MustGet())
		in.Locale = &l
	}
	u, err := s.deps.UpdateMe.Do(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.UpdateMe200JSONResponse(apiUser(u)), nil
}
