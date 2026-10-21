package usecase

import (
	"context"
	"errors"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/domain/user"
)

// deliveryBatch bounds how many deliveries one run sends.
const deliveryBatch = 50

// DeliveryResult counts what one run of the delivery worker did.
type DeliveryResult struct {
	Sent    int
	Retried int // failed, to be tried again
	Failed  int // failed for good
}

// DeliverNotifications sends due deliveries over their channels, outside
// any database transaction, and stores the outcome (FR-520…FR-523). One
// worker runs per process.
type DeliverNotifications struct{ d Deps }

func NewDeliverNotifications(d Deps) *DeliverNotifications { return &DeliverNotifications{d: d} }

func (uc *DeliverNotifications) Do(ctx context.Context) (DeliveryResult, error) {
	var r DeliveryResult
	due, err := uc.d.Deliveries.ListDue(ctx, uc.d.Clock.Now(), deliveryBatch)
	if err != nil {
		return r, err
	}
	hs := uc.d.householdCache()
	locales := map[domain.ID]user.Locale{}
	for _, dl := range due {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		c, err := uc.d.Channels.ByID(ctx, dl.UserID, dl.ChannelID)
		if found, err := ignoreNotFound(err); err != nil {
			return r, err
		} else if !found {
			continue // the channel and its deliveries are gone
		}
		h, err := hs.get(ctx, dl.HouseholdID)
		if err != nil {
			return r, err
		}
		locale, ok := locales[dl.UserID]
		if !ok {
			if locale, err = uc.d.localeOf(ctx, dl.UserID); err != nil {
				return r, err
			}
			locales[dl.UserID] = locale
		}
		var sendErr error
		if !c.Enabled {
			sendErr = errPermanent{errChannelDisabled}
		} else {
			msg := outgoing(dl, h)
			msg.Locale = locale
			sendErr = uc.d.send(ctx, c, msg)
		}
		now := uc.d.Clock.Now()
		var permanent errPermanent
		switch {
		case sendErr == nil:
			dl = dl.Succeeded(now)
			r.Sent++
		case errors.As(sendErr, &permanent):
			dl = dl.Abandon(now, errorText(sendErr))
			r.Failed++
		default:
			dl = dl.Failure(now, errorText(sendErr))
			if dl.Status == notification.Failed {
				r.Failed++
			} else {
				r.Retried++
			}
		}
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := uc.d.Deliveries.Update(ctx, dl); err != nil {
				_, err = ignoreNotFound(err)
				return err
			}
			if dl.Status == notification.Pending {
				return nil
			}
			cur, err := uc.d.Channels.ByID(ctx, dl.UserID, dl.ChannelID)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			recordOutcome(&cur, sendErr, now)
			return uc.d.Channels.Update(ctx, cur)
		})
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

// errChannelDisabled is stable English; the Web UI translates it (013).
var errChannelDisabled = errors.New("channel is disabled")

// localeOf is the language of a person; one who is gone gets the default.
func (d Deps) localeOf(ctx context.Context, id domain.ID) (user.Locale, error) {
	u, err := d.Users.ByID(ctx, id)
	if found, err := ignoreNotFound(err); err != nil {
		return "", err
	} else if !found {
		return user.DefaultLocale, nil
	}
	return u.EffectiveLocale(), nil
}

func outgoing(dl notification.Delivery, h *household.Household) OutgoingMessage {
	msg := OutgoingMessage{ID: string(dl.ID), Type: string(dl.Type), Title: dl.Title, Body: dl.Body, URL: dl.URL,
		HouseholdID: dl.HouseholdID, OccurredAt: dl.OccurredAt, Data: dl.Data}
	if h != nil {
		msg.HouseholdName = h.Name
	}
	return msg
}
