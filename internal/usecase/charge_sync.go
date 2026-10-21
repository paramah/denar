package usecase

import (
	"context"
	"errors"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
)

// DefaultChargeHorizonDays is how far ahead charges are expected when the
// configuration says nothing.
const DefaultChargeHorizonDays = 90

func (d Deps) chargeHorizon() int {
	if d.ChargeHorizonDays <= 0 {
		return DefaultChargeHorizonDays
	}
	return d.ChargeHorizonDays
}

// syncCharges makes the expected charges of the subscription due from on
// follow its plan up to the horizon (FR-304, FR-305): expected charges off
// the plan go, those with another amount are corrected, and terms of the
// plan without a charge get one when due on or after createFrom. Charges
// in other statuses are left alone, so their terms never come back. Call
// it in a transaction; it returns how many charges it created.
func (d Deps) syncCharges(ctx context.Context, s subscription.Subscription, from, createFrom, today domain.Date) (int, error) {
	if d.Charges == nil {
		return 0, nil
	}
	plan := s.Plan(from, today.AddDays(d.chargeHorizon()))
	planned := map[domain.Date]subscription.Planned{}
	for _, p := range plan {
		planned[p.DueDate] = p
	}
	existing, err := d.Charges.ListBySubscription(ctx, s.HouseholdID, s.ID, from)
	if err != nil {
		return 0, err
	}
	have := map[domain.Date]bool{}
	now := d.Clock.Now()
	for _, c := range existing {
		have[c.DueDate] = true
		if c.Status != subscription.Expected {
			continue
		}
		p, ok := planned[c.DueDate]
		switch {
		case !ok:
			if err := d.Charges.Delete(ctx, s.HouseholdID, c.ID); err != nil {
				return 0, err
			}
		case p.Amount != c.Amount || p.Currency != c.Currency:
			c.Amount, c.Currency, c.UpdatedAt = p.Amount, p.Currency, now
			if err := d.Charges.Update(ctx, c); err != nil {
				return 0, err
			}
		}
	}
	created := 0
	for _, p := range plan {
		if have[p.DueDate] || p.DueDate.Before(createFrom) {
			continue
		}
		err := d.Charges.Create(ctx, subscription.Charge{ID: d.IDs.NewID(), HouseholdID: s.HouseholdID, SubscriptionID: s.ID,
			DueDate: p.DueDate, Amount: p.Amount, Currency: p.Currency, Status: subscription.Expected, CreatedAt: now, UpdatedAt: now})
		var ce domain.ConflictError
		if errors.As(err, &ce) && ce.Reason == domain.ConflictDuplicateCharge {
			continue
		}
		if err != nil {
			return 0, err
		}
		created++
	}
	return created, nil
}
