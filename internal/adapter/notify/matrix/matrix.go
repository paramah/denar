// Package matrix sends notifications to a Matrix room through the
// client-server API (FR-544).
package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"git.eiac.dev/tools/denar/internal/adapter/notify/notifyhttp"
	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.Notifier = (*Notifier)(nil)

var errCode = regexp.MustCompile(`^M_[A-Z_]{1,64}$`)

// Notifier sends m.room.message events. Client nil means
// notifyhttp.DefaultClient.
type Notifier struct {
	Client *http.Client
	Log    *slog.Logger // nil: errors of the target are not logged
}

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
	raw, err := notifyhttp.Do(n.Client, n.Log, req)
	if err != nil {
		// Only the error code, which the specification limits to M_*
		// names: the homeserver is the person's URL, and free text from
		// it would tell them what answers there (009).
		var me matrixError
		if json.Unmarshal(raw, &me) == nil && errCode.MatchString(me.ErrCode) {
			return errors.New("Matrix: " + err.Error() + " " + me.ErrCode)
		}
	}
	return err
}
