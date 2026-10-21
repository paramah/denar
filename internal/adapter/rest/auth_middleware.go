package rest

import (
	"context"
	"net/http"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

// publicRoutes are served without credentials: the health check and the
// operations the contract marks "security: []".
var publicRoutes = map[string]bool{
	"GET /healthz":                   true,
	"GET /api/auth/config":           true,
	"GET /api/auth/login":            true,
	"POST /api/auth/login":           true,
	"GET /api/auth/callback":         true,
	"POST /api/auth/logout":          true,
	"POST /api/invitations/preview":  true,
	"POST /api/invitations/register": true,
}

// caller is what authentication learned about a request. Operations get no
// http.Request, so the cookies they need travel here.
type caller struct {
	principal    usecase.Principal // zero for public routes
	sessionToken string
	oidcState    string
}

func callerFrom(ctx context.Context) caller {
	if info, ok := ctx.Value(requestInfoKey{}).(*requestInfo); ok {
		return info.caller
	}
	return caller{}
}

// principalFrom returns the user or bot an operation acts for.
func principalFrom(ctx context.Context) usecase.Principal {
	return callerFrom(ctx).principal
}

// bearerToken returns the token of an Authorization: Bearer header and
// whether the header is there at all.
func bearerToken(r *http.Request) (token string, present bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", true
	}
	return strings.TrimSpace(token), true
}

// authenticate resolves the credentials of every route but the public ones
// and answers 401 without valid ones. It runs in front of the router, so
// nothing of a request is decoded for someone unknown. A bot token in the
// Authorization header (FR-035) and a session cookie are alternatives; a
// request with both gets 400. A session the use case extends gets its
// cookie extended too.
func authenticate(deps Deps, lim *limits, mux *http.ServeMux, fail func(http.ResponseWriter, *http.Request, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := caller{
			sessionToken: cookieValue(r, sessionCookieName),
			oidcState:    cookieValue(r, oidcCookieName),
		}
		// An empty pattern is a path the router has nothing under.
		if _, route := mux.Handler(r); route != "" && !publicRoutes[route] {
			token, bearer := bearerToken(r)
			switch {
			case bearer && c.sessionToken != "":
				fail(w, r, domain.ErrBadRequest)
				return
			case bearer:
				p, err := authenticateBot(r, deps, lim, w, token)
				if err != nil {
					fail(w, r, err)
					return
				}
				c.principal = p
			default:
				if deps.Authenticate == nil {
					fail(w, r, domain.ErrUnauthenticated)
					return
				}
				auth, err := deps.Authenticate.Do(r.Context(), c.sessionToken)
				if err != nil {
					fail(w, r, err)
					return
				}
				if auth.Renewed {
					http.SetCookie(w, sessionCookie(deps, c.sessionToken))
				}
				c.principal = auth.Principal
			}
		}
		if info, ok := r.Context().Value(requestInfoKey{}).(*requestInfo); ok {
			info.caller = c
		}
		next.ServeHTTP(w, r)
	})
}

// authenticateBot checks a bot token under the limits: an address with too
// many rejected tokens and a bot over its request rate get 429.
func authenticateBot(r *http.Request, deps Deps, lim *limits, w http.ResponseWriter, token string) (usecase.Principal, error) {
	address := limitKey(limitedClient(clientAddr(r, lim.trusted)))
	if blocked, retry := lim.bearerFailures.blocked(address); blocked {
		setRetryAfter(w, retry)
		return usecase.Principal{}, errRateLimited
	}
	if deps.AuthenticateBot == nil {
		return usecase.Principal{}, domain.ErrUnauthenticated
	}
	p, err := deps.AuthenticateBot.Do(r.Context(), token)
	if err != nil {
		lim.bearerFailures.allow(address)
		return usecase.Principal{}, err
	}
	if ok, retry := lim.botRequests.allow(limitKey(string(p.Bot.ID))); !ok {
		setRetryAfter(w, retry)
		return usecase.Principal{}, errRateLimited
	}
	return p, nil
}
