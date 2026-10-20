package household

import (
	"errors"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
)

func fields(err error) []string {
	var out []string
	var walk func(error)
	walk = func(err error) {
		switch e := err.(type) {
		case domain.ValidationError:
			out = append(out, e.Field)
		case interface{ Unwrap() []error }:
			for _, c := range e.Unwrap() {
				walk(c)
			}
		}
	}
	walk(err)
	return out
}

func TestValidate(t *testing.T) {
	ok := Household{Name: " Dom ", BaseCurrency: "pln", Timezone: "Europe/Warsaw", MonthStartDay: 1}.Normalize()
	if ok.Name != "Dom" || ok.BaseCurrency != "PLN" {
		t.Errorf("Normalize = %+v", ok)
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc  string
		h     Household
		field string
	}{
		{"empty name", Household{Name: "", BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 1}, "name"},
		{"long name", Household{Name: strings.Repeat("x", 101), BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 1}, "name"},
		{"unknown currency", Household{Name: "D", BaseCurrency: "XYZ", Timezone: "UTC", MonthStartDay: 1}, "base_currency"},
		{"bad tz", Household{Name: "D", BaseCurrency: "PLN", Timezone: "Mars/Olympus", MonthStartDay: 1}, "timezone"},
		{"local tz", Household{Name: "D", BaseCurrency: "PLN", Timezone: "Local", MonthStartDay: 1}, "timezone"},
		{"empty tz", Household{Name: "D", BaseCurrency: "PLN", Timezone: "", MonthStartDay: 1}, "timezone"},
		{"day 0", Household{Name: "D", BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 0}, "month_start_day"},
		{"day 29", Household{Name: "D", BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 29}, "month_start_day"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.h.Validate()
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("err = %v", err)
			}
			got := fields(err)
			if len(got) != 1 || got[0] != tt.field {
				t.Errorf("fields = %v, want [%s]", got, tt.field)
			}
		})
	}
	if got := fields(Household{}.Validate()); len(got) != 4 {
		t.Errorf("empty household reports %v, want all four fields", got)
	}
	if ValidateStartDay(28) != nil {
		t.Error("28 rejected")
	}
}
