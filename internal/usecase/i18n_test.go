package usecase_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/usecase"
)

// speaks sets the language of a person.
func (f *notifyFamily) speaks(id domain.ID, l user.Locale) user.User {
	f.t.Helper()
	u, err := f.Users.ByID(ctx, id)
	f.must(err)
	u.Locale = &l
	f.must(f.Users.Update(ctx, u))
	return u
}

func TestDispatchInTheLanguageOfEachRecipient(t *testing.T) {
	f := newNotifyFamily(t)
	f.speaks("member", user.LocaleEN)
	f.speaks("owner", user.LocalePL)
	hook := f.channel("member", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Hook",
		Config: map[string]string{"url": "https://hooks.example.com/a"}})
	_, err := usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.member), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, InApp: true, ChannelIDs: []domain.ID{hook.ID}},
	})
	noErr(t, err)

	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	r := f.dispatch()
	if r.Notifications != 4 || r.Deliveries != 1 {
		t.Fatalf("result = %+v", r)
	}
	titles := map[domain.ID]string{}
	for _, n := range f.Notifications.All() {
		titles[n.UserID] = n.Title
		if n.Link != "/h/h1/subscriptions/s1" {
			t.Errorf("link of %s = %q", n.UserID, n.Link)
		}
	}
	want := map[domain.ID]string{
		"member": "Renewal: Netflix in 3 days",
		"owner":  "Odnowienie: Netflix za 3 dni",
		"admin":  "Odnowienie: Netflix za 3 dni", // no preference: Polish
		"viewer": "Odnowienie: Netflix za 3 dni",
	}
	for id, title := range want {
		if titles[id] != title {
			t.Errorf("title for %s = %q, want %q", id, titles[id], title)
		}
	}
	d := f.Deliveries.All()[0]
	if d.Title != "Renewal: Netflix in 3 days" || d.Body != "Netflix renews on 2026-10-31: 43.00 PLN." {
		t.Errorf("delivery = %q / %q", d.Title, d.Body)
	}

	// The worker hands the language to the channel, for its fixed texts.
	r2, err := usecase.NewDeliverNotifications(f.deps).Do(ctx)
	noErr(t, err)
	if r2.Sent != 1 {
		t.Fatalf("deliver = %+v", r2)
	}
	if msg := f.hooks.Messages()[0].Message; msg.Locale != user.LocaleEN || msg.Title != "Renewal: Netflix in 3 days" {
		t.Errorf("message = %+v", msg)
	}
}

func TestTestChannelInTheLanguageOfThePerson(t *testing.T) {
	f := newNotifyFamily(t)
	member := f.speaks("member", user.LocaleEN)
	c := f.channel("member", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Hook", Config: map[string]string{"url": "https://x.example.com"}})
	_, err := usecase.NewTestChannel(f.deps).Do(ctx, human(member), c.ID)
	noErr(t, err)
	msg := f.hooks.Messages()[0].Message
	if msg.Title != "Denar channel test" || msg.Body != "This channel (Hook) works: Denar notifications will arrive here." ||
		msg.Locale != user.LocaleEN {
		t.Errorf("message = %+v", msg)
	}
}

func TestInvitationEmailInTheLanguageOfTheInviter(t *testing.T) {
	f := newNotifyFamily(t)
	admin := f.speaks("admin", user.LocaleEN)
	inv := usecase.NewCreateInvitation(f.deps, usecase.InvitationConfig{TTL: time.Hour, BaseURL: "https://denar.example.com"})
	v, err := inv.Do(ctx, human(admin), "h1", usecase.CreateInvitationInput{Email: "grace@example.com", Role: membership.RoleMember, SendEmail: true})
	noErr(t, err)
	m := f.mailer.Sent[0]
	expires := v.Invitation.ExpiresAt.UTC().Format("2006-01-02 15:04")
	want := "User admin invites you to the household “Dom h1” in Denar.\n\nOpen the link to join (valid until " + expires +
		" UTC):\nhttps://denar.example.com/invite#" + v.Token + "\n"
	if m.Subject != "Invitation to the household Dom h1" || m.Body != want {
		t.Errorf("mail = %q\n%q", m.Subject, m.Body)
	}

	// Without a preference: Polish, as before.
	_, err = inv.Do(ctx, human(f.owner), "h1", usecase.CreateInvitationInput{Email: "ada@example.com", Role: membership.RoleMember, SendEmail: true})
	noErr(t, err)
	if m := f.mailer.Sent[1]; m.Subject != "Zaproszenie do gospodarstwa Dom h1" || !strings.Contains(m.Body, "zaprasza Cię do gospodarstwa „Dom h1” w Denarze.") {
		t.Errorf("polish mail = %q\n%q", m.Subject, m.Body)
	}
}

func TestDeliveryErrorsAreStableEnglish(t *testing.T) {
	f := newNotifyFamily(t)
	off := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Off",
		Config: map[string]string{"url": "https://hooks.example.com/b"}})
	_, err := usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.owner), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, InApp: true, ChannelIDs: []domain.ID{off.ID}},
	})
	noErr(t, err)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	f.dispatch()
	_, err = usecase.NewUpdateChannel(f.deps).Do(ctx, human(f.owner), off.ID, usecase.UpdateChannelInput{Enabled: domain.Some(false)})
	noErr(t, err)
	_, err = usecase.NewDeliverNotifications(f.deps).Do(ctx)
	noErr(t, err)
	if d := f.Deliveries.All()[0]; d.LastError != "channel is disabled" {
		t.Errorf("disabled channel error = %q", d.LastError)
	}

	tg := f.channel("owner", usecase.CreateChannelInput{Type: notification.Telegram, Name: "TG",
		Config: map[string]string{"chat_id": "1"}, Secrets: map[string]string{"bot_token": "t"}})
	noKey := f.deps
	noKey.Secrets = nil
	res, err := usecase.NewTestChannel(noKey).Do(ctx, human(f.owner), tg.ID)
	noErr(t, err)
	if res.Error != "DENAR_MASTER_KEY is not set: cannot read the channel secrets" {
		t.Errorf("no key = %q", res.Error)
	}
	otherKey := f.deps
	otherKey.Secrets = brokenBox{}
	res, err = usecase.NewTestChannel(otherKey).Do(ctx, human(f.owner), tg.ID)
	noErr(t, err)
	if res.Error != "cannot read the channel secrets (DENAR_MASTER_KEY changed?): set them again" {
		t.Errorf("unreadable = %q", res.Error)
	}
	email := f.channel("owner", usecase.CreateChannelInput{Type: notification.Email, Name: "Mail"})
	noSMTP := f.deps
	noSMTP.Mailer = nil
	res, err = usecase.NewTestChannel(noSMTP).Do(ctx, human(f.owner), email.ID)
	noErr(t, err)
	if res.Error != "SMTP is not configured on the server" {
		t.Errorf("no smtp = %q", res.Error)
	}
}

// brokenBox seals but cannot open: a changed master key.
type brokenBox struct{}

func (brokenBox) Seal(p, _ []byte) ([]byte, error) { return p, nil }
func (brokenBox) Open([]byte, []byte) ([]byte, error) {
	return nil, errors.New("message authentication failed")
}
