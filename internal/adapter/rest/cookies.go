package rest

import (
	"net/http"
	"strings"
	"time"
)

const (
	// sessionCookieName is the name the contract gives the session cookie.
	sessionCookieName = "denar_session"
	// oidcCookieName holds the state of an OIDC login in progress, tying
	// the callback to the browser that started it.
	oidcCookieName = "denar_oidc"
	oidcCookiePath = "/api/auth"
	// oidcCookieTTL covers the time a started OIDC login may take.
	oidcCookieTTL = 10 * time.Minute
)

func newCookie(deps Deps, name, value, path string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   int(ttl / time.Second),
		HttpOnly: true,
		Secure:   strings.HasPrefix(deps.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

func sessionCookie(deps Deps, token string) *http.Cookie {
	return newCookie(deps, sessionCookieName, token, "/", deps.SessionTTL)
}

func oidcCookie(deps Deps, state string) *http.Cookie {
	return newCookie(deps, oidcCookieName, state, oidcCookiePath, oidcCookieTTL)
}

// expired returns the cookie that makes the browser drop c.
func expired(c *http.Cookie) *http.Cookie {
	c.Value, c.MaxAge = "", -1
	return c
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
