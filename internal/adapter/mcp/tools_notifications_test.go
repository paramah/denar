package mcp

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/notification"
)

func TestListNotifications(t *testing.T) {
	w := newWorld(t)
	for i, n := range []notification.Notification{
		{ID: "n1", WalletID: ""},
		{ID: "n2", WalletID: "w-card"},
		{ID: "n3", WalletID: "w-cash"},
	} {
		n.UserID, n.HouseholdID, n.EventID, n.Type = w.member.ID, "h1", domain.ID("ev"+string(rune('1'+i))), event.SubscriptionRenewalUpcoming
		n.Title, n.Body, n.Link, n.CreatedAt = "Odnowienie: Netflix jutro", "Netflix odnowi się 22.10.2026.", "/h/h1/subscriptions/s1", w.now()
		if _, err := w.Notifications.Create(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = w.Notifications.Create(ctx, notification.Notification{ID: "n9", UserID: w.owner.ID, HouseholdID: "h1", EventID: "ev1",
		Type: event.EnvelopeOverspent, Title: "Ada's", CreatedAt: w.now()})
	if err := w.Notifications.MarkRead(ctx, w.member.ID, "n1", w.now()); err != nil {
		t.Fatal(err)
	}

	s := w.connect(w.addBot("b1", w.member, []bot.Scope{bot.ScopeNotificationsRead}, "w-card"))
	a := ok(t, s, "list_notifications", nil)
	if got := a.list("notifications"); len(got) != 2 || a.str("notifications", 0, "id") != "n2" || a.str("notifications", 1, "id") != "n1" ||
		a.str("notifications", 0, "title") != "Odnowienie: Netflix jutro" || a.str("notifications", 0, "link") != "/h/h1/subscriptions/s1" ||
		a.get("notifications", 1, "read") != true || a.get("unread_count") != float64(1) || a.get("next_cursor") != nil {
		t.Errorf("notifications = %s", a.raw)
	}
	a = ok(t, s, "list_notifications", map[string]any{"unread_only": true, "limit": 1})
	if got := a.list("notifications"); len(got) != 1 || a.str("notifications", 0, "id") != "n2" {
		t.Errorf("unread = %s", a.raw)
	}

	s = w.connect(w.addBot("b2", w.member, []bot.Scope{bot.ScopeWalletsRead}))
	refused(t, s, "list_notifications", nil, codeForbidden)
}
