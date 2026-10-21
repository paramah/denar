// Package notifyhttp holds what the HTTP notifiers share: the client that
// refuses private addresses, the text of a message and errors that never
// carry a URL (which may hold a token), a secret or the body of an answer.
package notifyhttp

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/usecase"
)

// UserAgent identifies Denar to the services it calls.
const UserAgent = "Denar-Notifier/1"

// Timeout bounds one notification request, connecting included.
const Timeout = 10 * time.Second

// NewClient returns the client notifiers send with: a 10 s timeout, no
// redirects followed (a redirect could carry a token elsewhere), no proxy
// from the environment (a proxy would connect for us, past the check) and
// a dialer that checks every address it connects to against policy, after
// DNS resolution, so a name that resolves, or later re-resolves, to a
// private address is refused too (009, SSRF).
func NewClient(policy notification.TargetPolicy) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return notification.ErrTargetForbidden
			}
			addr, err := netip.ParseAddr(host)
			if err != nil {
				return notification.ErrTargetForbidden
			}
			return policy.CheckAddr(addr)
		},
	}
	return &http.Client{
		Timeout: Timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// DefaultClient is the client of notifiers that get none: public
// addresses only.
var DefaultClient = NewClient(notification.TargetPolicy{})

// Text is the plain text of a message: title, body, link and household.
func Text(msg usecase.OutgoingMessage) string {
	var b strings.Builder
	b.WriteString(msg.Title)
	if msg.Body != "" {
		b.WriteString("\n\n" + msg.Body)
	}
	if msg.URL != "" {
		b.WriteString("\n\n" + msg.URL)
	}
	if msg.HouseholdName != "" {
		b.WriteString("\n" + notification.HouseholdLine(msg.Locale, msg.HouseholdName))
	}
	return b.String()
}

// Do sends the request and turns anything but a 2xx answer into a short
// error, "HTTP 503", that never carries the body of the answer: errors are
// shown to the person who set the channel up, and the body of whatever
// their URL points at must not reach them (009). The start of the body
// goes to log instead, with the host only (paths may hold tokens). A
// transport error loses its URL; a refused address says why. The body of
// any answer is returned, up to 64 KiB.
func Do(client *http.Client, log *slog.Logger, req *http.Request) ([]byte, error) {
	if client == nil {
		client = DefaultClient
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, notification.ErrTargetForbidden) {
			return nil, notification.ErrTargetForbidden
		}
		var uerr *url.Error
		if errors.As(err, &uerr) {
			if uerr.Timeout() {
				return nil, errors.New("connection timed out")
			}
			return nil, fmt.Errorf("connection failed: %v", uerr.Err)
		}
		return nil, errors.New("connection failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if log != nil {
			log.Warn("notification target answered with an error", "host", req.URL.Hostname(), "status", resp.StatusCode,
				"body", Snippet(body))
		}
		return body, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// Snippet is the start of a body for the log: at most 200 characters of
// valid UTF-8, "" otherwise.
func Snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}
