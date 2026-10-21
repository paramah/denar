// Package webhook sends notifications as JSON to a URL, signed with
// HMAC-SHA256 when the channel has a secret (FR-541, AD-508).
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/notify/notifyhttp"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// Headers of a webhook request.
const (
	SignatureHeader = "X-Denar-Signature"
	EventHeader     = "X-Denar-Event"
	DeliveryHeader  = "X-Denar-Delivery"
)

var _ usecase.Notifier = (*Notifier)(nil)

// Notifier posts to webhooks. Client nil means notifyhttp.DefaultClient.
type Notifier struct {
	Client *http.Client
	Log    *slog.Logger // nil: errors of the target are not logged
}

// Payload is the JSON body of a webhook request.
type Payload struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	URL        string            `json:"url"`
	Household  Household         `json:"household"`
	OccurredAt time.Time         `json:"occurred_at"`
	Data       map[string]string `json:"data"`
}

type Household struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Sign returns the signature header value of a body: sha256=<hex HMAC>.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (n *Notifier) Send(ctx context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	data := msg.Data
	if data == nil {
		data = map[string]string{}
	}
	body, err := json.Marshal(Payload{ID: msg.ID, Type: msg.Type, Title: msg.Title, Body: msg.Body, URL: msg.URL,
		Household: Household{ID: string(msg.HouseholdID), Name: msg.HouseholdName}, OccurredAt: msg.OccurredAt.UTC(), Data: data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.Config["url"], bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(EventHeader, msg.Type)
	req.Header.Set(DeliveryHeader, msg.ID)
	if secret := target.Secrets["secret"]; secret != "" {
		req.Header.Set(SignatureHeader, Sign(secret, body))
	}
	_, err = notifyhttp.Do(n.Client, n.Log, req)
	return err
}
