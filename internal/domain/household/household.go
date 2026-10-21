// Package household models a household: the space that owns budgets,
// wallets and bots, with its base currency, time zone and the day its
// budget month starts.
package household

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/user"
)

// Household is a space of data shared by its members. BaseCurrency never
// changes after creation (decision D-007).
type Household struct {
	ID            domain.ID
	Name          string
	BaseCurrency  string // ISO 4217
	Timezone      string // IANA, for example Europe/Warsaw
	MonthStartDay int    // 1..28, the day a budget month begins
	CreatedBy     domain.ID
	CreatedAt     time.Time
}

const (
	MaxNameLength = 100
	MinStartDay   = 1
	MaxStartDay   = 28
)

// Normalize trims the name and upper-cases the currency code.
func (h Household) Normalize() Household {
	h.Name = strings.TrimSpace(h.Name)
	h.BaseCurrency = strings.ToUpper(strings.TrimSpace(h.BaseCurrency))
	h.Timezone = strings.TrimSpace(h.Timezone)
	return h
}

// Validate checks a normalised household and reports every problem on the
// fields name, base_currency, timezone and month_start_day.
func (h Household) Validate() error {
	var errs []error
	if err := ValidateName(h.Name); err != nil {
		errs = append(errs, err)
	}
	if err := money.ValidateCurrency("base_currency", h.BaseCurrency); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateTimezone(h.Timezone); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateStartDay(h.MonthStartDay); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ValidateName checks a trimmed household name.
func ValidateName(name string) error {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return domain.ValidationError{Field: "name", Message: "must not be empty"}
	case n > MaxNameLength:
		return domain.ValidationError{Field: "name", Message: "must be at most 100 characters"}
	case strings.ContainsFunc(name, user.Unprintable):
		return domain.ValidationError{Field: "name", Message: "must not contain control characters"}
	}
	return nil
}

// ValidateTimezone checks an IANA time zone name. "Local" and the empty
// name, which the standard library accepts, are refused.
func ValidateTimezone(tz string) error {
	invalid := domain.ValidationError{Field: "timezone", Message: "must be an IANA time zone such as Europe/Warsaw"}
	if tz == "" || tz == "Local" {
		return invalid
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return invalid
	}
	return nil
}

// ValidateStartDay checks the first day of a budget month.
func ValidateStartDay(day int) error {
	if day < MinStartDay || day > MaxStartDay {
		return domain.ValidationError{Field: "month_start_day", Message: "must be between 1 and 28"}
	}
	return nil
}
