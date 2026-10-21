package usecase

import (
	"context"
	"maps"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/domain/user"
)

// dispatchBatch bounds how many events one run dispatches.
const dispatchBatch = 100

// recipients returns the people an event of the household is for: active
// members who see the event's wallet; every member without a wallet,
// nobody for a wallet that is not there (FR-511). Bots are not members.
func (d Deps) recipients(ctx context.Context, e event.Event) ([]user.User, error) {
	members, err := d.Memberships.ListByHousehold(ctx, e.HouseholdID)
	if err != nil {
		return nil, err
	}
	ids := make([]domain.ID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	users, err := d.Users.ByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	users = slices.DeleteFunc(users, func(u user.User) bool { return !u.Active })
	walletID := domain.ID(e.Payload["wallet_id"])
	if walletID == "" {
		return users, nil
	}
	w, err := d.Wallets.ByID(ctx, e.HouseholdID, walletID)
	if found, err := ignoreNotFound(err); err != nil {
		return nil, err
	} else if !found {
		return nil, nil
	}
	return slices.DeleteFunc(users, func(u user.User) bool { return !w.VisibleTo(u.ID) }), nil
}

// DispatchResult counts what one dispatch run did.
type DispatchResult struct {
	Events        int
	Notifications int
	Deliveries    int
}

// DispatchEvents turns pending events into notifications and deliveries,
// each event in one transaction with marking it dispatched (FR-510).
type DispatchEvents struct{ d Deps }

func NewDispatchEvents(d Deps) *DispatchEvents { return &DispatchEvents{d: d} }

func (uc *DispatchEvents) Do(ctx context.Context) (DispatchResult, error) {
	var r DispatchResult
	pending, err := uc.d.Events.ListPending(ctx, dispatchBatch)
	if err != nil {
		return r, err
	}
	hs := uc.d.householdCache()
	for _, e := range pending {
		h, err := hs.get(ctx, e.HouseholdID)
		if err != nil {
			return r, err
		}
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			if h != nil {
				n, dl, err := uc.d.dispatch(ctx, *h, e)
				if err != nil {
					return err
				}
				r.Notifications += n
				r.Deliveries += dl
			}
			return uc.d.Events.MarkDispatched(ctx, e.ID, uc.d.Clock.Now())
		})
		if err != nil {
			return r, err
		}
		r.Events++
	}
	return r, nil
}

func (d Deps) dispatch(ctx context.Context, h household.Household, e event.Event) (notifications, deliveries int, err error) {
	people, err := d.recipients(ctx, e)
	if err != nil {
		return 0, 0, err
	}
	now := d.Clock.Now()
	for _, u := range people {
		// Each recipient reads the notification in their own language
		// (FR-1307); the stored text keeps the language of this moment.
		msg := notification.Render(u.EffectiveLocale(), e)
		prefs, err := d.Preferences.List(ctx, u.ID, h.ID)
		if err != nil {
			return notifications, deliveries, err
		}
		pref := notification.For(prefs, e.Type)
		if pref.InApp {
			stored, err := d.Notifications.Create(ctx, notification.Notification{ID: d.IDs.NewID(), UserID: u.ID, HouseholdID: h.ID,
				EventID: e.ID, Type: e.Type, Title: msg.Title, Body: msg.Body, Link: msg.Link, WalletID: domain.ID(e.Payload["wallet_id"]),
				ResourceType: e.ResourceType, ResourceID: e.ResourceID, CreatedAt: now})
			if err != nil {
				return notifications, deliveries, err
			}
			if stored {
				notifications++
			}
		}
		if len(pref.ChannelIDs) == 0 {
			continue
		}
		channels, err := d.Channels.ListByUser(ctx, u.ID)
		if err != nil {
			return notifications, deliveries, err
		}
		for _, c := range channels {
			if !c.Enabled || !slices.Contains(pref.ChannelIDs, c.ID) {
				continue
			}
			stored, err := d.Deliveries.Create(ctx, notification.Delivery{ID: d.IDs.NewID(), EventID: e.ID, UserID: u.ID,
				HouseholdID: h.ID, ChannelID: c.ID, Type: e.Type, Title: msg.Title, Body: msg.Body, URL: d.BaseURL + msg.Link,
				Data: maps.Clone(e.Payload), OccurredAt: e.OccurredAt, Status: notification.Pending, NextAttemptAt: now,
				CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return notifications, deliveries, err
			}
			if stored {
				deliveries++
			}
		}
	}
	return notifications, deliveries, nil
}
