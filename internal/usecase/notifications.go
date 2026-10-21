package usecase

import (
	"context"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/notification"
)

// NotificationPage is one page of notifications.
type NotificationPage struct {
	Items      []notification.Notification
	NextCursor string
}

// notificationFilter returns whose notifications the principal reads. A
// person reads their own, optionally of one household they are a member
// of. A bot reads its creator's in its household, with notifications:read,
// without those of wallets outside its list (AD-503).
func (d Deps) notificationFilter(ctx context.Context, p Principal, householdID domain.ID) (NotificationFilter, error) {
	if !p.IsBot() {
		f := NotificationFilter{UserID: p.User.ID, HouseholdID: householdID}
		if householdID != "" {
			if _, err := d.access().household(ctx, p, householdID); err != nil {
				return NotificationFilter{}, err
			}
		}
		return f, nil
	}
	if householdID != "" && householdID != p.Bot.HouseholdID {
		return NotificationFilter{}, domain.ErrNotFound
	}
	s, err := d.access().household(ctx, p, p.Bot.HouseholdID)
	if err != nil {
		return NotificationFilter{}, err
	}
	if err := requireScope(s, bot.ScopeNotificationsRead); err != nil {
		return NotificationFilter{}, err
	}
	return NotificationFilter{UserID: s.Bot.CreatedBy, HouseholdID: s.Household.ID, RestrictWallets: len(s.WalletIDs) > 0,
		WalletIDs: s.WalletIDs}, nil
}

type ListNotificationsInput struct {
	HouseholdID domain.ID
	UnreadOnly  bool
	Cursor      string
	Limit       domain.Optional[int]
}

// ListNotifications lists notifications newest first (FR-550, FR-560).
type ListNotifications struct{ d Deps }

func NewListNotifications(d Deps) *ListNotifications { return &ListNotifications{d: d} }

func (uc *ListNotifications) Do(ctx context.Context, p Principal, in ListNotificationsInput) (NotificationPage, error) {
	page, err := NewPage(in.Cursor, in.Limit)
	if err != nil {
		return NotificationPage{}, err
	}
	f, err := uc.d.notificationFilter(ctx, p, in.HouseholdID)
	if err != nil {
		return NotificationPage{}, err
	}
	f.UnreadOnly = in.UnreadOnly
	items, next, err := uc.d.Notifications.List(ctx, f, page)
	if err != nil {
		return NotificationPage{}, cursorError(err, in.Cursor)
	}
	return NotificationPage{Items: items, NextCursor: next}, nil
}

// CountUnreadNotifications counts unread notifications, of one household
// or all.
type CountUnreadNotifications struct{ d Deps }

func NewCountUnreadNotifications(d Deps) *CountUnreadNotifications { return &CountUnreadNotifications{d: d} }

func (uc *CountUnreadNotifications) Do(ctx context.Context, p Principal, householdID domain.ID) (int, error) {
	f, err := uc.d.notificationFilter(ctx, p, householdID)
	if err != nil {
		return 0, err
	}
	return uc.d.Notifications.CountUnread(ctx, f)
}

// MarkNotificationRead marks one of the person's notifications read;
// people only.
type MarkNotificationRead struct{ d Deps }

func NewMarkNotificationRead(d Deps) *MarkNotificationRead { return &MarkNotificationRead{d: d} }

func (uc *MarkNotificationRead) Do(ctx context.Context, p Principal, id domain.ID) (notification.Notification, error) {
	if err := requireHuman(p); err != nil {
		return notification.Notification{}, err
	}
	if err := uc.d.Notifications.MarkRead(ctx, p.User.ID, id, uc.d.Clock.Now()); err != nil {
		return notification.Notification{}, plainNotFound(err)
	}
	n, err := uc.d.Notifications.ByID(ctx, p.User.ID, id)
	return n, plainNotFound(err)
}

// MarkAllNotificationsRead marks the person's unread notifications read,
// of one household or all; people only. It returns how many it marked.
type MarkAllNotificationsRead struct{ d Deps }

func NewMarkAllNotificationsRead(d Deps) *MarkAllNotificationsRead { return &MarkAllNotificationsRead{d: d} }

func (uc *MarkAllNotificationsRead) Do(ctx context.Context, p Principal, householdID domain.ID) (int, error) {
	if err := requireHuman(p); err != nil {
		return 0, err
	}
	f, err := uc.d.notificationFilter(ctx, p, householdID)
	if err != nil {
		return 0, err
	}
	return uc.d.Notifications.MarkAllRead(ctx, f, uc.d.Clock.Now())
}
