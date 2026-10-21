package user

import (
	"errors"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func TestParseLocale(t *testing.T) {
	for _, in := range []string{"pl", "en"} {
		got, err := ParseLocale(in)
		if err != nil || string(got) != in {
			t.Errorf("ParseLocale(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "de", "PL", "en-GB", " pl"} {
		_, err := ParseLocale(in)
		var ve domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "locale" || ve.Message != "must be pl or en" {
			t.Errorf("ParseLocale(%q) err = %v", in, err)
		}
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ParseLocale(%q) is not ErrInvalid", in)
		}
	}
}

func TestEffectiveLocale(t *testing.T) {
	en := LocaleEN
	pl := LocalePL
	if got := EffectiveLocale(nil); got != LocalePL {
		t.Errorf("nil = %q", got)
	}
	if got := EffectiveLocale(&en); got != LocaleEN {
		t.Errorf("en = %q", got)
	}
	if got := EffectiveLocale(&pl); got != LocalePL {
		t.Errorf("pl = %q", got)
	}
	bogus := Locale("de")
	if got := EffectiveLocale(&bogus); got != LocalePL {
		t.Errorf("unknown = %q", got)
	}
	if got := (User{Locale: &en}).EffectiveLocale(); got != LocaleEN {
		t.Errorf("method = %q", got)
	}
}
