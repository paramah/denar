package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/notification"
)

// sendTimeout bounds one message to a channel (NFR-051).
const sendTimeout = 10 * time.Second

// ChannelView is a channel without its sealed secrets.
type ChannelView struct {
	notification.Channel
	HasSecret bool
}

func channelView(c notification.Channel) ChannelView {
	v := ChannelView{Channel: c, HasSecret: c.HasSecret()}
	v.SealedSecrets = nil
	return v
}

// Reasons a channel type is unavailable.
const (
	UnavailableSMTP    = "smtp_not_configured"
	UnavailableSecrets = "secrets_unavailable"
)

// ChannelTypeInfo says whether the server can send over a channel type
// and what the type takes.
type ChannelTypeInfo struct {
	Type      notification.ChannelType
	Available bool
	// UnavailableReason is UnavailableSMTP or UnavailableSecrets; "" when
	// available.
	UnavailableReason string
	Spec              notification.TypeSpec
}

// unavailable says why the server cannot use the channel type, or "".
func (d Deps) unavailable(t notification.ChannelType) string {
	if t == notification.Email && (d.Mailer == nil || d.Notifiers[t] == nil) {
		return UnavailableSMTP
	}
	if notification.Spec(t).NeedsSecret() && d.Secrets == nil {
		return UnavailableSecrets
	}
	if d.Notifiers[t] == nil {
		return UnavailableSMTP
	}
	return ""
}

func unavailableError(reason string) error {
	if reason == UnavailableSecrets {
		return domain.ConflictError{Reason: domain.ConflictSecretsUnavailable}
	}
	return domain.ConflictError{Reason: domain.ConflictSMTPNotConfigured}
}

// ListChannelTypes tells which channel types the server can use (AD-515).
type ListChannelTypes struct{ d Deps }

func NewListChannelTypes(d Deps) *ListChannelTypes { return &ListChannelTypes{d: d} }

func (uc *ListChannelTypes) Do(_ context.Context, p Principal) ([]ChannelTypeInfo, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	out := make([]ChannelTypeInfo, len(notification.ChannelTypes))
	for i, t := range notification.ChannelTypes {
		reason := uc.d.unavailable(t)
		out[i] = ChannelTypeInfo{Type: t, Available: reason == "", UnavailableReason: reason, Spec: notification.Spec(t)}
	}
	return out, nil
}

// sealSecrets encrypts the secrets of a channel bound to its id; no
// secrets give nil.
func (d Deps) sealSecrets(id domain.ID, secrets map[string]string) ([]byte, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	if d.Secrets == nil {
		return nil, domain.ConflictError{Reason: domain.ConflictSecretsUnavailable}
	}
	raw, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	return d.Secrets.Seal(raw, []byte(id))
}

// errNoMasterKey and errSecretsUnreadable are the reasons a channel with
// secrets cannot be used.
var (
	errNoMasterKey       = errors.New("brak klucza DENAR_MASTER_KEY do odczytu sekretów kanału")
	errSecretsUnreadable = errors.New("nie można odczytać sekretów kanału (zmieniony DENAR_MASTER_KEY?) — ustaw je ponownie")
)

// openSecrets decrypts the secrets of a channel.
func (d Deps) openSecrets(c notification.Channel) (map[string]string, error) {
	if !c.HasSecret() {
		return map[string]string{}, nil
	}
	if d.Secrets == nil {
		return nil, errNoMasterKey
	}
	raw, err := d.Secrets.Open(c.SealedSecrets, []byte(c.ID))
	if err != nil {
		return nil, errSecretsUnreadable
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, errSecretsUnreadable
	}
	return out, nil
}

// errPermanent marks a failure retrying cannot fix.
type errPermanent struct{ error }

// send sends a message over a channel. A failure that retrying cannot fix
// (no notifier, no key) is an errPermanent.
func (d Deps) send(ctx context.Context, c notification.Channel, msg OutgoingMessage) error {
	if reason := d.unavailable(c.Type); reason != "" {
		if reason == UnavailableSecrets {
			return errPermanent{errNoMasterKey}
		}
		return errPermanent{errors.New("serwer nie ma skonfigurowanego SMTP")}
	}
	secrets, err := d.openSecrets(c)
	if err != nil {
		return errPermanent{err}
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return d.Notifiers[c.Type].Send(ctx, ChannelTarget{Type: c.Type, Config: c.Config, Secrets: secrets}, msg)
}

// errorText shortens an error for storing on a channel or a delivery.
func errorText(err error) string {
	s := err.Error()
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// ListChannels returns the person's channels by name; people only.
type ListChannels struct{ d Deps }

func NewListChannels(d Deps) *ListChannels { return &ListChannels{d: d} }

func (uc *ListChannels) Do(ctx context.Context, p Principal) ([]ChannelView, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	all, err := uc.d.Channels.ListByUser(ctx, p.User.ID)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelView, len(all))
	for i, c := range all {
		out[i] = channelView(c)
	}
	return out, nil
}

// GetChannel returns one of the person's channels.
type GetChannel struct{ d Deps }

func NewGetChannel(d Deps) *GetChannel { return &GetChannel{d: d} }

func (uc *GetChannel) Do(ctx context.Context, p Principal, id domain.ID) (ChannelView, error) {
	if err := requireHuman(p); err != nil {
		return ChannelView{}, err
	}
	c, err := uc.d.Channels.ByID(ctx, p.User.ID, id)
	if err != nil {
		return ChannelView{}, plainNotFound(err)
	}
	return channelView(c), nil
}

type CreateChannelInput struct {
	Type    notification.ChannelType
	Name    string
	Config  map[string]string
	Secrets map[string]string
	Enabled domain.Optional[bool]
}

// CreateChannel adds a channel of the person (FR-530…FR-534).
type CreateChannel struct{ d Deps }

func NewCreateChannel(d Deps) *CreateChannel { return &CreateChannel{d: d} }

func (uc *CreateChannel) Do(ctx context.Context, p Principal, in CreateChannelInput) (ChannelView, error) {
	if err := requireHuman(p); err != nil {
		return ChannelView{}, err
	}
	name := strings.TrimSpace(in.Name)
	config, err := notification.NormalizeConfig(in.Type, in.Config, p.User.Email)
	if err = errors.Join(notification.ValidateName("name", name), err); err != nil {
		return ChannelView{}, err
	}
	secrets, err := notification.CheckSecrets(in.Type, in.Secrets, true)
	if err != nil {
		return ChannelView{}, err
	}
	if reason := uc.d.unavailable(in.Type); reason != "" {
		return ChannelView{}, unavailableError(reason)
	}
	now := uc.d.Clock.Now()
	c := notification.Channel{ID: uc.d.IDs.NewID(), UserID: p.User.ID, Type: in.Type, Name: name, Config: config, Enabled: true,
		CreatedAt: now, UpdatedAt: now}
	if in.Enabled.Set {
		c.Enabled = in.Enabled.Value
	}
	if c.SealedSecrets, err = uc.d.sealSecrets(c.ID, secrets); err != nil {
		return ChannelView{}, err
	}
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		all, err := uc.d.Channels.ListByUser(ctx, p.User.ID)
		if err != nil {
			return err
		}
		if len(all) >= notification.MaxChannels {
			return domain.ConflictError{Reason: domain.ConflictTooManyChannels}
		}
		return uc.d.Channels.Create(ctx, c)
	})
	if err != nil {
		return ChannelView{}, err
	}
	return channelView(c), nil
}

type UpdateChannelInput struct {
	Name   domain.Optional[string]
	Config domain.Optional[map[string]string]
	// Secrets replaces every secret when set; set to nil it clears them
	// (AD-511).
	Secrets domain.Optional[map[string]string]
	Enabled domain.Optional[bool]
}

// UpdateChannel changes a channel of the person. A new configuration or
// new secrets clear what was known about the channel working.
type UpdateChannel struct{ d Deps }

func NewUpdateChannel(d Deps) *UpdateChannel { return &UpdateChannel{d: d} }

func (uc *UpdateChannel) Do(ctx context.Context, p Principal, id domain.ID, in UpdateChannelInput) (ChannelView, error) {
	if err := requireHuman(p); err != nil {
		return ChannelView{}, err
	}
	var out notification.Channel
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		c, err := uc.d.Channels.ByID(ctx, p.User.ID, id)
		if err != nil {
			return plainNotFound(err)
		}
		changed := false
		if in.Name.Set {
			c.Name = strings.TrimSpace(in.Name.Value)
			if err := notification.ValidateName("name", c.Name); err != nil {
				return err
			}
		}
		if in.Config.Set {
			if c.Config, err = notification.NormalizeConfig(c.Type, in.Config.Value, p.User.Email); err != nil {
				return err
			}
			changed = true
		}
		if in.Secrets.Set {
			secrets, err := notification.CheckSecrets(c.Type, in.Secrets.Value, true)
			if err != nil {
				return err
			}
			if c.SealedSecrets, err = uc.d.sealSecrets(c.ID, secrets); err != nil {
				return err
			}
			changed = true
		}
		if in.Enabled.Set {
			c.Enabled = in.Enabled.Value
		}
		if changed {
			c.VerifiedAt, c.LastError, c.LastErrorAt = time.Time{}, "", time.Time{}
		}
		c.UpdatedAt = uc.d.Clock.Now()
		if err := uc.d.Channels.Update(ctx, c); err != nil {
			return err
		}
		out = c
		return nil
	})
	if err != nil {
		return ChannelView{}, err
	}
	return channelView(out), nil
}

// DeleteChannel removes a channel of the person with its deliveries.
type DeleteChannel struct{ d Deps }

func NewDeleteChannel(d Deps) *DeleteChannel { return &DeleteChannel{d: d} }

func (uc *DeleteChannel) Do(ctx context.Context, p Principal, id domain.ID) error {
	if err := requireHuman(p); err != nil {
		return err
	}
	return plainNotFound(uc.d.Channels.Delete(ctx, p.User.ID, id))
}

// ChannelTestResult is the outcome of a test message.
type ChannelTestResult struct {
	OK       bool
	Error    string
	TestedAt time.Time
}

// TestChannel sends a test message over a channel of the person, now, and
// records the outcome on the channel (FR-535).
type TestChannel struct{ d Deps }

func NewTestChannel(d Deps) *TestChannel { return &TestChannel{d: d} }

func (uc *TestChannel) Do(ctx context.Context, p Principal, id domain.ID) (ChannelTestResult, error) {
	if err := requireHuman(p); err != nil {
		return ChannelTestResult{}, err
	}
	c, err := uc.d.Channels.ByID(ctx, p.User.ID, id)
	if err != nil {
		return ChannelTestResult{}, plainNotFound(err)
	}
	link := uc.d.BaseURL
	sendErr := uc.d.send(ctx, c, OutgoingMessage{ID: string(uc.d.IDs.NewID()), Type: "test", Title: "Test kanału Denar",
		Body: "Ten kanał (" + c.Name + ") działa: tu będą trafiać powiadomienia Denara.", URL: link, OccurredAt: uc.d.Clock.Now(),
		Data: map[string]string{}})
	now := uc.d.Clock.Now()
	res := ChannelTestResult{OK: sendErr == nil, TestedAt: now}
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := uc.d.Channels.ByID(ctx, p.User.ID, id)
		if found, err := ignoreNotFound(err); err != nil || !found {
			return err
		}
		recordOutcome(&cur, sendErr, now)
		return uc.d.Channels.Update(ctx, cur)
	})
	if sendErr != nil {
		res.Error = errorText(sendErr)
	}
	return res, err
}

// recordOutcome stores on a channel that a message went through, or why
// it did not.
func recordOutcome(c *notification.Channel, err error, now time.Time) {
	if err == nil {
		c.VerifiedAt, c.LastError, c.LastErrorAt = now, "", time.Time{}
		return
	}
	c.LastError, c.LastErrorAt = errorText(err), now
}
