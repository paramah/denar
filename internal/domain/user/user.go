// Package user models the people who sign in.
package user

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
)

// User is a human account. OIDCIssuer and OIDCSubject are set for an
// account linked to the identity provider; PasswordHash for one that signs
// in with a password. An account may have both or neither (an account of
// an invitation not yet accepted does not exist at all). InstanceAdmin is
// the bootstrap account; it may use the command line tools but sees no
// household it is not a member of. An inactive user cannot sign in.
type User struct {
	ID            domain.ID
	Email         string
	Name          string
	PasswordHash  string
	OIDCIssuer    string
	OIDCSubject   string
	InstanceAdmin bool
	Active        bool
	CreatedAt     time.Time
}

// Limits, in characters.
const (
	MaxEmailLength    = 254
	MaxNameLength     = 100
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

// NormalizeEmail returns the form emails are stored and compared in:
// trimmed and lower case.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks a normalised email, reporting on field.
func ValidateEmail(field, email string) error {
	if msg := checkEmail(email); msg != "" {
		return domain.ValidationError{Field: field, Message: msg}
	}
	return nil
}

// ValidateName checks a display name (trimmed by the caller).
func ValidateName(field, name string) error {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return domain.ValidationError{Field: field, Message: "must not be empty"}
	case n > MaxNameLength:
		return domain.ValidationError{Field: field, Message: "must be at most 100 characters"}
	case strings.ContainsFunc(name, Unprintable):
		return domain.ValidationError{Field: field, Message: "must not contain control characters"}
	}
	return nil
}

// ValidatePassword checks a new password: 12 to 128 characters, not only
// whitespace. The message never contains the password.
func ValidatePassword(field, pw string) error {
	n := utf8.RuneCountInString(pw)
	switch {
	case strings.TrimSpace(pw) == "":
		return domain.ValidationError{Field: field, Message: "must not be blank"}
	case n < MinPasswordLength:
		return domain.ValidationError{Field: field, Message: "must be at least 12 characters"}
	case n > MaxPasswordLength:
		return domain.ValidationError{Field: field, Message: "must be at most 128 characters"}
	}
	return nil
}

// ValidateNewUser checks the email (normalised) and name (trimmed) of an
// account, joining every problem.
func ValidateNewUser(email, name string) error {
	return errors.Join(ValidateEmail("email", email), ValidateName("name", name))
}

// Unprintable reports the characters no name or email may hold: control
// characters and those that change the direction of text.
func Unprintable(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)
}

func checkEmail(email string) string {
	if email == "" {
		return "must not be empty"
	}
	if utf8.RuneCountInString(email) > MaxEmailLength {
		return "must be at most 254 characters"
	}
	const invalid = "must be an email address"
	if strings.ContainsFunc(email, func(r rune) bool { return unicode.IsSpace(r) || Unprintable(r) }) {
		return invalid
	}
	local, host, found := strings.Cut(email, "@")
	if !found || local == "" || strings.Contains(host, "@") {
		return invalid
	}
	if !strings.Contains(host, ".") || slices.Contains(strings.Split(host, "."), "") {
		return invalid
	}
	return ""
}
