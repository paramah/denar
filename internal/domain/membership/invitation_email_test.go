package membership

import (
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain/user"
)

func TestInvitationEmail(t *testing.T) {
	expires := time.Date(2026, 10, 28, 9, 5, 0, 0, time.FixedZone("CET", 3600))
	subject, body := InvitationEmail(user.LocalePL, "Ada", "Dom", "https://x/invite#t", expires)
	if subject != "Zaproszenie do gospodarstwa Dom" ||
		body != "Ada zaprasza Cię do gospodarstwa „Dom” w Denarze.\n\nOtwórz link, aby dołączyć (ważny do 28.10.2026 08:05 UTC):\nhttps://x/invite#t\n" {
		t.Errorf("pl = %q\n%q", subject, body)
	}
	subject, body = InvitationEmail(user.LocaleEN, "Ada", "Home", "https://x/invite#t", expires)
	if subject != "Invitation to the household Home" ||
		body != "Ada invites you to the household “Home” in Denar.\n\nOpen the link to join (valid until 2026-10-28 08:05 UTC):\nhttps://x/invite#t\n" {
		t.Errorf("en = %q\n%q", subject, body)
	}
}
