// Package connection describes links to outside services that feed
// transactions in: banks (PSD2 aggregators) and billing of providers
// (OpenAI, Apple receipts). Stage 008 holds the model only; the connectors
// come in stages 009-011.
package connection

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Kind is what a connection brings in.
type Kind string

const (
	// Bank brings the transactions of bank accounts.
	Bank Kind = "bank"
	// Billing brings the charges of a provider.
	Billing Kind = "billing"
)

func (k Kind) Valid() bool { return k == Bank || k == Billing }

// Status is the state of a connection.
type Status string

const (
	Active   Status = "active"
	Error    Status = "error"
	Disabled Status = "disabled"
)

func (s Status) Valid() bool { return s == Active || s == Error || s == Disabled }

// MaxLastError is the longest error kept on a connection.
const MaxLastError = 500

// Connection is a household's link to a provider. Its credentials are
// stored sealed only; the domain never sees them in the clear.
type Connection struct {
	ID          domain.ID
	HouseholdID domain.ID
	Provider    string
	Kind        Kind
	Status      Status
	DisplayName string
	// SealedCredentials are the credentials sealed with the master key,
	// bound to the connection's id.
	SealedCredentials []byte
	LastSyncAt        time.Time
	LastError         string
	CreatedBy         domain.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var providerKey = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

// ValidProvider reports whether key may name a provider.
func ValidProvider(key string) bool { return providerKey.MatchString(key) }

// Normalize trims the display name and shortens the last error.
func (c Connection) Normalize() Connection {
	c.DisplayName = strings.TrimSpace(c.DisplayName)
	c.LastError = Truncate(strings.TrimSpace(c.LastError), MaxLastError)
	return c
}

// Validate checks the connection.
func (c Connection) Validate() error {
	var errs []error
	if !ValidProvider(c.Provider) {
		errs = append(errs, domain.ValidationError{Field: "provider", Message: "must be 1 to 40 lower-case letters, digits, _ or -"})
	}
	if !c.Kind.Valid() {
		errs = append(errs, domain.ValidationError{Field: "kind", Message: "must be bank or billing"})
	}
	if !c.Status.Valid() {
		errs = append(errs, domain.ValidationError{Field: "status", Message: "must be active, error or disabled"})
	}
	if n := utf8.RuneCountInString(c.DisplayName); n < 1 || n > 100 {
		errs = append(errs, domain.ValidationError{Field: "display_name", Message: "must be 1 to 100 characters"})
	}
	if utf8.RuneCountInString(c.LastError) > MaxLastError {
		errs = append(errs, domain.ValidationError{Field: "last_error", Message: "is too long"})
	}
	return errors.Join(errs...)
}

// Account is an account of the provider and the wallet it goes to.
type Account struct {
	ConnectionID domain.ID
	ExternalID   string
	Name         string
	// Currency is the account's, "" when the provider does not say.
	Currency string
	// WalletID is the wallet its transactions go to; "" while unmapped.
	WalletID domain.ID
}

// Normalize trims the account's texts.
func (a Account) Normalize() Account {
	a.ExternalID, a.Name, a.Currency = strings.TrimSpace(a.ExternalID), strings.TrimSpace(a.Name), strings.ToUpper(strings.TrimSpace(a.Currency))
	return a
}

// Validate checks the account.
func (a Account) Validate() error {
	var errs []error
	if n := utf8.RuneCountInString(a.ExternalID); n < 1 || n > 200 {
		errs = append(errs, domain.ValidationError{Field: "external_id", Message: "must be 1 to 200 characters"})
	}
	if utf8.RuneCountInString(a.Name) > 100 {
		errs = append(errs, domain.ValidationError{Field: "name", Message: "must be at most 100 characters"})
	}
	return errors.Join(errs...)
}

// Truncate shortens s to at most n runes.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
