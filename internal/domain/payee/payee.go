// Package payee models who money is paid to or received from, with the
// patterns that recognise them in free text and the envelope they usually
// go to.
package payee

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
)

// Limits of a payee.
const (
	MaxNameLength       = 100
	MaxPatterns         = 20
	MaxPatternLength    = 100
	MaxEnvelopeIDLength = 64
)

// Payee is a counterparty of transactions.
type Payee struct {
	ID          domain.ID
	HouseholdID domain.ID
	Name        string
	// Patterns are plain substrings, compared without case, that identify
	// the payee in text such as a bank statement line.
	Patterns []string
	// DefaultEnvelopeID is given to new expenses of the payee that name no
	// envelope ("" none).
	DefaultEnvelopeID domain.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Normalize trims the name and the patterns and drops empty and repeated
// patterns.
func (p Payee) Normalize() Payee {
	p.Name = strings.TrimSpace(p.Name)
	out := []string{}
	for _, pat := range p.Patterns {
		pat = strings.TrimSpace(pat)
		if pat != "" && !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, pat) }) {
			out = append(out, pat)
		}
	}
	p.Patterns = out
	return p
}

// Validate checks a normalised payee.
func (p Payee) Validate() error {
	var errs []error
	if n := utf8.RuneCountInString(p.Name); n == 0 || n > MaxNameLength {
		errs = append(errs, domain.ValidationError{Field: "name", Message: "must be 1 to 100 characters"})
	}
	if len(p.Patterns) > MaxPatterns {
		errs = append(errs, domain.ValidationError{Field: "patterns", Message: "may have at most 20 patterns"})
	}
	for _, pat := range p.Patterns {
		if utf8.RuneCountInString(pat) > MaxPatternLength {
			errs = append(errs, domain.ValidationError{Field: "patterns", Message: "must each be at most 100 characters"})
			break
		}
	}
	if len(p.DefaultEnvelopeID) > MaxEnvelopeIDLength {
		errs = append(errs, domain.ValidationError{Field: "default_envelope_id", Message: "is not an envelope id"})
	}
	return errors.Join(errs...)
}

// Matches reports whether one of the payee's patterns occurs in text,
// ignoring case.
func (p Payee) Matches(text string) bool {
	lower := strings.ToLower(text)
	for _, pat := range p.Patterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}

// Resolve finds the payee text refers to: the one of that name, ignoring
// case, else the first by name whose pattern occurs in the text.
func Resolve(payees []Payee, text string) (Payee, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Payee{}, false
	}
	for _, p := range payees {
		if strings.EqualFold(p.Name, text) {
			return p, true
		}
	}
	sorted := slices.Clone(payees)
	slices.SortStableFunc(sorted, func(a, b Payee) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	for _, p := range sorted {
		if p.Matches(text) {
			return p, true
		}
	}
	return Payee{}, false
}
