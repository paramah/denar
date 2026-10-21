package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/notify/notifyhttp"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// local may reach the test server on 127.0.0.1 (the default client may not).
var local = notifyhttp.NewClient(notification.TargetPolicy{AllowPrivate: true})

var msg = usecase.OutgoingMessage{ID: "d1", Type: "subscription.renewal_upcoming", Title: "Odnowienie: Netflix za 3 dni",
	Body: "Netflix odnowi się 31.10.2026: 43.00 PLN.", URL: "https://denar.example.com/h/h1/subscriptions/s1", HouseholdID: "h1",
	HouseholdName: "Dom", OccurredAt: time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC), Data: map[string]string{"amount": "43.00"}}

func TestSendSigned(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, body = r, must(io.ReadAll(r.Body))
		w.WriteHeader(204)
	}))
	defer srv.Close()
	err := (&Notifier{Client: local}).Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"url": srv.URL + "/hook"},
		Secrets: map[string]string{"secret": "s3cret"}}, msg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != "POST" || got.URL.Path != "/hook" || got.Header.Get("Content-Type") != "application/json" ||
		got.Header.Get(EventHeader) != msg.Type || got.Header.Get(DeliveryHeader) != "d1" {
		t.Errorf("request = %s %s %v", got.Method, got.URL, got.Header)
	}
	if sig := got.Header.Get(SignatureHeader); sig != Sign("s3cret", body) || !strings.HasPrefix(sig, "sha256=") || len(sig) != 71 {
		t.Errorf("signature = %q", sig)
	}
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "d1" || p.Title != msg.Title || p.Body != msg.Body || p.URL != msg.URL || p.Household != (Household{"h1", "Dom"}) ||
		!p.OccurredAt.Equal(msg.OccurredAt) || p.Data["amount"] != "43.00" {
		t.Errorf("payload = %+v", p)
	}
	if strings.Contains(string(body), "s3cret") {
		t.Error("the secret is in the body")
	}
}

func TestSendUnsignedAndErrors(t *testing.T) {
	status := 204
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig = r.Header.Get(SignatureHeader)
		w.WriteHeader(status)
		w.Write([]byte("nope"))
	}))
	defer srv.Close()
	target := usecase.ChannelTarget{Config: map[string]string{"url": srv.URL}}
	if err := (&Notifier{Client: local}).Send(context.Background(), target, msg); err != nil || sig != "" {
		t.Fatalf("unsigned = %v, signature %q", err, sig)
	}
	status = 500
	if err := (&Notifier{Client: local}).Send(context.Background(), target, msg); err == nil || err.Error() != "HTTP 500" {
		t.Errorf("500 = %v", err)
	}
	srv.Close()
	err := (&Notifier{Client: local}).Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"url": srv.URL + "/secret-path?token=x"}}, msg)
	if err == nil || strings.Contains(err.Error(), "secret-path") || strings.Contains(err.Error(), "token=x") {
		t.Errorf("closed server = %v", err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
