package usecase_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
	"git.eiac.dev/tools/denar/internal/usecase/fakes"
)

// notifyFamily is a family with notifications wired: a webhook notifier,
// an e-mail notifier and mailer, the fake secret box, wallets w1 (shared)
// and wp (private: owner and member), and a bot of the member.
type notifyFamily struct {
	*family
	hooks  *fakes.Notifier
	emails *fakes.Notifier
	mailer *fakes.Mailer
}

func newNotifyFamily(t *testing.T) *notifyFamily {
	f := &notifyFamily{family: newFamily(t), hooks: &fakes.Notifier{}, emails: &fakes.Notifier{}, mailer: &fakes.Mailer{}}
	f.deps.Notifiers = map[notification.ChannelType]usecase.Notifier{
		notification.Webhook: f.hooks, notification.Ntfy: f.hooks, notification.Telegram: f.hooks, notification.Matrix: f.hooks,
		notification.Email: f.emails,
	}
	f.deps.Mailer = f.mailer
	f.deps.Secrets = fakes.SecretBox{}
	f.deps.BaseURL = "https://denar.example.com"
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWallet("wp", "h1", f.owner, wallet.Private, f.member.ID)
	return f
}

func (f *notifyFamily) appendEvent(id domain.ID, typ event.Type, walletID domain.ID) {
	f.t.Helper()
	_, err := f.Events.Append(ctx, event.Event{ID: id, HouseholdID: "h1", Type: typ, ResourceType: event.ResourceSubscription,
		ResourceID: "s1", DedupeKey: "key:" + string(id), OccurredAt: f.Clock.Now(),
		Payload: map[string]string{"subscription_id": "s1", "subscription_name": "Netflix", "vendor": "netflix", "vendor_name": "Netflix",
			"wallet_id": string(walletID), "due_date": "2026-10-31", "amount": "43.00", "currency": "PLN", "days_before": "3"}})
	f.must(err)
}

func (f *notifyFamily) dispatch() usecase.DispatchResult {
	f.t.Helper()
	r, err := usecase.NewDispatchEvents(f.deps).Do(ctx)
	f.must(err)
	return r
}

func (f *notifyFamily) channel(as domain.ID, in usecase.CreateChannelInput) usecase.ChannelView {
	f.t.Helper()
	u, err := f.Users.ByID(ctx, as)
	f.must(err)
	v, err := usecase.NewCreateChannel(f.deps).Do(ctx, human(u), in)
	f.must(err)
	return v
}

func recipientsOf(ns []notification.Notification, eventID domain.ID) []domain.ID {
	out := []domain.ID{}
	for _, n := range ns {
		if n.EventID == eventID {
			out = append(out, n.UserID)
		}
	}
	slices.Sort(out)
	return out
}

func TestDispatchRecipients(t *testing.T) {
	f := newNotifyFamily(t)
	f.addBot("b1", "h1", f.member, bot.ScopeNotificationsRead)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	f.appendEvent("ev2", event.SubscriptionRenewalUpcoming, "w1")
	f.appendEvent("ev3", event.SubscriptionRenewalUpcoming, "wp")
	f.appendEvent("ev4", event.SubscriptionRenewalUpcoming, "gone")
	r := f.dispatch()
	if r.Events != 4 || r.Notifications != 4+4+2 || r.Deliveries != 0 {
		t.Errorf("result = %+v", r)
	}
	all := f.Notifications.All()
	everyone := []domain.ID{"admin", "member", "owner", "viewer"}
	for ev, want := range map[domain.ID][]domain.ID{"ev1": everyone, "ev2": everyone, "ev3": {"member", "owner"}, "ev4": {}} {
		if got := recipientsOf(all, ev); !slices.Equal(got, want) {
			t.Errorf("recipients of %s = %v, want %v", ev, got, want)
		}
	}
	n := all[0]
	if n.Title != "Odnowienie: Netflix za 3 dni" || n.Link != "/h/h1/subscriptions/s1" || n.HouseholdID != "h1" ||
		n.ResourceType != "subscription" || n.ResourceID != "s1" || n.Read() {
		t.Errorf("notification = %+v", n)
	}
	if pending, _ := f.Events.ListPending(ctx, 10); len(pending) != 0 {
		t.Errorf("pending after dispatch = %d", len(pending))
	}
	if r := f.dispatch(); r != (usecase.DispatchResult{}) {
		t.Errorf("second run = %+v", r)
	}

	// An inactive person and one who left get nothing.
	viewer := f.viewer
	viewer.Active = false
	f.must(f.Users.Update(ctx, viewer))
	f.must(f.Memberships.Delete(ctx, "h1", "admin"))
	f.appendEvent("ev5", event.SubscriptionChargeMissed, "")
	f.dispatch()
	if got := recipientsOf(f.Notifications.All(), "ev5"); !slices.Equal(got, []domain.ID{"member", "owner"}) {
		t.Errorf("recipients after changes = %v", got)
	}
}

func TestDispatchPreferences(t *testing.T) {
	f := newNotifyFamily(t)
	hook := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Hook",
		Config: map[string]string{"url": "https://hooks.example.com/a"}, Secrets: map[string]string{"secret": "s3cret"}})
	off := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Off",
		Config: map[string]string{"url": "https://hooks.example.com/b"}, Enabled: domain.Some(false)})
	other := f.channel("member", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Member hook",
		Config: map[string]string{"url": "https://hooks.example.com/c"}})
	_, err := usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.owner), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, InApp: false, ChannelIDs: []domain.ID{hook.ID, off.ID}},
	})
	noErr(t, err)
	_, err = usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.owner), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, ChannelIDs: []domain.ID{other.ID}},
	})
	wantField(t, err, "items.channel_ids")

	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "w1")
	f.appendEvent("ev2", event.SubscriptionChargeMissed, "w1")
	r := f.dispatch()
	if r.Deliveries != 1 {
		t.Fatalf("result = %+v", r)
	}
	if got := recipientsOf(f.Notifications.All(), "ev1"); !slices.Equal(got, []domain.ID{"admin", "member", "viewer"}) {
		t.Errorf("in-app of ev1 = %v (the owner turned it off)", got)
	}
	if got := recipientsOf(f.Notifications.All(), "ev2"); len(got) != 4 {
		t.Errorf("in-app of ev2 = %v (defaults)", got)
	}
	d := f.Deliveries.All()[0]
	if d.ChannelID != hook.ID || d.UserID != "owner" || d.URL != "https://denar.example.com/h/h1/subscriptions/s1" ||
		d.Status != notification.Pending || d.Data["due_date"] != "2026-10-31" || d.Title != "Odnowienie: Netflix za 3 dni" {
		t.Errorf("delivery = %+v", d)
	}

	// The worker sends it with the secret opened and the household's name.
	res, err := usecase.NewDeliverNotifications(f.deps).Do(ctx)
	noErr(t, err)
	if res.Sent != 1 {
		t.Fatalf("deliver = %+v", res)
	}
	sent := f.hooks.Messages()
	if len(sent) != 1 || sent[0].Target.Secrets["secret"] != "s3cret" || sent[0].Target.Config["url"] != "https://hooks.example.com/a" ||
		sent[0].Message.HouseholdName != "Dom h1" || sent[0].Message.ID != string(d.ID) || sent[0].Message.URL != d.URL {
		t.Errorf("sent = %+v", sent)
	}
	c, _ := f.Channels.ByID(ctx, "owner", hook.ID)
	if c.VerifiedAt.IsZero() || c.LastError != "" {
		t.Errorf("channel after success = %+v", c)
	}
	if res, _ := usecase.NewDeliverNotifications(f.deps).Do(ctx); res != (usecase.DeliveryResult{}) {
		t.Errorf("second run = %+v", res)
	}
}

func TestDeliveryRetriesWithBackoff(t *testing.T) {
	f := newNotifyFamily(t)
	hook := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Hook",
		Config: map[string]string{"url": "https://hooks.example.com/a"}})
	_, err := usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.owner), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, InApp: true, ChannelIDs: []domain.ID{hook.ID}}})
	noErr(t, err)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	f.dispatch()
	deliver := usecase.NewDeliverNotifications(f.deps)
	for range notification.MaxAttempts {
		f.hooks.Errs = append(f.hooks.Errs, errors.New("HTTP 503"))
	}
	start := f.Clock.Now()
	waits := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute}
	for i, wait := range waits {
		r, err := deliver.Do(ctx)
		noErr(t, err)
		if r.Retried != 1 {
			t.Fatalf("attempt %d = %+v", i+1, r)
		}
		d := f.Deliveries.All()[0]
		if d.Attempts != i+1 || !d.NextAttemptAt.Equal(f.Clock.Now().Add(wait)) || d.LastError != "HTTP 503" {
			t.Fatalf("after attempt %d: %+v", i+1, d)
		}
		// Not due a second before.
		f.Clock.Advance(wait - time.Second)
		if r, _ := deliver.Do(ctx); r != (usecase.DeliveryResult{}) {
			t.Fatalf("ran before the backoff: %+v", r)
		}
		f.Clock.Advance(time.Second)
		if c, _ := f.Channels.ByID(ctx, "owner", hook.ID); c.LastError != "" {
			t.Fatalf("channel error before the last attempt: %q", c.LastError)
		}
	}
	r, err := deliver.Do(ctx)
	noErr(t, err)
	d := f.Deliveries.All()[0]
	if r.Failed != 1 || d.Status != notification.Failed || d.Attempts != notification.MaxAttempts {
		t.Fatalf("last attempt = %+v, %+v", r, d)
	}
	c, _ := f.Channels.ByID(ctx, "owner", hook.ID)
	if c.LastError != "HTTP 503" || c.LastErrorAt.IsZero() {
		t.Errorf("channel = %+v", c)
	}
	if f.Clock.Now().Sub(start) != 31*time.Minute {
		t.Errorf("took %v", f.Clock.Now().Sub(start))
	}
	if len(f.hooks.Messages()) != 0 {
		t.Error("a failing notifier sent")
	}
}

func TestDeliveryToUnusableChannels(t *testing.T) {
	f := newNotifyFamily(t)
	tg := f.channel("owner", usecase.CreateChannelInput{Type: notification.Telegram, Name: "TG",
		Config: map[string]string{"chat_id": "42"}, Secrets: map[string]string{"bot_token": "1:abc"}})
	off := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Off",
		Config: map[string]string{"url": "https://hooks.example.com/b"}})
	gone := f.channel("owner", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Gone",
		Config: map[string]string{"url": "https://hooks.example.com/c"}})
	_, err := usecase.NewSetNotificationPreferences(f.deps).Do(ctx, human(f.owner), "h1", []notification.Preference{
		{Type: event.SubscriptionRenewalUpcoming, InApp: true, ChannelIDs: []domain.ID{tg.ID, off.ID, gone.ID}}})
	noErr(t, err)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	if r := f.dispatch(); r.Deliveries != 3 {
		t.Fatalf("dispatch = %+v", r)
	}
	_, err = usecase.NewUpdateChannel(f.deps).Do(ctx, human(f.owner), off.ID, usecase.UpdateChannelInput{Enabled: domain.Some(false)})
	noErr(t, err)
	noErr(t, usecase.NewDeleteChannel(f.deps).Do(ctx, human(f.owner), gone.ID))
	// The server lost its master key.
	f.deps.Secrets = nil
	r, err := usecase.NewDeliverNotifications(f.deps).Do(ctx)
	noErr(t, err)
	if r.Failed != 2 || r.Sent != 0 || len(f.Deliveries.All()) != 2 {
		t.Fatalf("deliver = %+v, %d deliveries", r, len(f.Deliveries.All()))
	}
	for _, d := range f.Deliveries.All() {
		if d.Status != notification.Failed || d.Attempts != 0 || d.LastError == "" {
			t.Errorf("delivery = %+v", d)
		}
	}
	c, _ := f.Channels.ByID(ctx, "owner", tg.ID)
	if !strings.Contains(c.LastError, "DENAR_MASTER_KEY") {
		t.Errorf("telegram channel error = %q", c.LastError)
	}
}

func TestChannels(t *testing.T) {
	f := newNotifyFamily(t)
	create := usecase.NewCreateChannel(f.deps)
	p := human(f.member)

	v, err := create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Ntfy, Name: " Telefon ",
		Config: map[string]string{"topic": "denar-dom"}, Secrets: map[string]string{"token": "tk_secret"}})
	noErr(t, err)
	if v.Name != "Telefon" || !v.HasSecret || v.SealedSecrets != nil || v.Config["server"] != notification.DefaultNtfyServer || !v.Enabled {
		t.Errorf("view = %+v", v)
	}
	stored, _ := f.Channels.ByID(ctx, "member", v.ID)
	if len(stored.SealedSecrets) == 0 || strings.Contains(string(stored.SealedSecrets), "tk_secret") {
		t.Errorf("secrets stored in the clear: %q", stored.SealedSecrets)
	}
	_, err = create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "telefon", Config: map[string]string{"url": "https://x.example.com"}})
	wantConflict(t, err, domain.ConflictDuplicateName)

	mail, err := create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Email, Name: "Poczta"})
	noErr(t, err)
	if mail.Config["address"] != "member@example.com" || mail.HasSecret {
		t.Errorf("email = %+v", mail)
	}
	_, err = create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Matrix, Name: "M", Config: map[string]string{
		"homeserver": "https://matrix.org", "room_id": "!r:matrix.org"}})
	wantField(t, err, "secrets.access_token")
	_, err = create.Do(ctx, asBot(f.addBot("b1", "h1", f.member, bot.ScopeNotificationsRead)), usecase.CreateChannelInput{Type: notification.Email, Name: "X"})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)

	// Without a master key, secrets are refused, e-mail and plain webhooks work.
	noKey := f.deps
	noKey.Secrets = nil
	_, err = usecase.NewCreateChannel(noKey).Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "Signed",
		Config: map[string]string{"url": "https://x.example.com"}, Secrets: map[string]string{"secret": "s"}})
	wantConflict(t, err, domain.ConflictSecretsUnavailable)
	_, err = usecase.NewCreateChannel(noKey).Do(ctx, p, usecase.CreateChannelInput{Type: notification.Telegram, Name: "TG",
		Config: map[string]string{"chat_id": "1"}, Secrets: map[string]string{"bot_token": "x"}})
	wantConflict(t, err, domain.ConflictSecretsUnavailable)
	_, err = usecase.NewCreateChannel(noKey).Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "Plain",
		Config: map[string]string{"url": "https://x.example.com"}})
	noErr(t, err)
	// Without SMTP, e-mail is refused.
	noSMTP := f.deps
	noSMTP.Mailer = nil
	noSMTP.Notifiers = map[notification.ChannelType]usecase.Notifier{notification.Webhook: f.hooks}
	_, err = usecase.NewCreateChannel(noSMTP).Do(ctx, p, usecase.CreateChannelInput{Type: notification.Email, Name: "E"})
	wantConflict(t, err, domain.ConflictSMTPNotConfigured)
	types, err := usecase.NewListChannelTypes(noKey).Do(ctx, p)
	noErr(t, err)
	reasons := map[notification.ChannelType]string{}
	for _, ti := range types {
		reasons[ti.Type] = ti.UnavailableReason
		if ti.Available != (ti.UnavailableReason == "") {
			t.Errorf("type = %+v", ti)
		}
	}
	if reasons[notification.Email] != "" || reasons[notification.Webhook] != "" || reasons[notification.Telegram] != usecase.UnavailableSecrets ||
		reasons[notification.Matrix] != usecase.UnavailableSecrets {
		t.Errorf("reasons without a key = %v", reasons)
	}
	types, _ = usecase.NewListChannelTypes(noSMTP).Do(ctx, p)
	if types[0].Type != notification.Email || types[0].UnavailableReason != usecase.UnavailableSMTP {
		t.Errorf("email without SMTP = %+v", types[0])
	}

	// Update: secrets replaced or cleared, config checked, others' not found.
	update := usecase.NewUpdateChannel(f.deps)
	u, err := update.Do(ctx, p, v.ID, usecase.UpdateChannelInput{Secrets: domain.Some[map[string]string](nil), Name: domain.Some("Nowy")})
	noErr(t, err)
	if u.HasSecret || u.Name != "Nowy" {
		t.Errorf("cleared = %+v", u)
	}
	_, err = update.Do(ctx, p, v.ID, usecase.UpdateChannelInput{Config: domain.Some(map[string]string{"topic": "a b"})})
	wantField(t, err, "config.topic")
	_, err = update.Do(ctx, human(f.owner), v.ID, usecase.UpdateChannelInput{Name: domain.Some("X")})
	wantErr(t, err, domain.ErrNotFound)
	list, err := usecase.NewListChannels(f.deps).Do(ctx, p)
	noErr(t, err)
	if len(list) != 3 || list[0].Name != "Nowy" || list[1].Name != "Plain" || list[2].Name != "Poczta" {
		t.Errorf("list = %+v", list)
	}
	_, err = usecase.NewGetChannel(f.deps).Do(ctx, human(f.owner), v.ID)
	wantErr(t, err, domain.ErrNotFound)
	wantErr(t, usecase.NewDeleteChannel(f.deps).Do(ctx, human(f.owner), v.ID), domain.ErrNotFound)

	// At most 20.
	for i := len(list); i < notification.MaxChannels; i++ {
		_, err := create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "H" + string(rune('a'+i)),
			Config: map[string]string{"url": "https://x.example.com"}})
		noErr(t, err)
	}
	_, err = create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "One too many", Config: map[string]string{"url": "https://x.example.com"}})
	wantConflict(t, err, domain.ConflictTooManyChannels)
}

func TestTestChannel(t *testing.T) {
	f := newNotifyFamily(t)
	c := f.channel("member", usecase.CreateChannelInput{Type: notification.Webhook, Name: "Hook", Config: map[string]string{"url": "https://x.example.com"}})
	test := usecase.NewTestChannel(f.deps)
	f.hooks.Errs = []error{errors.New("HTTP 404")}
	res, err := test.Do(ctx, human(f.member), c.ID)
	noErr(t, err)
	if res.OK || res.Error != "HTTP 404" || res.TestedAt.IsZero() {
		t.Errorf("failed test = %+v", res)
	}
	stored, _ := f.Channels.ByID(ctx, "member", c.ID)
	if stored.LastError != "HTTP 404" || !stored.VerifiedAt.IsZero() {
		t.Errorf("after the failed test = %+v", stored)
	}
	res, err = test.Do(ctx, human(f.member), c.ID)
	noErr(t, err)
	if !res.OK || res.Error != "" {
		t.Errorf("test = %+v", res)
	}
	stored, _ = f.Channels.ByID(ctx, "member", c.ID)
	if stored.LastError != "" || stored.VerifiedAt.IsZero() {
		t.Errorf("after the test = %+v", stored)
	}
	if msg := f.hooks.Messages()[0].Message; msg.Type != "test" || msg.Title != "Test kanału Denar" {
		t.Errorf("message = %+v", msg)
	}
	_, err = test.Do(ctx, human(f.owner), c.ID)
	wantErr(t, err, domain.ErrNotFound)
}

func TestNotificationsInTheApp(t *testing.T) {
	f := newNotifyFamily(t)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	f.appendEvent("ev2", event.SubscriptionRenewalUpcoming, "wp")
	f.appendEvent("ev3", event.SubscriptionChargeMissed, "w1")
	f.dispatch()
	list := usecase.NewListNotifications(f.deps)
	count := usecase.NewCountUnreadNotifications(f.deps)

	page, err := list.Do(ctx, human(f.member), usecase.ListNotificationsInput{Limit: domain.Some(2)})
	noErr(t, err)
	if len(page.Items) != 2 || page.NextCursor == "" || page.Items[0].EventID != "ev3" {
		t.Fatalf("first page = %+v", page)
	}
	page, err = list.Do(ctx, human(f.member), usecase.ListNotificationsInput{Limit: domain.Some(2), Cursor: page.NextCursor})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].EventID != "ev1" || page.NextCursor != "" {
		t.Fatalf("second page = %+v", page)
	}
	_, err = list.Do(ctx, human(f.member), usecase.ListNotificationsInput{Cursor: "junk"})
	wantField(t, err, "cursor")
	_, err = list.Do(ctx, human(f.outsider), usecase.ListNotificationsInput{HouseholdID: "h1"})
	wantErr(t, err, domain.ErrNotFound)
	if n, _ := count.Do(ctx, human(f.viewer), "h1"); n != 2 {
		t.Errorf("viewer's unread = %d", n)
	}

	mark := usecase.NewMarkNotificationRead(f.deps)
	first := page.Items[0]
	n, err := mark.Do(ctx, human(f.member), first.ID)
	noErr(t, err)
	if !n.Read() {
		t.Errorf("marked = %+v", n)
	}
	_, err = mark.Do(ctx, human(f.owner), first.ID)
	wantErr(t, err, domain.ErrNotFound)
	page, _ = list.Do(ctx, human(f.member), usecase.ListNotificationsInput{UnreadOnly: true})
	if len(page.Items) != 2 {
		t.Errorf("unread = %d", len(page.Items))
	}
	marked, err := usecase.NewMarkAllNotificationsRead(f.deps).Do(ctx, human(f.member), "h1")
	noErr(t, err)
	if marked != 2 {
		t.Errorf("read-all = %d", marked)
	}
	if n, _ := count.Do(ctx, human(f.member), ""); n != 0 {
		t.Errorf("unread after read-all = %d", n)
	}

	// A bot reads its creator's notifications, within its wallet list.
	b := f.addBot("b1", "h1", f.owner, bot.ScopeNotificationsRead)
	page, err = list.Do(ctx, asBot(b), usecase.ListNotificationsInput{})
	noErr(t, err)
	if len(page.Items) != 3 {
		t.Errorf("bot sees %d", len(page.Items))
	}
	b.WalletIDs = []domain.ID{"w1"}
	f.must(f.Bots.Update(ctx, b))
	page, err = list.Do(ctx, asBot(b), usecase.ListNotificationsInput{})
	noErr(t, err)
	if got := []domain.ID{page.Items[0].EventID, page.Items[1].EventID}; len(page.Items) != 2 || !slices.Equal(got, []domain.ID{"ev3", "ev1"}) {
		t.Errorf("bot with w1 sees %+v", page.Items)
	}
	if n, _ := count.Do(ctx, asBot(b), ""); n != 2 {
		t.Errorf("bot's unread = %d", n)
	}
	_, err = list.Do(ctx, asBot(b), usecase.ListNotificationsInput{HouseholdID: "h2"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = mark.Do(ctx, asBot(b), first.ID)
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	_, err = usecase.NewMarkAllNotificationsRead(f.deps).Do(ctx, asBot(b), "")
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	noScope := f.addBot("b2", "h1", f.owner, bot.ScopeWalletsRead)
	_, err = list.Do(ctx, asBot(noScope), usecase.ListNotificationsInput{})
	wantForbidden(t, err, domain.ForbiddenScope)
}

func TestNotificationPreferences(t *testing.T) {
	f := newNotifyFamily(t)
	get := usecase.NewGetNotificationPreferences(f.deps)
	set := usecase.NewSetNotificationPreferences(f.deps)
	prefs, err := get.Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if len(prefs) != len(event.Types) {
		t.Fatalf("prefs = %+v", prefs)
	}
	for _, p := range prefs {
		if !p.InApp || len(p.ChannelIDs) != 0 {
			t.Errorf("default = %+v", p)
		}
	}
	c := f.channel("viewer", usecase.CreateChannelInput{Type: notification.Email, Name: "Mail"})
	prefs, err = set.Do(ctx, human(f.viewer), "h1", []notification.Preference{{Type: event.EnvelopeOverspent, InApp: false, ChannelIDs: []domain.ID{c.ID}}})
	noErr(t, err)
	p := notification.For(prefs, event.EnvelopeOverspent)
	if p.InApp || !slices.Equal(p.ChannelIDs, []domain.ID{c.ID}) || len(prefs) != len(event.Types) {
		t.Errorf("set = %+v", prefs)
	}
	noErr(t, usecase.NewDeleteChannel(f.deps).Do(ctx, human(f.viewer), c.ID))
	prefs, _ = get.Do(ctx, human(f.viewer), "h1")
	if p := notification.For(prefs, event.EnvelopeOverspent); len(p.ChannelIDs) != 0 || p.InApp {
		t.Errorf("after the channel went = %+v", p)
	}
	_, err = set.Do(ctx, human(f.viewer), "h1", []notification.Preference{{Type: "nope"}})
	wantField(t, err, "items.type")
	_, err = get.Do(ctx, human(f.outsider), "h1")
	wantErr(t, err, domain.ErrNotFound)
	_, err = set.Do(ctx, human(f.outsider), "h1", nil)
	wantErr(t, err, domain.ErrNotFound)
	_, err = get.Do(ctx, asBot(f.addBot("b1", "h1", f.owner, bot.ScopeNotificationsRead)), "h1")
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
}

func TestCheckBudgets(t *testing.T) {
	f := newNotifyFamily(t)
	f.addEnvelopes("h1", "food", "trip", "later")
	trip, _ := f.Envelopes.ByID(ctx, "h1", "trip")
	trip.Name, trip.Goal = "Wakacje", budget.Goal{Type: budget.GoalTargetByDate, Amount: 300000, TargetDate: day("2026-10-26")}
	f.must(f.Envelopes.Update(ctx, trip))
	later, _ := f.Envelopes.ByID(ctx, "h1", "later")
	later.Goal = budget.Goal{Type: budget.GoalTargetByDate, Amount: 1000, TargetDate: day("2026-11-30")}
	f.must(f.Envelopes.Update(ctx, later))
	_, err := usecase.NewSetAssignment(f.deps).Do(ctx, human(f.member), "h1", "trip", usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("2500.00")})
	noErr(t, err)
	_, err = usecase.NewRecordTransaction(f.deps).Do(ctx, human(f.member), "h1", usecase.RecordTransactionInput{WalletID: "w1",
		Amount: pln("-12.34"), Date: day("2026-10-20"), EnvelopeID: "food"})
	noErr(t, err)

	check := usecase.NewCheckBudgets(f.deps)
	n, err := check.Do(ctx)
	noErr(t, err)
	if n != 2 {
		t.Fatalf("events = %d", n)
	}
	var over, goal event.Event
	for _, e := range f.Events.All() {
		switch e.Type {
		case event.EnvelopeOverspent:
			over = e
		case event.EnvelopeGoalDue:
			goal = e
		}
	}
	if over.ResourceID != "food" || over.DedupeKey != "overspent:food:2026-10" || over.Payload["available"] != "-12.34" ||
		over.Payload["month"] != "2026-10" || over.Payload["wallet_id"] != "" || over.ResourceType != "envelope" {
		t.Errorf("overspent = %+v", over)
	}
	if goal.ResourceID != "trip" || goal.DedupeKey != "goal_due:trip:2026-10-26" || goal.Payload["missing"] != "500.00" ||
		goal.Payload["goal_amount"] != "3000.00" || goal.Payload["days_left"] != "5" || goal.Payload["envelope_name"] != "Wakacje" {
		t.Errorf("goal = %+v", goal)
	}
	if n, _ := check.Do(ctx); n != 0 {
		t.Errorf("second run = %d", n)
	}
	f.dispatch()
	var texts []string
	for _, nt := range f.Notifications.All() {
		if nt.UserID == "viewer" {
			texts = append(texts, nt.Title+" → "+nt.Link)
		}
	}
	slices.Sort(texts)
	want := []string{"Cel koperty Wakacje za 5 dni → /h/h1/budget/2026-10", "Przekroczona koperta: food → /h/h1/budget/2026-10"}
	if !slices.Equal(texts, want) {
		t.Errorf("viewer's notifications = %q", texts)
	}
}

func TestPruneNotifications(t *testing.T) {
	f := newNotifyFamily(t)
	f.appendEvent("ev1", event.SubscriptionRenewalUpcoming, "")
	f.dispatch()
	_, err := usecase.NewMarkAllNotificationsRead(f.deps).Do(ctx, human(f.owner), "")
	noErr(t, err)
	prune := usecase.NewPruneNotifications(f.deps)
	f.Clock.Advance(179 * 24 * time.Hour)
	if r, _ := prune.Do(ctx); r != (usecase.PruneResult{}) {
		t.Errorf("too early = %+v", r)
	}
	f.Clock.Advance(2 * 24 * time.Hour)
	r, err := prune.Do(ctx)
	noErr(t, err)
	if r.Notifications != 1 || r.Events != 0 {
		t.Errorf("after 181 days = %+v", r)
	}
	f.Clock.Advance(220 * 24 * time.Hour)
	if r, _ := prune.Do(ctx); r.Events != 1 {
		t.Errorf("after 401 days = %+v", r)
	}
}

func TestInvitationByEmail(t *testing.T) {
	f := newNotifyFamily(t)
	inv := usecase.NewCreateInvitation(f.deps, usecase.InvitationConfig{TTL: time.Hour, BaseURL: "https://denar.example.com"})
	v, err := inv.Do(ctx, human(f.admin), "h1", usecase.CreateInvitationInput{Email: "Grace@Example.com", Role: membership.RoleMember, SendEmail: true})
	noErr(t, err)
	if !v.EmailSent || len(f.mailer.Sent) != 1 {
		t.Fatalf("invitation = %+v, mail %+v", v, f.mailer.Sent)
	}
	m := f.mailer.Sent[0]
	if m.To != "grace@example.com" || !strings.Contains(m.Subject, "Dom h1") || !strings.Contains(m.Body, "https://denar.example.com/invite#"+v.Token) {
		t.Errorf("mail = %+v", m)
	}
	f.mailer.Err = errors.New("smtp: 550")
	v, err = inv.Do(ctx, human(f.admin), "h1", usecase.CreateInvitationInput{Email: "x@example.com", Role: membership.RoleMember, SendEmail: true})
	noErr(t, err)
	if v.EmailSent || v.EmailError == "" || v.Invitation.ID == "" {
		t.Errorf("failed mail = %+v", v)
	}
	noMail := f.deps
	noMail.Mailer = nil
	_, err = usecase.NewCreateInvitation(noMail, usecase.InvitationConfig{TTL: time.Hour}).Do(ctx, human(f.admin), "h1",
		usecase.CreateInvitationInput{Email: "y@example.com", Role: membership.RoleMember, SendEmail: true})
	wantConflict(t, err, domain.ConflictSMTPNotConfigured)
}

// TestChannelPrivateTargets: channels aimed at localhost or a private
// literal address are refused unless the administrator opted in (009).
func TestChannelPrivateTargets(t *testing.T) {
	f := newNotifyFamily(t)
	p := human(f.member)
	create := usecase.NewCreateChannel(f.deps)
	for _, in := range []usecase.CreateChannelInput{
		{Type: notification.Webhook, Name: "Lokalny", Config: map[string]string{"url": "http://localhost:5678/webhook"}},
		{Type: notification.Webhook, Name: "Metadane", Config: map[string]string{"url": "http://169.254.169.254/latest"}},
		{Type: notification.Ntfy, Name: "LAN", Config: map[string]string{"server": "http://192.168.1.5", "topic": "dom"}},
		{Type: notification.Matrix, Name: "M", Config: map[string]string{"homeserver": "http://[::1]:8008", "room_id": "!r:x"},
			Secrets: map[string]string{"access_token": "t"}},
	} {
		_, err := create.Do(ctx, p, in)
		field := "config.url"
		switch in.Type {
		case notification.Ntfy:
			field = "config.server"
		case notification.Matrix:
			field = "config.homeserver"
		}
		wantField(t, err, field)
	}

	v, err := create.Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "Publiczny",
		Config: map[string]string{"url": "https://hooks.example.com/x"}})
	noErr(t, err)
	_, err = usecase.NewUpdateChannel(f.deps).Do(ctx, p, v.ID,
		usecase.UpdateChannelInput{Config: domain.Some(map[string]string{"url": "http://10.0.0.1/hook"})})
	wantField(t, err, "config.url")

	open := f.deps
	open.NotifyTargets = notification.TargetPolicy{AllowPrivate: true}
	_, err = usecase.NewCreateChannel(open).Do(ctx, p, usecase.CreateChannelInput{Type: notification.Webhook, Name: "n8n",
		Config: map[string]string{"url": "http://localhost:5678/webhook"}})
	noErr(t, err)
}
