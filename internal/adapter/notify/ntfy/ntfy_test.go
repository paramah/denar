package ntfy

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.eiac.dev/tools/denar/internal/adapter/notify/notifyhttp"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// local may reach the test server on 127.0.0.1 (the default client may not).
var local = notifyhttp.NewClient(notification.TargetPolicy{AllowPrivate: true})

func TestSend(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got, body = r, string(b)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()
	msg := usecase.OutgoingMessage{ID: "d1", Title: "Przekroczona koperta: Jedzenie", Body: "Koperta jest na minusie.",
		URL: "https://denar.example.com/h/h1/budget/2026-10", HouseholdName: "Dom"}
	err := (&Notifier{Client: local}).Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"server": srv.URL + "/", "topic": "denar-dom"},
		Secrets: map[string]string{"token": "tk_abc"}}, msg)
	if err != nil {
		t.Fatal(err)
	}
	title, err := new(mime.WordDecoder).DecodeHeader(got.Header.Get("Title"))
	if got.Method != "POST" || got.URL.Path != "/denar-dom" || title != msg.Title || err != nil || got.Header.Get("Click") != msg.URL ||
		got.Header.Get("Authorization") != "Bearer tk_abc" || body != "Koperta jest na minusie.\nGospodarstwo: Dom" {
		t.Errorf("request = %s %s %v %q (title %q)", got.Method, got.URL, got.Header, body, title)
	}
	err = (&Notifier{Client: local}).Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"server": srv.URL, "topic": "t"}}, msg)
	if err != nil || got.Header.Get("Authorization") != "" {
		t.Errorf("without a token = %v, %v", err, got.Header)
	}
}

func TestSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
	}))
	defer srv.Close()
	err := (&Notifier{Client: local}).Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"server": srv.URL, "topic": "t"}},
		usecase.OutgoingMessage{Title: "x"})
	if err == nil || err.Error() != "HTTP 401" {
		t.Errorf("err = %v", err)
	}
}
