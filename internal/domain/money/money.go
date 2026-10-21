package money

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/paramah/denar/internal/domain"
)

var (
	// ErrCurrencyMismatch: amounts of different currencies are added or
	// compared without a conversion.
	ErrCurrencyMismatch = errors.New("money: currencies differ")
	// ErrOverflow: the result does not fit in int64 minor units.
	ErrOverflow = errors.New("money: amount out of range")
)

// Money is an amount in minor units of a currency (grosze for PLN, cents
// for USD, yen for JPY).
type Money struct {
	Amount   int64
	Currency Currency
}

// New returns the amount in the currency with the code.
func New(amount int64, code string) (Money, error) {
	c, ok := LookupCurrency(code)
	if !ok {
		return Money{}, domain.ValidationError{Field: "currency", Message: "must be a supported ISO 4217 currency code"}
	}
	return Money{Amount: amount, Currency: c}, nil
}

// Zero is no money in the currency.
func Zero(c Currency) Money { return Money{Currency: c} }

func (m Money) IsZero() bool { return m.Amount == 0 }

// Sign returns -1, 0 or +1.
func (m Money) Sign() int {
	switch {
	case m.Amount < 0:
		return -1
	case m.Amount > 0:
		return 1
	}
	return 0
}

// Neg returns the amount with the opposite sign.
func (m Money) Neg() Money { return Money{Amount: -m.Amount, Currency: m.Currency} }

// Add returns m + o; both must be in the same currency.
func (m Money) Add(o Money) (Money, error) {
	if m.Currency.Code != o.Currency.Code {
		return Money{}, ErrCurrencyMismatch
	}
	sum, ok := AddInt64(m.Amount, o.Amount)
	if !ok {
		return Money{}, ErrOverflow
	}
	return Money{Amount: sum, Currency: m.Currency}, nil
}

// Sub returns m - o; both must be in the same currency.
func (m Money) Sub(o Money) (Money, error) {
	if o.Amount == math.MinInt64 {
		return Money{}, ErrOverflow
	}
	return m.Add(o.Neg())
}

// AddInt64 adds and reports whether the sum fits.
func AddInt64(a, b int64) (int64, bool) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, false
	}
	return s, true
}

// String writes the amount as a decimal with the currency's digits, such
// as "-12.34" or, for JPY, "1200". The currency code is not included.
func (m Money) String() string {
	neg := m.Amount < 0
	u := uint64(m.Amount)
	if neg {
		u = -u // two's complement: correct for MinInt64 too
	}
	digits := strconv.FormatUint(u, 10)
	if d := m.Currency.Decimals; d > 0 {
		if len(digits) <= d {
			digits = strings.Repeat("0", d-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-d] + "." + digits[len(digits)-d:]
	}
	if neg {
		return "-" + digits
	}
	return digits
}

// Parse reads a decimal amount of the currency with the code: an optional
// minus, digits, and at most as many decimals as the currency has. Anything
// else — a plus, a comma, spaces, an exponent, more decimals, a value out
// of range — is a domain.ValidationError on field.
func Parse(field, s, code string) (Money, error) {
	c, ok := LookupCurrency(code)
	if !ok {
		return Money{}, domain.ValidationError{Field: field, Message: "has an unsupported currency"}
	}
	bad := domain.ValidationError{Field: field, Message: "must be a decimal amount with at most " +
		strconv.Itoa(c.Decimals) + " decimal places"}
	neg := strings.HasPrefix(s, "-")
	body := strings.TrimPrefix(s, "-")
	whole, frac, hasPoint := strings.Cut(body, ".")
	if whole == "" || !allDigits(whole) || (hasPoint && (frac == "" || !allDigits(frac))) || len(frac) > c.Decimals {
		return Money{}, bad
	}
	digits := strings.TrimLeft(whole+frac+strings.Repeat("0", c.Decimals-len(frac)), "0")
	if digits == "" {
		return Money{Currency: c}, nil
	}
	u, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return Money{}, domain.ValidationError{Field: field, Message: "is out of range"}
	}
	switch {
	case neg && u <= 1<<63:
		return Money{Amount: int64(-u), Currency: c}, nil // -(1<<63) wraps to MinInt64
	case !neg && u <= math.MaxInt64:
		return Money{Amount: int64(u), Currency: c}, nil
	}
	return Money{}, domain.ValidationError{Field: field, Message: "is out of range"}
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// RoundHalfEven rounds r to the nearest integer, ties to even (banker's
// rounding).
func RoundHalfEven(r *big.Rat) (int64, error) {
	num, den := new(big.Int).Set(r.Num()), r.Denom() // den > 0
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	// q is truncated toward zero; compare 2|rem| with den.
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	switch c := twice.Cmp(den); {
	case c > 0 || (c == 0 && q.Bit(0) == 1):
		if num.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	if !q.IsInt64() {
		return 0, ErrOverflow
	}
	return q.Int64(), nil
}

// scale is the factor that turns minor units of from into minor units of
// to, given a rate between whole units.
func scale(from, to Currency) *big.Rat {
	exp := to.Decimals - from.Decimals
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(exp))), nil)
	if exp >= 0 {
		return new(big.Rat).SetInt(p)
	}
	return new(big.Rat).SetFrac(big.NewInt(1), p)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// convertMinor converts minor units of from into minor units of to at the
// rate (one unit of from is rate units of to), unrounded.
func convertMinor(amount int64, rate *big.Rat, from, to Currency) *big.Rat {
	r := new(big.Rat).SetInt64(amount)
	r.Mul(r, rate)
	return r.Mul(r, scale(from, to))
}

// Convert converts m into the currency to at the rate, where one unit of
// m's currency is rate units of to, rounding half to even.
func Convert(m Money, rate *big.Rat, to Currency) (Money, error) {
	n, err := RoundHalfEven(convertMinor(m.Amount, rate, m.Currency, to))
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: n, Currency: to}, nil
}

// ConvertParts converts the parts of an amount (the lines of a split) and
// makes them add up to total, the converted amount of the whole: what
// rounding leaves over goes to the part of the largest absolute value
// (the first of equals).
func ConvertParts(parts []int64, rate *big.Rat, from, to Currency, total int64) ([]int64, error) {
	out := make([]int64, len(parts))
	if len(parts) == 0 {
		return out, nil
	}
	var sum int64
	largest := 0
	for i, p := range parts {
		n, err := RoundHalfEven(convertMinor(p, rate, from, to))
		if err != nil {
			return nil, err
		}
		out[i] = n
		var ok bool
		if sum, ok = AddInt64(sum, n); !ok {
			return nil, ErrOverflow
		}
		if abs64(p) > abs64(parts[largest]) {
			largest = i
		}
	}
	rest := total - sum
	fixed, ok := AddInt64(out[largest], rest)
	if !ok {
		return nil, ErrOverflow
	}
	out[largest] = fixed
	return out, nil
}

func abs64(n int64) uint64 {
	if n < 0 {
		return uint64(-n)
	}
	return uint64(n)
}
