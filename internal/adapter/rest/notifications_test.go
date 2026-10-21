package rest

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestChannelSecretsNeverReturn: a secret goes in, only has_secret comes
// out, in every answer about the channel.
func TestChannelSecretsNeverReturn(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	c := w.as(h, "member")
	rec := c.do("POST", "/api/me/channels", `{"type":"telegram","name":"TG","config":{"chat_id":"42"},"secrets":{"bot_token":"123:SECRET"}}`)
	if rec.Code != 201 {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	var ch struct {
		ID        string `json:"id"`
		HasSecret bool   `json:"has_secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ch); err != nil || !ch.HasSecret {
		t.Fatalf("channel = %s", rec.Body)
	}
	for _, r := range []struct{ method, target string }{{"GET", "/api/me/channels"}, {"GET", "/api/me/channels/" + ch.ID},
		{"POST", "/api/me/channels/" + ch.ID + "/test"}} {
		rec := c.do(r.method, r.target, "")
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "SECRET") {
			t.Errorf("%s %s = %d %s", r.method, r.target, rec.Code, rec.Body)
		}
	}
	if sent := w.notifier.Messages(); len(sent) != 1 || sent[0].Target.Secrets["bot_token"] != "123:SECRET" {
		t.Errorf("the test did not use the secret: %+v", sent)
	}
	if strings.Contains(w.logs.String(), "SECRET") {
		t.Error("the secret is in the log")
	}
}
