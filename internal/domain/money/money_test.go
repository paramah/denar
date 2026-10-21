package money

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func mustCur(t *testing.T, code string) Currency {
	t.Helper()
	c, ok := LookupCurrency(code)
	if !ok {
		t.Fatalf("no currency %s", code)
	}
	return c
}

func TestParseAndString(t *testing.T) {
	tests := []struct {
		in, code string
		ok       bool
		minor    int64
		out      string
	}{
		{"12.34", "PLN", true, 1234, "12.34"},
		{"-12.3", "PLN", true, -1230, "-12.30"},
		{"12", "PLN", true, 1200, "12.00"},
		{"0", "PLN", true, 0, "0.00"},
		{"-0.05", "PLN", true, -5, "-0.05"},
		{"-0", "PLN", true, 0, "0.00"},
		{"1200", "JPY", true, 1200, "1200"},
		{"1.5", "JPY", false, 0, ""},
		{"1.234", "KWD", true, 1234, "1.234"},
		{"12.345", "PLN", false, 0, ""},
		{"12.", "PLN", false, 0, ""},
		{".5", "PLN", false, 0, ""},
		{"+1", "PLN", false, 0, ""},
		{"1e3", "PLN", false, 0, ""},
		{"1,50", "PLN", false, 0, ""},
		{" 1", "PLN", false, 0, ""},
		{"", "PLN", false, 0, ""},
		{"-", "PLN", false, 0, ""},
		{"92233720368547758.07", "PLN", true, math.MaxInt64, "92233720368547758.07"},
		{"92233720368547758.08", "PLN", false, 0, ""},
		{"-92233720368547758.08", "PLN", true, math.MinInt64, "-92233720368547758.08"},
		{"99999999999999999999999", "PLN", false, 0, ""},
		{"1", "XYZ", false, 0, ""},
	}
	for _, tt := range tests {
		m, err := Parse("amount", tt.in, tt.code)
		if (err == nil) != tt.ok {
			t.Errorf("Parse(%q, %s) err = %v, want ok %v", tt.in, tt.code, err, tt.ok)
			continue
		}
		if err != nil {
			var ve domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != "amount" {
				t.Errorf("Parse(%q) err = %#v, want a ValidationError on amount", tt.in, err)
			}
			continue
		}
		if m.Amount != tt.minor || m.String() != tt.out || m.Currency.Code != tt.code {
			t.Errorf("Parse(%q, %s) = %d %q", tt.in, tt.code, m.Amount, m.String())
		}
	}
}

func TestAddSub(t *testing.T) {
	pln, usd := mustCur(t, "PLN"), mustCur(t, "USD")
	a, b := Money{Amount: 150, Currency: pln}, Money{Amount: -50, Currency: pln}
	sum, err := a.Add(b)
	if err != nil || sum.Amount != 100 {
		t.Errorf("Add = %v, %v", sum, err)
	}
	diff, err := a.Sub(b)
	if err != nil || diff.Amount != 200 {
		t.Errorf("Sub = %v, %v", diff, err)
	}
	if _, err := a.Add(Money{Amount: 1, Currency: usd}); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Add across currencies: %v", err)
	}
	if _, err := (Money{Amount: math.MaxInt64, Currency: pln}).Add(Money{Amount: 1, Currency: pln}); !errors.Is(err, ErrOverflow) {
		t.Errorf("overflow: %v", err)
	}
	if _, err := (Money{Amount: math.MinInt64, Currency: pln}).Sub(Money{Amount: 1, Currency: pln}); !errors.Is(err, ErrOverflow) {
		t.Errorf("underflow: %v", err)
	}
	if a.Neg().Amount != -150 || a.Sign() != 1 || b.Sign() != -1 || !Zero(pln).IsZero() {
		t.Error("Neg/Sign/Zero")
	}
	if _, err := New(1, "XYZ"); err == nil {
		t.Error("New accepted an unknown currency")
	}
}

func TestRoundHalfEven(t *testing.T) {
	tests := []struct {
		num, den int64
		want     int64
	}{
		{5, 2, 2}, {7, 2, 4}, {-5, 2, -2}, {-7, 2, -4}, {1, 3, 0}, {2, 3, 1}, {-2, 3, -1},
		{10, 1, 10}, {0, 7, 0}, {25, 10, 2}, {35, 10, 4}, {251, 100, 3}, {-251, 100, -3},
	}
	for _, tt := range tests {
		got, err := RoundHalfEven(big.NewRat(tt.num, tt.den))
		if err != nil || got != tt.want {
			t.Errorf("RoundHalfEven(%d/%d) = %d, %v; want %d", tt.num, tt.den, got, err, tt.want)
		}
	}
	huge := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(1))
	if _, err := RoundHalfEven(huge); !errors.Is(err, ErrOverflow) {
		t.Errorf("huge: %v", err)
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		amount   string
		from, to string
		rate     string
		want     string
	}{
		{"-10.00", "USD", "PLN", "4.0000", "-40.00"},
		{"1.00", "USD", "PLN", "3.9512", "3.95"},
		{"0.01", "USD", "PLN", "0.5", "0.00"},   // 0.005 → half-even 0
		{"0.03", "USD", "PLN", "0.5", "0.02"},   // 0.015 → 0.02
		{"0.05", "USD", "PLN", "0.5", "0.02"},   // 0.025 → 0.02
		{"-0.05", "USD", "PLN", "0.5", "-0.02"}, // symmetric
		{"1000", "JPY", "PLN", "0.026", "26.00"},
		{"10.00", "PLN", "JPY", "38.4615", "385"},
		{"1.000", "KWD", "PLN", "12.9", "12.90"},
		{"12.34", "PLN", "PLN", "1", "12.34"},
	}
	for _, tt := range tests {
		m, err := Parse("a", tt.amount, tt.from)
		if err != nil {
			t.Fatal(err)
		}
		r, err := ParseRate("rate", tt.rate)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Convert(m, r, mustCur(t, tt.to))
		if err != nil || got.String() != tt.want || got.Currency.Code != tt.to {
			t.Errorf("Convert(%s %s @%s → %s) = %s %s, %v; want %s", tt.amount, tt.from, tt.rate, tt.to, got, got.Currency.Code, err, tt.want)
		}
	}
}

func TestConvertParts(t *testing.T) {
	usd, pln := mustCur(t, "USD"), mustCur(t, "PLN")
	r, _ := ParseRate("rate", "3.3333")
	tests := []struct {
		parts []int64
		total int64
	}{
		{[]int64{-100, -100, -100}, -1000},
		{[]int64{-1, -1, -1}, -10},
		{[]int64{500, -500}, 0},
		{[]int64{-299}, -997},
	}
	for _, tt := range tests {
		got, err := ConvertParts(tt.parts, r, usd, pln, tt.total)
		if err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, g := range got {
			sum += g
		}
		if sum != tt.total || len(got) != len(tt.parts) {
			t.Errorf("ConvertParts(%v) = %v, sum %d want %d", tt.parts, got, sum, tt.total)
		}
	}
	got, _ := ConvertParts([]int64{-100, -200}, r, usd, pln, -1000)
	// -333.33 and -666.66 → residual -0.01 goes to the larger line.
	if got[0] != -333 || got[1] != -667 {
		t.Errorf("residual placement = %v", got)
	}
}

func TestParseFormatRate(t *testing.T) {
	for _, ok := range []string{"4", "3.9512", "0.000123456789", "1.000000000000"} {
		if _, err := ParseRate("rate", ok); err != nil {
			t.Errorf("ParseRate(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "0", "0.0", "-1", "1e2", "1,2", "abc", "1.0000000000001", "+1", ".5", "1."} {
		if _, err := ParseRate("rate", bad); err == nil {
			t.Errorf("ParseRate(%q) accepted", bad)
		}
	}
	tests := map[string]string{"4.0000": "4", "3.9512": "3.9512", "0.123456789": "0.12345679"}
	for in, want := range tests {
		r, _ := ParseRate("rate", in)
		if got := FormatRate(r); got != want {
			t.Errorf("FormatRate(%s) = %s, want %s", in, got, want)
		}
	}
	cross := CrossRate(big.NewRat(4, 1), big.NewRat(43, 10))
	if FormatRate(cross) != "0.93023256" {
		t.Errorf("CrossRate = %s", FormatRate(cross))
	}
}
