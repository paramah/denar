package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
)

// jobBatch bounds how many charges one run of a job looks at.
const jobBatch = 1000

// households reads households once per job run.
type householdCache struct {
	d    Deps
	byID map[domain.ID]*household.Household
}

func (d Deps) householdCache() *householdCache {
	return &householdCache{d: d, byID: map[domain.ID]*household.Household{}}
}

// get returns the household, or nil when it is gone.
func (c *householdCache) get(ctx context.Context, id domain.ID) (*household.Household, error) {
	if h, ok := c.byID[id]; ok {
		return h, nil
	}
	h, err := c.d.Households.ByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		c.byID[id] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.byID[id] = &h
	return &h, nil
}

// GenerateCharges is the job that keeps every subscription's expected
// charges up to the horizon (FR-304). It returns how many it created; a
// second run creates none (NFR-031).
type GenerateCharges struct{ d Deps }

func NewGenerateCharges(d Deps) *GenerateCharges { return &GenerateCharges{d: d} }

func (uc *GenerateCharges) Do(ctx context.Context) (int, error) {
	subs, err := uc.d.Subscriptions.List(ctx, "")
	if err != nil {
		return 0, err
	}
	hs := uc.d.householdCache()
	created := 0
	for _, sub := range subs {
		h, err := hs.get(ctx, sub.HouseholdID)
		if err != nil {
			return created, err
		}
		if h == nil {
			continue
		}
		today := today(*h, uc.d.Clock.Now())
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			fresh, err := uc.d.Subscriptions.ByID(ctx, sub.HouseholdID, sub.ID)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			n, err := uc.d.syncCharges(ctx, fresh, today, today, today)
			created += n
			return err
		})
		if err != nil {
			return created, err
		}
	}
	return created, nil
}

// MarkMissedCharges is the job that marks expected charges not paid within
// their window as missed (FR-306), with an audit entry and the event
// subscription.charge_missed. It returns how many it marked.
type MarkMissedCharges struct{ d Deps }

func NewMarkMissedCharges(d Deps) *MarkMissedCharges { return &MarkMissedCharges{d: d} }

func (uc *MarkMissedCharges) Do(ctx context.Context) (int, error) {
	// Every household's today is at most a day after UTC's.
	until := domain.DateOf(uc.d.Clock.Now(), time.UTC).AddDays(1)
	open, err := uc.d.Charges.ListOpen(ctx, "", domain.Date{}, until, jobBatch)
	if err != nil {
		return 0, err
	}
	hs := uc.d.householdCache()
	marked := 0
	for _, c := range open {
		if c.Status != subscription.Expected {
			continue
		}
		h, err := hs.get(ctx, c.HouseholdID)
		if err != nil {
			return marked, err
		}
		if h == nil {
			continue
		}
		today := today(*h, uc.d.Clock.Now())
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			cur, err := uc.d.Charges.ByID(ctx, c.HouseholdID, c.ID)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			sub, err := uc.d.Subscriptions.ByID(ctx, c.HouseholdID, c.SubscriptionID)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			if cur.Status != subscription.Expected || today.Before(cur.MissedOn(sub.Match.DateWindowDays)) {
				return nil
			}
			cur, err = uc.d.setChargeStatus(ctx, subscriptionsActor, cur, subscription.Missed, "", "charge.missed", "", nil)
			if err != nil {
				return err
			}
			if _, err := uc.d.emit(ctx, sub, event.SubscriptionChargeMissed, "missed:"+string(cur.ID), chargeFacts(cur)); err != nil {
				return err
			}
			marked++
			return nil
		})
		if err != nil {
			return marked, err
		}
	}
	return marked, nil
}

// SendSubscriptionReminders is the job that puts reminders in the outbox
// (FR-315): subscription.renewal_upcoming for expected charges of active
// subscriptions, subscription.trial_ending for trials, once per term and
// offset, for the smallest offset whose day has come (AD-407). It returns
// how many events it stored.
type SendSubscriptionReminders struct{ d Deps }

func NewSendSubscriptionReminders(d Deps) *SendSubscriptionReminders {
	return &SendSubscriptionReminders{d: d}
}

func (uc *SendSubscriptionReminders) Do(ctx context.Context) (int, error) {
	subs, err := uc.d.Subscriptions.List(ctx, "")
	if err != nil {
		return 0, err
	}
	hs := uc.d.householdCache()
	stored := 0
	for _, sub := range subs {
		if sub.Status != subscription.Active && sub.Status != subscription.Trial {
			continue
		}
		h, err := hs.get(ctx, sub.HouseholdID)
		if err != nil {
			return stored, err
		}
		if h == nil {
			continue
		}
		today := today(*h, uc.d.Clock.Now())
		err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			n, err := uc.d.remind(ctx, sub, today)
			stored += n
			return err
		})
		if err != nil {
			return stored, err
		}
	}
	return stored, nil
}

func (d Deps) remind(ctx context.Context, sub subscription.Subscription, today domain.Date) (int, error) {
	stored := 0
	if sub.Status == subscription.Trial && !sub.TrialEndsAt.Before(today) {
		if offset, ok := sub.ReminderDue(sub.TrialEndsAt, today); ok {
			p := sub.PriceOn(sub.TrialEndsAt)
			facts := map[string]string{"trial_ends_at": sub.TrialEndsAt.String(), "days_before": strconv.Itoa(offset),
				"amount": decimal(p.Amount, p.Currency), "currency": p.Currency}
			key := "trial:" + string(sub.ID) + ":" + sub.TrialEndsAt.String() + ":" + strconv.Itoa(offset)
			ok, err := d.emit(ctx, sub, event.SubscriptionTrialEnding, key, facts)
			if err != nil {
				return stored, err
			}
			if ok {
				stored++
			}
		}
	}
	if sub.Status != subscription.Active {
		return stored, nil
	}
	charges, err := d.Charges.ListBySubscription(ctx, sub.HouseholdID, sub.ID, today)
	if err != nil {
		return stored, err
	}
	for _, c := range charges {
		if c.Status != subscription.Expected || c.DueDate.After(today.AddDays(subscription.MaxReminderOffset)) {
			continue
		}
		offset, ok := sub.ReminderDue(c.DueDate, today)
		if !ok {
			continue
		}
		facts := chargeFacts(c)
		facts["days_before"] = strconv.Itoa(offset)
		ok, err := d.emit(ctx, sub, event.SubscriptionRenewalUpcoming, "renewal:"+string(c.ID)+":"+strconv.Itoa(offset), facts)
		if err != nil {
			return stored, err
		}
		if ok {
			stored++
		}
	}
	return stored, nil
}

// SubscriptionJobResult counts what one run of the subscription jobs did.
type SubscriptionJobResult struct {
	Created   int // charges
	Missed    int // charges
	Reminders int // events
}

// RunSubscriptionJobs runs the subscription jobs in order: charges first,
// so that missed charges and reminders see them.
type RunSubscriptionJobs struct {
	generate *GenerateCharges
	missed   *MarkMissedCharges
	remind   *SendSubscriptionReminders
}

func NewRunSubscriptionJobs(d Deps) *RunSubscriptionJobs {
	return &RunSubscriptionJobs{generate: NewGenerateCharges(d), missed: NewMarkMissedCharges(d), remind: NewSendSubscriptionReminders(d)}
}

func (uc *RunSubscriptionJobs) Do(ctx context.Context) (SubscriptionJobResult, error) {
	var r SubscriptionJobResult
	var err error
	if r.Created, err = uc.generate.Do(ctx); err != nil {
		return r, err
	}
	if r.Missed, err = uc.missed.Do(ctx); err != nil {
		return r, err
	}
	r.Reminders, err = uc.remind.Do(ctx)
	return r, err
}

// unlinkCharge opens again the charge a transaction about to be deleted
// pays (AD-414); call it in the transaction of the deletion.
func (d Deps) unlinkCharge(ctx context.Context, h household.Household, actor domain.Actor, txID domain.ID) error {
	if d.Charges == nil {
		return nil
	}
	c, err := d.Charges.ByTransaction(ctx, h.ID, txID)
	if found, err := ignoreNotFound(err); err != nil || !found {
		return err
	}
	sub, err := d.Subscriptions.ByID(ctx, h.ID, c.SubscriptionID)
	if err != nil {
		return err
	}
	status := c.OpenStatus(sub.Match.DateWindowDays, today(h, d.Clock.Now()))
	_, err = d.setChargeStatus(ctx, actor, c, status, "", "charge.unmatched", "transaction_deleted", audit.Changes{})
	return err
}
