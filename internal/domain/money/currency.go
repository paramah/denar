// Package money holds currencies, amounts and exchange rates. Amounts are
// int64 minor units of a currency; there are no floats anywhere.
package money

import (
	"slices"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Currency is an ISO 4217 currency with the number of its minor-unit digits.
type Currency struct {
	Code     string
	Decimals int
}

// currencies is the closed list the instance knows, by code.
var currencies = map[string]Currency{}

func init() {
	for _, c := range []Currency{
		{"PLN", 2}, {"EUR", 2}, {"USD", 2}, {"GBP", 2}, {"CHF", 2}, {"CZK", 2},
		{"SEK", 2}, {"NOK", 2}, {"DKK", 2}, {"HUF", 2}, {"RON", 2}, {"BGN", 2},
		{"UAH", 2}, {"TRY", 2}, {"ISK", 0}, {"JPY", 0}, {"KRW", 0}, {"CNY", 2},
		{"HKD", 2}, {"SGD", 2}, {"INR", 2}, {"AUD", 2}, {"NZD", 2}, {"CAD", 2},
		{"MXN", 2}, {"BRL", 2}, {"ZAR", 2}, {"ILS", 2}, {"AED", 2}, {"THB", 2},
		{"KWD", 3}, {"BHD", 3},
	} {
		currencies[c.Code] = c
	}
}

// LookupCurrency returns the currency of an upper-case ISO 4217 code.
func LookupCurrency(code string) (Currency, bool) {
	c, ok := currencies[code]
	return c, ok
}

// Currencies returns every known currency, ordered by code.
func Currencies() []Currency {
	out := make([]Currency, 0, len(currencies))
	for _, c := range currencies {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Currency) int { return strings.Compare(a.Code, b.Code) })
	return out
}

// ValidateCurrency checks that code names a known currency, reporting the
// problem on field.
func ValidateCurrency(field, code string) error {
	if _, ok := LookupCurrency(code); !ok {
		return domain.ValidationError{Field: field, Message: "must be a supported ISO 4217 currency code"}
	}
	return nil
}
