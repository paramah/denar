package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestSend(t *testing.T) {
	var got *http.Request
	var m message
	status, answer := 200, `{"event_id":"$x"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		json.NewDecoder(r.Body).Decode(&m)
		w.WriteHeader(status)
		w.Write([]byte(answer))
	}))
	defer srv.Close()
	target := usecase.ChannelTarget{Config: map[string]string{"homeserver": srv.URL + "/", "room_id": "!room:example.org"},
		Secrets: map[string]string{"access_token": "syt_abc"}}
	msg := usecase.OutgoingMessage{ID: "d1", Title: "Zmiana ceny: Netflix", Body: "Od 01.11.2026 Netflix kosztuje 49.00 PLN."}
	if err := (&Notifier{}).Send(context.Background(), target, msg); err != nil {
		t.Fatal(err)
	}
	if got.Method != "PUT" || got.URL.EscapedPath() != "/_matrix/client/v3/rooms/%21room:example.org/send/m.room.message/d1" ||
		got.Header.Get("Authorization") != "Bearer syt_abc" || m.MsgType != "m.text" || m.Body != "Zmiana ceny: Netflix\n\nOd 01.11.2026 Netflix kosztuje 49.00 PLN." {
		t.Errorf("request = %s %s %v %+v", got.Method, got.URL.EscapedPath(), got.Header, m)
	}
	status, answer = 403, `{"errcode":"M_FORBIDDEN","error":"You are not in the room"}`
	if err := (&Notifier{}).Send(context.Background(), target, msg); err == nil || err.Error() != "Matrix: M_FORBIDDEN You are not in the room" {
		t.Errorf("forbidden = %v", err)
	}
}
