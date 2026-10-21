package usecase_test

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
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
}
