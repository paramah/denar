// Package notification models what people are told about events of their
// households: notifications in the app, the channels that carry them
// elsewhere (e-mail, webhook, ntfy, Telegram, Matrix), the preferences that
// choose between them and the deliveries waiting to be sent.
package notification

import (
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
)

// ChannelType is the kind of a channel.
type ChannelType string

const (
	Email    ChannelType = "email"
	Webhook  ChannelType = "webhook"
	Ntfy     ChannelType = "ntfy"
	Telegram ChannelType = "telegram"
	Matrix   ChannelType = "matrix"
)

// ChannelTypes are the channel types in a stable order.
var ChannelTypes = []ChannelType{Email, Webhook, Ntfy, Telegram, Matrix}

// Valid reports whether t is a known channel type.
func (t ChannelType) Valid() bool { return slices.Contains(ChannelTypes, t) }

// MaxChannels is how many channels one person may have.
const MaxChannels = 20

// DefaultNtfyServer is the ntfy server of a channel that names none.
const DefaultNtfyServer = "https://ntfy.sh"

// Field is a configuration or secret field of a channel type.
type Field struct {
	Name     string
	Required bool
}

// TypeSpec says which fields a channel type takes.
type TypeSpec struct {
	Config  []Field
	Secrets []Field
}

// NeedsSecret reports whether the type cannot work without a secret.
func (s TypeSpec) NeedsSecret() bool {
	return slices.ContainsFunc(s.Secrets, func(f Field) bool { return f.Required })
}

var specs = map[ChannelType]TypeSpec{
	Email:    {Config: []Field{{"address", false}}},
	Webhook:  {Config: []Field{{"url", true}}, Secrets: []Field{{"secret", false}}},
	Ntfy:     {Config: []Field{{"server", false}, {"topic", true}}, Secrets: []Field{{"token", false}}},
	Telegram: {Config: []Field{{"chat_id", true}}, Secrets: []Field{{"bot_token", true}}},
	Matrix:   {Config: []Field{{"homeserver", true}, {"room_id", true}}, Secrets: []Field{{"access_token", true}}},
}

// Spec returns the fields of the channel type.
func Spec(t ChannelType) TypeSpec { return specs[t] }

// Channel is a place a person gets notifications, other than the app.
type Channel struct {
	ID     domain.ID
	UserID domain.ID
	Type   ChannelType
	Name   string
	// Config holds the fields that are not secret.
	Config map[string]string
	// SealedSecrets is the secrets object, encrypted; nil without secrets.
	SealedSecrets []byte
	Enabled       bool
	// VerifiedAt is when a message last went through; zero before.
	VerifiedAt time.Time
	// LastError is the reason the last message failed for good, or the
	// last test failed; "" after a success.
	LastError   string
	LastErrorAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// HasSecret reports whether the channel keeps secrets.
func (c Channel) HasSecret() bool { return len(c.SealedSecrets) > 0 }

// ValidateName checks a channel name: 1 to 100 characters after trimming.
func ValidateName(field, name string) error {
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return domain.ValidationError{Field: field, Message: "must have 1 to 100 characters"}
	}
	return nil
}

var (
	topicPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	maxValueLen  = 2000
)

// NormalizeConfig checks the configuration of a channel type and returns
// it trimmed, with defaults filled in: an e-mail channel without an
// address gets defaultEmail, an ntfy channel without a server ntfy.sh.
func NormalizeConfig(t ChannelType, config map[string]string, defaultEmail string) (map[string]string, error) {
	if !t.Valid() {
		return nil, domain.ValidationError{Field: "type", Message: "must be one of email, webhook, ntfy, telegram, matrix"}
	}
	spec := specs[t]
	out := map[string]string{}
	for k, v := range config {
		if !slices.ContainsFunc(spec.Config, func(f Field) bool { return f.Name == k }) {
			return nil, domain.ValidationError{Field: "config." + k, Message: "is not a setting of a " + string(t) + " channel"}
		}
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) > maxValueLen {
			return nil, domain.ValidationError{Field: "config." + k, Message: "is too long"}
		}
		if v != "" {
			out[k] = v
		}
	}
	switch t {
	case Email:
		if out["address"] == "" {
			out["address"] = defaultEmail
		}
		a, err := mail.ParseAddress(out["address"])
		if err != nil || a.Address != out["address"] {
			return nil, domain.ValidationError{Field: "config.address", Message: "must be an e-mail address"}
		}
	case Ntfy:
		if out["server"] == "" {
			out["server"] = DefaultNtfyServer
		}
		out["server"] = strings.TrimRight(out["server"], "/")
		if !httpURL(out["server"]) {
			return nil, domain.ValidationError{Field: "config.server", Message: "must be an http(s) URL"}
		}
	case Matrix:
		out["homeserver"] = strings.TrimRight(out["homeserver"], "/")
	}
	for _, f := range spec.Config {
		if f.Required && out[f.Name] == "" {
			return nil, domain.ValidationError{Field: "config." + f.Name, Message: "is required"}
		}
	}
	switch t {
	case Webhook:
		if !httpURL(out["url"]) {
			return nil, domain.ValidationError{Field: "config.url", Message: "must be an http(s) URL"}
		}
	case Ntfy:
		if !topicPattern.MatchString(out["topic"]) {
			return nil, domain.ValidationError{Field: "config.topic", Message: "must have 1 to 64 letters, digits, - or _"}
		}
	case Telegram:
		if len(out["chat_id"]) > 64 || strings.ContainsAny(out["chat_id"], " /?#") {
			return nil, domain.ValidationError{Field: "config.chat_id", Message: "must be a chat id such as 123456789 or @channel"}
		}
	case Matrix:
		if !httpURL(out["homeserver"]) {
			return nil, domain.ValidationError{Field: "config.homeserver", Message: "must be an http(s) URL"}
		}
		if !strings.HasPrefix(out["room_id"], "!") || !strings.Contains(out["room_id"], ":") {
			return nil, domain.ValidationError{Field: "config.room_id", Message: "must be a room id such as !abc:example.org"}
		}
	}
	return out, nil
}

// CheckSecrets checks the secrets given for a channel type and returns
// them without empty values. With complete set, required secrets must be
// there.
func CheckSecrets(t ChannelType, secrets map[string]string, complete bool) (map[string]string, error) {
	spec := specs[t]
	out := map[string]string{}
	for k, v := range secrets {
		if !slices.ContainsFunc(spec.Secrets, func(f Field) bool { return f.Name == k }) {
			return nil, domain.ValidationError{Field: "secrets." + k, Message: "is not a secret of a " + string(t) + " channel"}
		}
		v = strings.TrimSpace(v)
		if len(v) > maxValueLen {
			return nil, domain.ValidationError{Field: "secrets." + k, Message: "is too long"}
		}
		if v != "" {
			out[k] = v
		}
	}
	if complete {
		for _, f := range spec.Secrets {
			if f.Required && out[f.Name] == "" {
				return nil, domain.ValidationError{Field: "secrets." + f.Name, Message: "is required"}
			}
		}
	}
	return out, nil
}

func httpURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
