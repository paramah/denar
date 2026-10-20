package money

import (
	"errors"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
)

func TestLookupCurrency(t *testing.T) {
	tests := []struct {
		code     string
		ok       bool
		decimals int
	}{
		{"PLN", true, 2}, {"EUR", true, 2}, {"JPY", true, 0}, {"KWD", true, 3},
		{"pln", false, 0}, {"XXX", false, 0}, {"", false, 0},
	}
	for _, tt := range tests {
		c, ok := LookupCurrency(tt.code)
		if ok != tt.ok || (ok && c.Decimals != tt.decimals) {
			t.Errorf("LookupCurrency(%q) = %+v, %v", tt.code, c, ok)
		}
	}
}

func TestCurrenciesSorted(t *testing.T) {
	all := Currencies()
	if len(all) < 20 {
		t.Fatalf("only %d currencies", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Code >= all[i].Code {
			t.Errorf("not sorted at %d: %s %s", i, all[i-1].Code, all[i].Code)
		}
	}
}

func TestValidateCurrency(t *testing.T) {
	if err := ValidateCurrency("base_currency", "PLN"); err != nil {
		t.Error(err)
	}
	err := ValidateCurrency("base_currency", "ABC")
	var ve domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "base_currency" {
		t.Errorf("err = %v", err)
	}
}
