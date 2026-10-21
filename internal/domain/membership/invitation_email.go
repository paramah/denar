package membership

import (
	"time"

	"github.com/paramah/denar/internal/domain/user"
)

// InvitationEmail writes the e-mail with an invitation link in the language
// of the person who invites (FR-1308); expires is shown in UTC.
func InvitationEmail(l user.Locale, inviter, householdName, link string, expires time.Time) (subject, body string) {
	at := expires.UTC()
	if user.EffectiveLocale(&l) == user.LocaleEN {
		return "Invitation to the household " + householdName,
			inviter + " invites you to the household “" + householdName + "” in Denar.\n\n" +
				"Open the link to join (valid until " + at.Format("2006-01-02 15:04") + " UTC):\n" + link + "\n"
	}
	return "Zaproszenie do gospodarstwa " + householdName,
		inviter + " zaprasza Cię do gospodarstwa „" + householdName + "” w Denarze.\n\n" +
			"Otwórz link, aby dołączyć (ważny do " + at.Format("02.01.2006 15:04") + " UTC):\n" + link + "\n"
}
