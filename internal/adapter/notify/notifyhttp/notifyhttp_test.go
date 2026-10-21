package notifyhttp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/usecase"
)

func get(t *testing.T, client *http.Client, log *slog.Logger, target string) ([]byte, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Do(client, log, req)
}

func TestDefaultClientRefusesLoopback(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	_, err := get(t, nil, nil, srv.URL+"/hook?token=secret")
	if !errors.Is(err, notification.ErrTargetForbidden) {
		t.Fatalf("default client: err = %v, want ErrTargetForbidden", err)
	}
	if hit {
		t.Fatal("the request reached the loopback server")
	}
	if strings.Contains(err.Error(), "token=secret") || !strings.Contains(err.Error(), "DENAR_NOTIFY_ALLOW_PRIVATE") {
		t.Errorf("message = %q", err)
	}

	// A name resolving to loopback is refused when connecting, not by its
	// spelling: the check runs on the resolved address.
	u, _ := url.Parse(srv.URL)
	_, err = get(t, nil, nil, "http://localhost:"+u.Port()+"/")
	if !errors.Is(err, notification.ErrTargetForbidden) || hit {
		t.Errorf("localhost: err = %v, hit = %v", err, hit)
	}
}

func TestOptInClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	for name, policy := range map[string]notification.TargetPolicy{
		"allow private": {AllowPrivate: true},
		"allowlist":     {Allowed: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}},
	} {
		body, err := get(t, NewClient(policy), nil, srv.URL)
		if err != nil || string(body) != "ok" {
			t.Errorf("%s: body %q, err %v", name, body, err)
		}
	}
	other := NewClient(notification.TargetPolicy{Allowed: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
	if _, err := get(t, other, nil, srv.URL); !errors.Is(err, notification.ErrTargetForbidden) {
		t.Errorf("allowlist of another network: %v", err)
	}
}

func TestClientIgnoresProxyAndRedirects(t *testing.T) {
	c := NewClient(notification.TargetPolicy{})
	if tr, ok := c.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Errorf("transport = %#v, want one without a proxy", c.Transport)
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	defer srv.Close()
	_, err := get(t, NewClient(notification.TargetPolicy{AllowPrivate: true}), nil, srv.URL)
	if err == nil || err.Error() != "HTTP 302" {
		t.Errorf("redirect: %v", err)
	}
}

func TestErrorsCarryNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"internal":"redis 10.0.0.7 version 7.2"}`))
	}))
	defer srv.Close()
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	body, err := get(t, NewClient(notification.TargetPolicy{AllowPrivate: true}), log, srv.URL+"/path/with-token")
	if err == nil || err.Error() != "HTTP 403" {
		t.Fatalf("err = %v, want HTTP 403", err)
	}
	if !strings.Contains(string(body), "redis") {
		t.Errorf("body for the notifier = %q", body)
	}
	if !strings.Contains(logs.String(), "redis 10.0.0.7") || strings.Contains(logs.String(), "with-token") {
		t.Errorf("log = %s", logs.String())
	}
}

func TestSnippet(t *testing.T) {
	if got := Snippet([]byte(strings.Repeat("ą", 300))); len([]rune(got)) != 201 {
		t.Errorf("long snippet has %d runes", len([]rune(got)))
	}
	if Snippet([]byte{0xff, 0xfe}) != "" || Snippet([]byte("  ")) != "" {
		t.Error("invalid or empty body gave a snippet")
	}
}

func TestTextHouseholdLineFollowsTheLocale(t *testing.T) {
	msg := usecase.OutgoingMessage{Title: "T", Body: "B", URL: "https://x/h", HouseholdName: "Dom"}
	if got := Text(msg); got != "T\n\nB\n\nhttps://x/h\nGospodarstwo: Dom" {
		t.Errorf("pl = %q", got)
	}
	msg.Locale = user.LocaleEN
	if got := Text(msg); got != "T\n\nB\n\nhttps://x/h\nHousehold: Dom" {
		t.Errorf("en = %q", got)
	}
}

func TestConnectionErrorsAreEnglish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := srv.URL
	srv.Close()
	_, err := get(t, NewClient(notification.TargetPolicy{AllowPrivate: true}), nil, target)
	if err == nil || !strings.HasPrefix(err.Error(), "connection failed") || strings.Contains(err.Error(), target) {
		t.Errorf("err = %v", err)
	}
}
