// Package notifyhttp holds what the HTTP notifiers share: the client, the
// text of a message and errors that never carry a URL (which may hold a
// token) or a secret.
package notifyhttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// UserAgent identifies Denar to the services it calls.
const UserAgent = "Denar-Notifier/1"

// DefaultClient is the client of notifiers that get none: a 10 s timeout,
// no redirects followed (a redirect could carry a token elsewhere).
var DefaultClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

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
		b.WriteString("\nGospodarstwo: " + msg.HouseholdName)
	}
	return b.String()
}

// Do sends the request and turns anything but a 2xx answer into a short
// error: "HTTP 503: <start of the body>". A transport error loses its URL.
// The body of a success is returned, up to 64 KiB.
func Do(client *http.Client, req *http.Request) ([]byte, error) {
	if client == nil {
		client = DefaultClient
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			if uerr.Timeout() {
				return nil, errors.New("przekroczony czas połączenia")
			}
			return nil, fmt.Errorf("błąd połączenia: %v", uerr.Err)
		}
		return nil, errors.New("błąd połączenia")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return body, fmt.Errorf("HTTP %d%s", resp.StatusCode, snippet(body))
	}
	return body, nil
}

func snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return ": " + s
}
