package rest

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain/user"
	"git.eiac.dev/tools/denar/internal/usecase"
)

const (
	// limitWindow is the sliding window every limit counts over.
	limitWindow = time.Minute
	// loginFailuresPerAccount password logins may fail per window for one
	// pair of client address and email; loginFailuresPerAddress for one
	// address over all emails (FR-011).
	loginFailuresPerAccount = 5
	loginFailuresPerAddress = 20
	// publicAttemptsPerAddress OIDC starts and invitation previews and
	// registrations may be made per window from one address.
	publicAttemptsPerAddress = 20
	// bearerFailuresPerAddress bot tokens may be rejected per window for one
	// address (NFR-001).
	bearerFailuresPerAddress = 20
	// defaultBotRate is the requests a bot may make per window.
	defaultBotRate = 120
	// maxLimitKeys caps a table, so a flood of made-up keys cannot grow it
	// without bound.
	maxLimitKeys = 50_000
	// sweepInterval is how often expired entries are dropped.
	sweepInterval = 10 * time.Second
)

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

// forgive takes back the newest event of the key: an attempt that turned
// out to succeed.
func (w *window) forgive(key [sha256.Size]byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch recent := w.events[key]; len(recent) {
	case 0:
	case 1:
		delete(w.events, key)
	default:
		w.events[key] = recent[:len(recent)-1]
	}
}

// evict drops an entry below the limit, so a flood of new keys does not
// hand a blocked key a fresh allowance; only when every sampled entry is
// at the limit does one of those go.
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

// limitedClient returns what a limit counts as one client: an IPv4 address,
// or the /64 of an IPv6 address.
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

func setRetryAfter(w http.ResponseWriter, d time.Duration) {
	seconds := int((d + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.Itoa(max(seconds, 1)))
}

// limits holds every rate limit of the adapter.
type limits struct {
	trusted        []netip.Prefix
	loginAccount   *window // failed logins per address and email
	loginAddress   *window // failed logins per address
	public         *window // OIDC starts, invitation previews and registrations per address
	bearerFailures *window // rejected bot tokens per address
	botRequests    *window // requests per bot
}

func newLimits(clock usecase.Clock, trusted []netip.Prefix, botRate int) *limits {
	if botRate <= 0 {
		botRate = defaultBotRate
	}
	return &limits{
		trusted:        trusted,
		loginAccount:   newWindow(loginFailuresPerAccount, clock),
		loginAddress:   newWindow(loginFailuresPerAddress, clock),
		public:         newWindow(publicAttemptsPerAddress, clock),
		bearerFailures: newWindow(bearerFailuresPerAddress, clock),
		botRequests:    newWindow(botRate, clock),
	}
}

// publicOperations are the unauthenticated operations limited per address.
var publicOperations = map[string]bool{
	"LoginStart":             true,
	"PreviewInvitation":      true,
	"RegisterWithInvitation": true,
}

// middleware applies the limits of the login and the public operations. A
// refused login gets 429 even with the right password, or the limit would
// tell a guesser when they got it right.
func (l *limits) middleware(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	switch {
	case operationID == "LoginLocal":
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			client := limitedClient(clientAddr(r, l.trusted))
			email := ""
			if login, ok := request.(api.LoginLocalRequestObject); ok && login.Body != nil {
				email = user.NormalizeEmail(login.Body.Email)
			}
			account, address := limitKey(client, email), limitKey(client)
			ok, retry := l.loginAccount.allow(account)
			if ok {
				if ok, retry = l.loginAddress.allow(address); !ok {
					l.loginAccount.forgive(account)
				}
			}
			if !ok {
				setRetryAfter(w, retry)
				return nil, errRateLimited
			}
			response, err := next(ctx, w, r, request)
			if err == nil {
				l.loginAccount.forgive(account)
				l.loginAddress.forgive(address)
			}
			return response, err
		}
	case publicOperations[operationID]:
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if ok, retry := l.public.allow(limitKey(operationID, limitedClient(clientAddr(r, l.trusted)))); !ok {
				setRetryAfter(w, retry)
				return nil, errRateLimited
			}
			return next(ctx, w, r, request)
		}
	}
	return next
}

// wallClock is the clock used when Deps has none.
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }
