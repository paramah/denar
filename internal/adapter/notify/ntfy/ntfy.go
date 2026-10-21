// Package ntfy publishes notifications to an ntfy topic (FR-542).
package ntfy

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/paramah/denar/internal/adapter/notify/notifyhttp"
	"github.com/paramah/denar/internal/usecase"
)

var _ usecase.Notifier = (*Notifier)(nil)

// Notifier publishes to ntfy. Client nil means notifyhttp.DefaultClient.
type Notifier struct {
	Client *http.Client
	Log    *slog.Logger // nil: errors of the target are not logged
}

func (n *Notifier) Send(ctx context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	text := msg.Body
	if msg.HouseholdName != "" {
		text += "\nGospodarstwo: " + msg.HouseholdName
	}
	server := strings.TrimRight(target.Config["server"], "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/"+target.Config["topic"], strings.NewReader(text))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	// Header values are ASCII; ntfy decodes RFC 2047 (AD-512).
	req.Header.Set("Title", mime.BEncoding.Encode("utf-8", msg.Title))
	if msg.URL != "" {
		req.Header.Set("Click", msg.URL)
	}
	if token := target.Secrets["token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	_, err = notifyhttp.Do(n.Client, n.Log, req)
	return err
}
