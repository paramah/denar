package connection_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/connection"
)

func TestConnectionValidate(t *testing.T) {
	ok := connection.Connection{Provider: "gocardless", Kind: connection.Bank, Status: connection.Active, DisplayName: "mBank"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid connection: %v", err)
	}
	for name, c := range map[string]connection.Connection{
		"provider": {Provider: "Go Cardless", Kind: connection.Bank, Status: connection.Active, DisplayName: "x"},
		"kind":     {Provider: "x", Kind: "atm", Status: connection.Active, DisplayName: "x"},
		"status":   {Provider: "x", Kind: connection.Bank, Status: "gone", DisplayName: "x"},
		"name":     {Provider: "x", Kind: connection.Bank, Status: connection.Active, DisplayName: ""},
	} {
		if err := c.Validate(); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: error = %v", name, err)
		}
	}
	long := ok
	long.LastError = strings.Repeat("ż", 600)
	if got := long.Normalize().LastError; len([]rune(got)) != connection.MaxLastError {
		t.Errorf("last error kept %d runes", len([]rune(got)))
	}
}

func TestAccountValidate(t *testing.T) {
	a := connection.Account{ExternalID: " acc-1 ", Name: " Konto ", Currency: "pln"}.Normalize()
	if a.ExternalID != "acc-1" || a.Name != "Konto" || a.Currency != "PLN" {
		t.Errorf("normalised = %+v", a)
	}
	if err := a.Validate(); err != nil {
		t.Error(err)
	}
	if err := (connection.Account{}).Validate(); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("empty account: %v", err)
	}
}
