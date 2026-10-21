// Package matrix sends notifications to a Matrix room through the
// client-server API (FR-544).
package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"git.eiac.dev/tools/denar/internal/adapter/notify/notifyhttp"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.Notifier = (*Notifier)(nil)

// Notifier sends m.room.message events. Client nil means
// notifyhttp.DefaultClient.
type Notifier struct{ Client *http.Client }

type message struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

type matrixError struct {
	ErrCode string `json:"errcode"`
	Error   string `json:"error"`
}

// Send uses the message id as the transaction id, so that the homeserver
// drops a message sent twice.
func (n *Notifier) Send(ctx context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	body, err := json.Marshal(message{MsgType: "m.text", Body: notifyhttp.Text(msg)})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(target.Config["homeserver"], "/") + "/_matrix/client/v3/rooms/" + url.PathEscape(target.Config["room_id"]) +
		"/send/m.room.message/" + url.PathEscape(msg.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("niepoprawny adres serwera Matrix")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.Secrets["access_token"])
	raw, err := notifyhttp.Do(n.Client, req)
	if err != nil {
		var me matrixError
		if json.Unmarshal(raw, &me) == nil && me.ErrCode != "" {
			return errors.New("Matrix: " + me.ErrCode + " " + me.Error)
		}
	}
	return err
}
