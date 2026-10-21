package portstest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/notification"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func newNotification(env Env, id, userID, householdID, eventID, walletID domain.ID) notification.Notification {
	return notification.Notification{ID: id, UserID: userID, HouseholdID: householdID, EventID: eventID, Type: event.SubscriptionRenewalUpcoming,
		Title: "Odnowienie", Body: "Treść", Link: "/h/" + string(householdID) + "/subscriptions/s1", WalletID: walletID,
		ResourceType: "subscription", ResourceID: "s1", CreatedAt: now(env)}
}

func notificationIDs(ns []notification.Notification) []domain.ID {
	out := []domain.ID{}
	for _, n := range ns {
		out = append(out, n.ID)
	}
	return out
}

// RunNotificationSuite checks a NotificationRepository. It needs Users and
// Households.
func RunNotificationSuite(t *testing.T, factory Factory) {
	ctx := context.Background()
	setup := func(t *testing.T) Env {
		env := factory(t)
		seedHousehold(t, env, "u2")
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u2")))
		return env
	}

	t.Run("Create once per event and person, read back", func(t *testing.T) {
		env := setup(t)
		n := newNotification(env, "n1", "u1", "h1", "e1", "w1")
		stored, err := env.Notifications.Create(ctx, n)
		noError(t, err)
		if !stored {
			t.Fatal("not stored")
		}
		stored, err = env.Notifications.Create(ctx, newNotification(env, "n2", "u1", "h1", "e1", ""))
		noError(t, err)
		if stored {
			t.Error("second notification of the same event stored")
		}
		stored, _ = env.Notifications.Create(ctx, newNotification(env, "n3", "u2", "h1", "e1", ""))
		if !stored {
			t.Error("another person's notification of the event not stored")
		}
		got, err := env.Notifications.ByID(ctx, "u1", "n1")
		noError(t, err)
		if !got.CreatedAt.Equal(n.CreatedAt) || got.Read() {
			t.Errorf("times = %+v", got)
		}
		got.CreatedAt = n.CreatedAt
		if !reflect.DeepEqual(got, n) {
			t.Errorf("notification = %+v, want %+v", got, n)
		}
		_, err = env.Notifications.ByID(ctx, "u2", "n1")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("List newest first with filters and a cursor; count; mark read", func(t *testing.T) {
		env := setup(t)
		for _, n := range []notification.Notification{
			newNotification(env, "n1", "u1", "h1", "e1", ""),
			newNotification(env, "n2", "u1", "h1", "e2", "w1"),
			newNotification(env, "n3", "u1", "h2", "e3", ""),
			newNotification(env, "n4", "u1", "h1", "e4", "w2"),
			newNotification(env, "n5", "u2", "h1", "e5", ""),
		} {
			_, err := env.Notifications.Create(ctx, n)
			noError(t, err)
		}
		all := usecase.NotificationFilter{UserID: "u1"}
		page, next, err := env.Notifications.List(ctx, all, usecase.Page{Limit: 3})
		noError(t, err)
		sameOrder(t, "first page", notificationIDs(page), []domain.ID{"n4", "n3", "n2"})
		page, next, err = env.Notifications.List(ctx, all, usecase.Page{Cursor: next, Limit: 3})
		noError(t, err)
		sameOrder(t, "second page", notificationIDs(page), []domain.ID{"n1"})
		if next != "" {
			t.Errorf("next after the last page = %q", next)
		}
		_, _, err = env.Notifications.List(ctx, usecase.NotificationFilter{UserID: "u1", HouseholdID: "h1"}, usecase.Page{Cursor: "garbage", Limit: 3})
		wantError(t, err, domain.ErrInvalid)

		h1 := usecase.NotificationFilter{UserID: "u1", HouseholdID: "h1"}
		page, _, _ = env.Notifications.List(ctx, h1, usecase.Page{Limit: 10})
		sameOrder(t, "h1", notificationIDs(page), []domain.ID{"n4", "n2", "n1"})
		bot := usecase.NotificationFilter{UserID: "u1", HouseholdID: "h1", RestrictWallets: true, WalletIDs: []domain.ID{"w1"}}
		page, _, _ = env.Notifications.List(ctx, bot, usecase.Page{Limit: 10})
		sameOrder(t, "wallets", notificationIDs(page), []domain.ID{"n2", "n1"})
		noWallets := usecase.NotificationFilter{UserID: "u1", RestrictWallets: true}
		page, _, _ = env.Notifications.List(ctx, noWallets, usecase.Page{Limit: 10})
		sameOrder(t, "no wallets", notificationIDs(page), []domain.ID{"n3", "n1"})

		count, err := env.Notifications.CountUnread(ctx, h1)
		noError(t, err)
		if count != 3 {
			t.Errorf("unread = %d", count)
		}
		at := now(env)
		noError(t, env.Notifications.MarkRead(ctx, "u1", "n2", at))
		wantError(t, env.Notifications.MarkRead(ctx, "u2", "n2", at), domain.ErrNotFound)
		wantError(t, env.Notifications.MarkRead(ctx, "u1", "nope", at), domain.ErrNotFound)
		noError(t, env.Notifications.MarkRead(ctx, "u1", "n2", at.Add(time.Hour)))
		got, _ := env.Notifications.ByID(ctx, "u1", "n2")
		if !got.ReadAt.Equal(at) {
			t.Errorf("read at = %v, want the first time %v", got.ReadAt, at)
		}
		unread := usecase.NotificationFilter{UserID: "u1", HouseholdID: "h1", UnreadOnly: true}
		page, _, _ = env.Notifications.List(ctx, unread, usecase.Page{Limit: 10})
		sameOrder(t, "unread", notificationIDs(page), []domain.ID{"n4", "n1"})

		n, err := env.Notifications.MarkAllRead(ctx, h1, at)
		noError(t, err)
		if n != 2 {
			t.Errorf("marked = %d", n)
		}
		if c, _ := env.Notifications.CountUnread(ctx, all); c != 1 {
			t.Errorf("unread of u1 after h1 = %d", c)
		}
		if c, _ := env.Notifications.CountUnread(ctx, usecase.NotificationFilter{UserID: "u2"}); c != 1 {
			t.Errorf("unread of u2 = %d", c)
		}
	})

	t.Run("DeleteReadBefore keeps unread and recently read", func(t *testing.T) {
		env := setup(t)
		for _, id := range []domain.ID{"n1", "n2", "n3"} {
			_, err := env.Notifications.Create(ctx, newNotification(env, id, "u1", "h1", "e"+id, ""))
			noError(t, err)
		}
		t0 := now(env)
		noError(t, env.Notifications.MarkRead(ctx, "u1", "n1", t0.Add(-200*24*time.Hour)))
		noError(t, env.Notifications.MarkRead(ctx, "u1", "n2", t0.Add(-10*24*time.Hour)))
		n, err := env.Notifications.DeleteReadBefore(ctx, t0.Add(-180*24*time.Hour))
		noError(t, err)
		if n != 1 {
			t.Errorf("deleted = %d", n)
		}
		page, _, _ := env.Notifications.List(ctx, usecase.NotificationFilter{UserID: "u1"}, usecase.Page{Limit: 10})
		sameOrder(t, "left", notificationIDs(page), []domain.ID{"n3", "n2"})
	})

	t.Run("Household deletion removes its notifications", func(t *testing.T) {
		env := setup(t)
		_, _ = env.Notifications.Create(ctx, newNotification(env, "n1", "u1", "h1", "e1", ""))
		_, _ = env.Notifications.Create(ctx, newNotification(env, "n2", "u1", "h2", "e2", ""))
		noError(t, env.Households.Delete(ctx, "h1"))
		page, _, _ := env.Notifications.List(ctx, usecase.NotificationFilter{UserID: "u1"}, usecase.Page{Limit: 10})
		sameOrder(t, "left", notificationIDs(page), []domain.ID{"n2"})
	})
}

func newChannel(env Env, id, userID domain.ID, name string) notification.Channel {
	return notification.Channel{ID: id, UserID: userID, Type: notification.Webhook, Name: name,
		Config: map[string]string{"url": "https://hooks.example.com/" + string(id)}, Enabled: true, CreatedAt: now(env), UpdatedAt: now(env)}
}

func newDelivery(env Env, id, eventID, userID, householdID, channelID domain.ID, due time.Time) notification.Delivery {
	return notification.Delivery{ID: id, EventID: eventID, UserID: userID, HouseholdID: householdID, ChannelID: channelID,
		Type: event.SubscriptionChargeMissed, Title: "Brak płatności", Body: "Treść", URL: "https://denar.example.com/h/h1",
		Data: map[string]string{"charge_id": "c1"}, OccurredAt: now(env), Status: notification.Pending, NextAttemptAt: due,
		CreatedAt: now(env), UpdatedAt: now(env)}
}

func deliveryIDs(ds []notification.Delivery) []domain.ID {
	out := []domain.ID{}
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

// RunChannelSuite checks a ChannelRepository with the DeliveryRepository
// it cascades to. It needs Users and Households.
func RunChannelSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create, read, update, list, delete", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		c := newChannel(env, "c1", "u1", "Hook")
		c.SealedSecrets = []byte{1, 2, 3}
		noError(t, env.Channels.Create(ctx, c))
		wantConflict(t, env.Channels.Create(ctx, newChannel(env, "c2", "u1", "hook")), domain.ConflictDuplicateName)
		noError(t, env.Channels.Create(ctx, newChannel(env, "c3", "u2", "Hook")))
		noError(t, env.Channels.Create(ctx, newChannel(env, "c4", "u1", "Alpha")))

		got, err := env.Channels.ByID(ctx, "u1", "c1")
		noError(t, err)
		if !got.CreatedAt.Equal(c.CreatedAt) || !got.UpdatedAt.Equal(c.UpdatedAt) {
			t.Errorf("times = %+v", got)
		}
		got.CreatedAt, got.UpdatedAt = c.CreatedAt, c.UpdatedAt
		if !reflect.DeepEqual(got, c) {
			t.Errorf("channel = %+v, want %+v", got, c)
		}
		_, err = env.Channels.ByID(ctx, "u2", "c1")
		wantError(t, err, domain.ErrNotFound)

		later := now(env).Add(time.Minute)
		c.Name, c.Config, c.SealedSecrets, c.Enabled = "Renamed", map[string]string{"url": "https://x.example.com"}, nil, false
		c.VerifiedAt, c.LastError, c.LastErrorAt, c.UpdatedAt = later, "HTTP 500", later, later
		c.Type = notification.Ntfy // never changes
		noError(t, env.Channels.Update(ctx, c))
		got, _ = env.Channels.ByID(ctx, "u1", "c1")
		if got.Type != notification.Webhook || got.Name != "Renamed" || got.Config["url"] != "https://x.example.com" || got.HasSecret() ||
			got.Enabled || !got.VerifiedAt.Equal(later) || got.LastError != "HTTP 500" || !got.LastErrorAt.Equal(later) || !got.UpdatedAt.Equal(later) {
			t.Errorf("updated = %+v", got)
		}
		wantConflict(t, env.Channels.Update(ctx, notification.Channel{ID: "c4", UserID: "u1", Name: "renamed", Config: map[string]string{}}),
			domain.ConflictDuplicateName)
		wantError(t, env.Channels.Update(ctx, notification.Channel{ID: "c3", UserID: "u1", Name: "X", Config: map[string]string{}}), domain.ErrNotFound)

		list, err := env.Channels.ListByUser(ctx, "u1")
		noError(t, err)
		var ids []domain.ID
		for _, x := range list {
			ids = append(ids, x.ID)
		}
		sameOrder(t, "channels", ids, []domain.ID{"c4", "c1"})

		wantError(t, env.Channels.Delete(ctx, "u2", "c1"), domain.ErrNotFound)
		noError(t, env.Channels.Delete(ctx, "u1", "c1"))
		_, err = env.Channels.ByID(ctx, "u1", "c1")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("Deleting a channel removes its deliveries", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Channels.Create(ctx, newChannel(env, "c1", "u1", "A")))
		noError(t, env.Channels.Create(ctx, newChannel(env, "c2", "u1", "B")))
		_, err := env.Deliveries.Create(ctx, newDelivery(env, "d1", "e1", "u1", "h1", "c1", now(env)))
		noError(t, err)
		_, err = env.Deliveries.Create(ctx, newDelivery(env, "d2", "e1", "u1", "h1", "c2", now(env)))
		noError(t, err)
		noError(t, env.Channels.Delete(ctx, "u1", "c1"))
		due, _ := env.Deliveries.ListDue(ctx, now(env), 10)
		sameOrder(t, "due", deliveryIDs(due), []domain.ID{"d2"})
	})
}

// RunPreferenceSuite checks a PreferenceRepository. It needs Users and
// Households.
func RunPreferenceSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Replace and list", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		got, err := env.Preferences.List(ctx, "u1", "h1")
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("before = %+v", got)
		}
		prefs := []notification.Preference{
			{Type: event.SubscriptionRenewalUpcoming, InApp: true, ChannelIDs: []domain.ID{"c1", "c2"}},
			{Type: event.EnvelopeOverspent, InApp: false, ChannelIDs: []domain.ID{}},
		}
		noError(t, env.Preferences.Replace(ctx, "u1", "h1", prefs))
		got, _ = env.Preferences.List(ctx, "u1", "h1")
		if !reflect.DeepEqual(got, prefs) {
			t.Errorf("prefs = %+v, want %+v", got, prefs)
		}
		if other, _ := env.Preferences.List(ctx, "u2", "h1"); len(other) != 0 {
			t.Errorf("another person's = %+v", other)
		}
		noError(t, env.Preferences.Replace(ctx, "u1", "h1", prefs[1:]))
		got, _ = env.Preferences.List(ctx, "u1", "h1")
		if len(got) != 1 || got[0].Type != event.EnvelopeOverspent {
			t.Errorf("replaced = %+v", got)
		}
		wantError(t, env.Preferences.Replace(ctx, "u1", "nope", prefs), domain.ErrNotFound)
		noError(t, env.Households.Delete(ctx, "h1"))
		if got, _ = env.Preferences.List(ctx, "u1", "h1"); len(got) != 0 {
			t.Errorf("after the household = %+v", got)
		}
	})
}

// RunDeliverySuite checks a DeliveryRepository. It needs Users,
// Households and Channels.
func RunDeliverySuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create once, list due, update, delete finished", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Channels.Create(ctx, newChannel(env, "c1", "u1", "A")))
		t0 := now(env)
		d1 := newDelivery(env, "d1", "e1", "u1", "h1", "c1", t0.Add(-time.Minute))
		stored, err := env.Deliveries.Create(ctx, d1)
		noError(t, err)
		if !stored {
			t.Fatal("not stored")
		}
		if stored, _ = env.Deliveries.Create(ctx, newDelivery(env, "dx", "e1", "u1", "h1", "c1", t0)); stored {
			t.Error("second delivery of the event to the channel stored")
		}
		_, _ = env.Deliveries.Create(ctx, newDelivery(env, "d2", "e2", "u1", "h1", "c1", t0.Add(-time.Hour)))
		_, _ = env.Deliveries.Create(ctx, newDelivery(env, "d3", "e3", "u1", "h1", "c1", t0.Add(time.Hour)))

		due, err := env.Deliveries.ListDue(ctx, t0, 10)
		noError(t, err)
		sameOrder(t, "due", deliveryIDs(due), []domain.ID{"d2", "d1"})
		if due, _ = env.Deliveries.ListDue(ctx, t0, 1); len(due) != 1 {
			t.Errorf("limit = %d", len(due))
		}
		got, err := env.Deliveries.ByID(ctx, "d1")
		noError(t, err)
		if !got.OccurredAt.Equal(d1.OccurredAt) || !got.NextAttemptAt.Equal(d1.NextAttemptAt) || !got.CreatedAt.Equal(d1.CreatedAt) {
			t.Errorf("times = %+v", got)
		}
		got.OccurredAt, got.NextAttemptAt, got.CreatedAt, got.UpdatedAt = d1.OccurredAt, d1.NextAttemptAt, d1.CreatedAt, d1.UpdatedAt
		if !reflect.DeepEqual(got, d1) {
			t.Errorf("delivery = %+v, want %+v", got, d1)
		}
		_, err = env.Deliveries.ByID(ctx, "nope")
		wantError(t, err, domain.ErrNotFound)

		sent := d1.Succeeded(t0.Add(-40 * 24 * time.Hour))
		noError(t, env.Deliveries.Update(ctx, sent))
		retry := newDelivery(env, "d2", "e2", "u1", "h1", "c1", t0).Failure(t0, "HTTP 503")
		noError(t, env.Deliveries.Update(ctx, retry))
		got, _ = env.Deliveries.ByID(ctx, "d2")
		if got.Attempts != 1 || got.LastError != "HTTP 503" || !got.NextAttemptAt.Equal(t0.Add(time.Minute)) || got.Status != notification.Pending {
			t.Errorf("retry = %+v", got)
		}
		got, _ = env.Deliveries.ByID(ctx, "d1")
		if got.Status != notification.Sent || !got.SentAt.Equal(sent.SentAt) {
			t.Errorf("sent = %+v", got)
		}
		wantError(t, env.Deliveries.Update(ctx, notification.Delivery{ID: "nope"}), domain.ErrNotFound)
		if due, _ = env.Deliveries.ListDue(ctx, t0, 10); len(due) != 0 {
			t.Errorf("due after = %v", deliveryIDs(due))
		}

		n, err := env.Deliveries.DeleteFinishedBefore(ctx, t0.Add(-30*24*time.Hour))
		noError(t, err)
		if n != 1 {
			t.Errorf("deleted = %d", n)
		}
		_, err = env.Deliveries.ByID(ctx, "d1")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Deliveries.ByID(ctx, "d2")
		noError(t, err)
	})
}

// RunNotificationHousekeepingSuite checks HouseholdRepository.ListIDs and
// EventOutbox.DeleteDispatchedBefore (stage 005).
func RunNotificationHousekeepingSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("ListIDs", func(t *testing.T) {
		env := factory(t)
		ids, err := env.Households.ListIDs(ctx)
		noError(t, err)
		if len(ids) != 0 {
			t.Errorf("ids = %v", ids)
		}
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		ids, _ = env.Households.ListIDs(ctx)
		sameSet(t, "ids", ids, []domain.ID{"h1", "h2"})
	})

	t.Run("DeleteDispatchedBefore keeps pending and recent events", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		t0 := now(env)
		for _, id := range []domain.ID{"e1", "e2", "e3"} {
			_, err := env.Events.Append(ctx, newEvent(env, id, "h1", "key:"+string(id)))
			noError(t, err)
		}
		noError(t, env.Events.MarkDispatched(ctx, "e1", t0.Add(-401*24*time.Hour)))
		noError(t, env.Events.MarkDispatched(ctx, "e2", t0.Add(-399*24*time.Hour)))
		n, err := env.Events.DeleteDispatchedBefore(ctx, t0.Add(-400*24*time.Hour))
		noError(t, err)
		if n != 1 {
			t.Errorf("deleted = %d", n)
		}
		stored, _ := env.Events.Append(ctx, newEvent(env, "e4", "h1", "key:e2"))
		if stored {
			t.Error("the kept event's key is free")
		}
		stored, _ = env.Events.Append(ctx, newEvent(env, "e5", "h1", "key:e1"))
		if !stored {
			t.Error("the deleted event's key is still taken")
		}
	})
}
