package user

import "github.com/paramah/denar/internal/domain"

// Locale is the language a person reads Denar in.
type Locale string

const (
	LocalePL Locale = "pl"
	LocaleEN Locale = "en"
)

// DefaultLocale is used for a person who chose no language: the server does
// not know the browser of a recipient.
const DefaultLocale = LocalePL

// Locales lists the supported languages, the default first.
var Locales = []Locale{LocalePL, LocaleEN}

// ParseLocale accepts exactly "pl" or "en".
func ParseLocale(s string) (Locale, error) {
	switch l := Locale(s); l {
	case LocalePL, LocaleEN:
		return l, nil
	}
	return "", domain.ValidationError{Field: "locale", Message: "must be pl or en"}
}

// EffectiveLocale returns the chosen language, or the default for none (or
// for a value that is not supported).
func EffectiveLocale(l *Locale) Locale {
	if l == nil {
		return DefaultLocale
	}
	if parsed, err := ParseLocale(string(*l)); err == nil {
		return parsed
	}
	return DefaultLocale
}

// EffectiveLocale returns the language texts for u are written in.
func (u User) EffectiveLocale() Locale { return EffectiveLocale(u.Locale) }
