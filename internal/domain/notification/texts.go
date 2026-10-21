package notification

import "github.com/paramah/denar/internal/domain/user"

// The fixed texts channels add to a message, in the language of the person
// the message is for (FR-1308). An unsupported language gives Polish.

func english(l user.Locale) bool { return user.EffectiveLocale(&l) == user.LocaleEN }

// HouseholdLine names the household a message is about.
func HouseholdLine(l user.Locale, name string) string {
	if english(l) {
		return "Household: " + name
	}
	return "Gospodarstwo: " + name
}

// EmailFooter closes an e-mail notification.
func EmailFooter(l user.Locale) string {
	if english(l) {
		return "Denar · change your notification settings in the app."
	}
	return "Denar · ustawienia powiadomień zmienisz w aplikacji."
}

// TestMessage is the message that checks a channel works.
func TestMessage(l user.Locale, channelName string) (title, body string) {
	if english(l) {
		return "Denar channel test", "This channel (" + channelName + ") works: Denar notifications will arrive here."
	}
	return "Test kanału Denar", "Ten kanał (" + channelName + ") działa: tu będą trafiać powiadomienia Denara."
}
