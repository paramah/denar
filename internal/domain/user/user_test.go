package user

import (
	"errors"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Ada@Example.COM\n"); got != "ada@example.com" {
		t.Errorf("got %q", got)
	}
}

func message(err error) string {
	var ve domain.ValidationError
	if errors.As(err, &ve) {
		return ve.Message
	}
	return ""
}

func TestValidateEmail(t *testing.T) {
	tests := []struct{ in, msg string }{
		{"ada@example.com", ""},
		{"", "must not be empty"},
		{"ada", "must be an email address"},
		{"ada@example", "must be an email address"},
		{"ada@@example.com", "must be an email address"},
		{"a da@example.com", "must be an email address"},
		{"ada@example..com", "must be an email address"},
		{strings.Repeat("a", 250) + "@x.pl", "must be at most 254 characters"},
	}
	for _, tt := range tests {
		if got := message(ValidateEmail("email", tt.in)); got != tt.msg {
			t.Errorf("ValidateEmail(%q) = %q, want %q", tt.in, got, tt.msg)
		}
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct{ in, msg string }{
		{"Ada", ""},
		{"", "must not be empty"},
		{strings.Repeat("ż", 101), "must be at most 100 characters"},
		{"A‮da", "must not contain control characters"},
		{"A\x00", "must not contain control characters"},
	}
	for _, tt := range tests {
		if got := message(ValidateName("name", tt.in)); got != tt.msg {
			t.Errorf("ValidateName(%q) = %q, want %q", tt.in, got, tt.msg)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct{ in, msg string }{
		{strings.Repeat("a", 12), ""},
		{strings.Repeat("a", 11), "must be at least 12 characters"},
		{strings.Repeat("a", 129), "must be at most 128 characters"},
		{strings.Repeat(" ", 12), "must not be blank"},
		{strings.Repeat("ż", 12), ""},
	}
	for _, tt := range tests {
		err := ValidatePassword("password", tt.in)
		if got := message(err); got != tt.msg {
			t.Errorf("ValidatePassword(%q) = %q, want %q", tt.in, got, tt.msg)
		}
		if err != nil && strings.Contains(err.Error(), tt.in) && strings.TrimSpace(tt.in) != "" {
			t.Error("error contains the password")
		}
	}
}

func TestValidateNewUserJoins(t *testing.T) {
	err := ValidateNewUser("", "")
	if !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "name") {
		t.Errorf("err = %v", err)
	}
}
