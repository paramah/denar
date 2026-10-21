package mcp

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

const (
	// limitWindow is the sliding window every limit counts over.
	limitWindow = time.Minute
	// bearerFailuresPerAddress rejected tokens per window from one client
	// address are answered 401; after that the address gets 429 until the
	// window moves on (the same limit as REST).
	bearerFailuresPerAddress = 20
	// maxLimitKeys caps a table, so made-up keys cannot grow it without
	// bound.
	maxLimitKeys = 50_000
	// sweepInterval is how often expired entries are dropped.
	sweepInterval = 10 * time.Second
	// extraPrincipal is where the principal travels in TokenInfo.Extra.
	extraPrincipal = "principal"
)

type principalKey struct{}

// bearerToken returns the token of an "Authorization: Bearer <token>"
// header. The header is the only place a token is read from.
func bearerToken(r *http.Request) (string, bool) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return "", false
	}
	return fields[1], true
}

// unauthorized is the one answer to every failed authentication: a missing
// token, an unknown or replaced one and the token of a disabled or expired
// bot look the same.
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}` + "\n"))
}

func tooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int((retryAfter + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.Itoa(max(seconds, 1)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too_many_requests"}` + "\n"))
}

// authenticate resolves the bearer token to its bot with the use case REST
// uses too, and keeps the principal for the rest of the request. An
// address with too many rejected tokens in the window is answered 429
// before its token is looked at.
func (s *server) authenticate(next http.Handler) http.Handler {
	failures := newWindow(bearerFailuresPerAddress, s.clock)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address := limitKey(limitedClient(clientAddr(r, s.deps.TrustedProxies)))
		if blocked, retry := failures.blocked(address); blocked {
			tooManyRequests(w, retry)
			return
		}
		var (
			p   usecase.Principal
			err error = domain.ErrUnauthenticated
		)
		if token, ok := bearerToken(r); ok && s.deps.Authenticate != nil {
			p, err = s.deps.Authenticate.Do(r.Context(), token)
		}
		switch {
		case err == nil:
		case errors.Is(err, domain.ErrUnauthenticated):
			failures.allow(address)
			unauthorized(w)
			return
		default:
			s.log.LogAttrs(r.Context(), slog.LevelError, "authentication failed", slog.String("error", err.Error()))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		noteBot(r.Context(), p.Bot.ID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

// limitRequests holds each bot to its rate: RatePerMinute requests in a
// sliding minute, as REST does with DENAR_BOT_RATE (AD-607).
func (s *server) limitRequests(next http.Handler) http.Handler {
	requests := newWindow(s.deps.RatePerMinute, s.clock)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := r.Context().Value(principalKey{}).(usecase.Principal)
		if ok, retry := requests.allow(limitKey(string(p.Bot.ID))); !ok {
			tooManyRequests(w, retry)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// verifiedPrincipal is the token verifier of the SDK middleware. The
// request is authenticated by then; it only passes the principal on.
func verifiedPrincipal(ctx context.Context, _ string, _ *http.Request) (*auth.TokenInfo, error) {
	p, ok := ctx.Value(principalKey{}).(usecase.Principal)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{UserID: string(p.Bot.ID), Extra: map[string]any{extraPrincipal: p}}, nil
}

// principalOf returns the bot a tool call is made by.
func principalOf(req *sdk.CallToolRequest) (usecase.Principal, bool) {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return usecase.Principal{}, false
	}
	p, ok := req.Extra.TokenInfo.Extra[extraPrincipal].(usecase.Principal)
	return p, ok && p.IsBot()
}

// window counts events per key over a sliding limitWindow, in memory and
// per process. Events it refuses are not counted.
type window struct {
	limit int
	clock usecase.Clock

	mu     sync.Mutex
	events map[[sha256.Size]byte][]time.Time // allowed events, oldest first
	swept  time.Time
}

func newWindow(limit int, clock usecase.Clock) *window {
	return &window{limit: limit, clock: clock, events: map[[sha256.Size]byte][]time.Time{}}
}

func limitKey(parts ...string) [sha256.Size]byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))
	return key
}

// allow records an event and reports whether it may proceed; when not,
// retryAfter is how long until the next one will.
func (w *window) allow(key [sha256.Size]byte) (ok bool, retryAfter time.Duration) {
	now := w.clock.Now()
	cutoff := now.Add(-limitWindow)
	w.mu.Lock()
	defer w.mu.Unlock()
	if now.Sub(w.swept) >= sweepInterval {
		for k, recent := range w.events {
			if !recent[len(recent)-1].After(cutoff) {
				delete(w.events, k)
			}
		}
		w.swept = now
	}
	recent, known := w.events[key]
	for len(recent) > 0 && !recent[0].After(cutoff) {
		recent = recent[1:]
	}
	if len(recent) >= w.limit {
		return false, recent[0].Sub(cutoff)
	}
	if !known && len(w.events) >= maxLimitKeys {
		w.evict()
	}
	w.events[key] = append(recent, now)
	return true, 0
}

// blocked reports whether the key is at its limit, without counting.
func (w *window) blocked(key [sha256.Size]byte) (bool, time.Duration) {
	cutoff := w.clock.Now().Add(-limitWindow)
	w.mu.Lock()
	defer w.mu.Unlock()
	recent := w.events[key]
	for len(recent) > 0 && !recent[0].After(cutoff) {
		recent = recent[1:]
	}
	if len(recent) >= w.limit {
		return true, recent[0].Sub(cutoff)
	}
	return false, 0
}

// evict drops an entry below the limit, so a flood of new keys does not
// hand a blocked key a fresh allowance.
func (w *window) evict() {
	const sample = 32
	var fallback [sha256.Size]byte
	seen := 0
	for key, recent := range w.events {
		if len(recent) < w.limit {
			delete(w.events, key)
			return
		}
		fallback = key
		if seen++; seen == sample {
			break
		}
	}
	delete(w.events, fallback)
}

// limitedClient returns what a limit counts as one client: an IPv4
// address, or the /64 of an IPv6 address.
func limitedClient(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	network, _ := addr.Prefix(64)
	return network.String()
}

// clientAddr returns the address a request came from. That is the peer's
// address, unless the peer is a trusted proxy: then X-Forwarded-For is read
// from the right, where the proxies append, and the first address that is
// not a trusted proxy is the client. A header from an untrusted peer is
// ignored. The REST adapter reads the address the same way; adapters do
// not import each other.
func clientAddr(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	client := peer.Unmap()
	if !isTrusted(client, trusted) {
		return client.String()
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, ok := parseHop(hops[i])
		if !ok {
			break
		}
		client = hop
		if !isTrusted(hop, trusted) {
			break
		}
	}
	return client.String()
}

func parseHop(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(s); err == nil {
		return addrPort.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
