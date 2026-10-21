package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestSend(t *testing.T) {
	var path string
	var got sendMessage
	answer := `{"ok":true,"result":{}}`
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		w.Write([]byte(answer))
	}))
	defer srv.Close()
	n := &Notifier{APIURL: srv.URL}
	target := usecase.ChannelTarget{Config: map[string]string{"chat_id": "-100123"}, Secrets: map[string]string{"bot_token": "123:ABC"}}
	msg := usecase.OutgoingMessage{Title: "Brak płatności: Netflix", Body: "Nie znaleziono płatności.", URL: "https://denar.example.com/h/h1/subscriptions/charges",
		HouseholdName: "Dom"}
	if err := n.Send(context.Background(), target, msg); err != nil {
		t.Fatal(err)
	}
	if path != "/bot123:ABC/sendMessage" || got.ChatID != "-100123" || !got.DisableWebPagePreview ||
		got.Text != "Brak płatności: Netflix\n\nNie znaleziono płatności.\n\nhttps://denar.example.com/h/h1/subscriptions/charges\nGospodarstwo: Dom" {
		t.Errorf("path %s, message %+v", path, got)
	}
	status, answer = 400, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`
	if err := n.Send(context.Background(), target, msg); err == nil || err.Error() != "Telegram: Bad Request: chat not found" {
		t.Errorf("chat not found = %v", err)
	}
	status, answer = 200, `{"ok":false}`
	if err := n.Send(context.Background(), target, msg); err == nil {
		t.Error("ok:false passed")
	}
	srv.Close()
	err := n.Send(context.Background(), target, msg)
	if err == nil || strings.Contains(err.Error(), "123:ABC") {
		t.Errorf("closed = %v", err)
	}
}
