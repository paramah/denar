package notification

import (
	"testing"

	"github.com/paramah/denar/internal/domain/user"
)

func TestFixedTexts(t *testing.T) {
	if got := HouseholdLine(user.LocalePL, "Dom"); got != "Gospodarstwo: Dom" {
		t.Errorf("pl household = %q", got)
	}
	if got := HouseholdLine(user.LocaleEN, "Home"); got != "Household: Home" {
		t.Errorf("en household = %q", got)
	}
	if got := HouseholdLine("", "Dom"); got != "Gospodarstwo: Dom" {
		t.Errorf("no locale = %q", got)
	}
	if got := EmailFooter(user.LocalePL); got != "Denar · ustawienia powiadomień zmienisz w aplikacji." {
		t.Errorf("pl footer = %q", got)
	}
	if got := EmailFooter(user.LocaleEN); got != "Denar · change your notification settings in the app." {
		t.Errorf("en footer = %q", got)
	}
	if title, body := TestMessage(user.LocalePL, "Hook"); title != "Test kanału Denar" || body != "Ten kanał (Hook) działa: tu będą trafiać powiadomienia Denara." {
		t.Errorf("pl test = %q %q", title, body)
	}
	if title, body := TestMessage(user.LocaleEN, "Hook"); title != "Denar channel test" || body != "This channel (Hook) works: Denar notifications will arrive here." {
		t.Errorf("en test = %q %q", title, body)
	}
}
