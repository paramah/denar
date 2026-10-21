package money

import (
	"math/big"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
)

// maxRateDecimals is how many decimals a rate written by hand may have.
const maxRateDecimals = 12

// ParseRate reads an exchange rate: a positive decimal such as "3.9512",
// with at most 12 decimals. A rate is how many units of one currency one
// unit of another costs; it is kept as an exact fraction.
func ParseRate(field, s string) (*big.Rat, error) {
	bad := domain.ValidationError{Field: field, Message: "must be a positive decimal number with at most 12 decimal places"}
	whole, frac, hasPoint := strings.Cut(s, ".")
	if !allDigits(whole) || (hasPoint && !allDigits(frac)) || len(frac) > maxRateDecimals || len(s) > 40 {
		return nil, bad
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 {
		return nil, bad
	}
	return r, nil
}

// FormatRate writes a rate with at most 8 decimals, rounded, without
// trailing zeros.
func FormatRate(r *big.Rat) string {
	s := r.FloatString(8)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// CrossRate is the rate from A to B given the rates of both against a
// third currency (PLN for NBP): one A costs fromPLN, one B costs toPLN.
func CrossRate(fromPLN, toPLN *big.Rat) *big.Rat {
	return new(big.Rat).Quo(fromPLN, toPLN)
}
