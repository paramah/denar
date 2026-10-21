package fakes

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"sync"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// SecretBox seals values reversibly, bound to their aad; it is no
// encryption, only the contract of usecase.SecretBox.
type SecretBox struct{}

func (SecretBox) Seal(plaintext, aad []byte) ([]byte, error) {
	return []byte("sealed|" + base64.StdEncoding.EncodeToString(aad) + "|" + base64.StdEncoding.EncodeToString(plaintext)), nil
}

func (SecretBox) Open(sealed, aad []byte) ([]byte, error) {
	parts := bytes.Split(sealed, []byte("|"))
	if len(parts) != 3 || string(parts[0]) != "sealed" || string(parts[1]) != base64.StdEncoding.EncodeToString(aad) {
		return nil, errors.New("cannot open")
	}
	return base64.StdEncoding.DecodeString(string(parts[2]))
}

// SentMessage is one message a fake Notifier was asked to send.
type SentMessage struct {
	Target  usecase.ChannelTarget
	Message usecase.OutgoingMessage
}

// Notifier records what it sends. Errs are returned by the next calls,
// one each, before it succeeds again.
type Notifier struct {
	mu   sync.Mutex
	Errs []error
	Sent []SentMessage
}

func (n *Notifier) Send(_ context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.Errs) > 0 {
		err := n.Errs[0]
		n.Errs = n.Errs[1:]
		return err
	}
	n.Sent = append(n.Sent, SentMessage{Target: target, Message: msg})
	return nil
}

// Messages returns what was sent so far.
func (n *Notifier) Messages() []SentMessage {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]SentMessage(nil), n.Sent...)
}

// Mail is one e-mail a fake Mailer sent.
type Mail struct{ To, Subject, Body string }

// Mailer records e-mail; Err makes it fail.
type Mailer struct {
	mu   sync.Mutex
	Err  error
	Sent []Mail
}

func (m *Mailer) SendMail(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.Sent = append(m.Sent, Mail{To: to, Subject: subject, Body: body})
	return nil
}
