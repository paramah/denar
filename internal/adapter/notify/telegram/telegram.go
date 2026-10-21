// Package telegram sends notifications through the Telegram Bot API
// method sendMessage (FR-543).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/paramah/denar/internal/adapter/notify/notifyhttp"
	"github.com/paramah/denar/internal/usecase"
)

// DefaultAPIURL is the Bot API of Telegram.
const DefaultAPIURL = "https://api.telegram.org"

var _ usecase.Notifier = (*Notifier)(nil)

// Notifier calls the Bot API at APIURL ("" means DefaultAPIURL). Client
// nil means notifyhttp.DefaultClient.
type Notifier struct {
	APIURL string
	Client *http.Client
	Log    *slog.Logger // nil: errors of the API are not logged
}

type sendMessage struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type answer struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (n *Notifier) Send(ctx context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	base := strings.TrimRight(n.APIURL, "/")
	if base == "" {
		base = DefaultAPIURL
	}
	body, err := json.Marshal(sendMessage{ChatID: target.Config["chat_id"], Text: notifyhttp.Text(msg), DisableWebPagePreview: true})
	if err != nil {
		return err
	}
	// The token is part of the path; notifyhttp keeps URLs out of errors.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/bot"+target.Secrets["bot_token"]+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid Bot API URL")
	}
	req.Header.Set("Content-Type", "application/json")
	raw, err := notifyhttp.Do(n.Client, n.Log, req)
	var a answer
	if json.Unmarshal(raw, &a) == nil && !a.OK && a.Description != "" {
		return errors.New("Telegram: " + a.Description)
	}
	if err != nil {
		return err
	}
	if !a.OK {
		return errors.New("Telegram: response without ok")
	}
	return nil
}
