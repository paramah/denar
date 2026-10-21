package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
)

// SubscriptionView is a subscription with what follows from it today: the
// price in effect, the next term to be paid (zero when none) and the cost
// per month and year in the price's currency.
type SubscriptionView struct {
	Subscription subscription.Subscription
	VendorName   string
	CurrentPrice subscription.PricePoint
	NextDue      domain.Date
	Cost         subscription.Cost
}

func subscriptionView(s subscription.Subscription, today domain.Date) SubscriptionView {
	v := SubscriptionView{Subscription: s, VendorName: subscription.VendorName(s.Vendor), CurrentPrice: s.PriceOn(today)}
	if next, ok := s.NextCharge(today); ok {
		v.NextDue = next
	}
	if c, ok := money.LookupCurrency(v.CurrentPrice.Currency); ok {
		v.Cost, _ = subscription.CostOf(v.CurrentPrice, s.Cycle, big.NewRat(1, 1), c)
	}
	return v
}

// CycleInput is a billing cycle as clients write it; Every 0 means 1.
type CycleInput struct {
	Unit   string
	Every  int
	Anchor domain.Date
}

func (c CycleInput) cycle() subscription.Cycle {
	every := c.Every
	if every == 0 {
		every = 1
	}
	return subscription.Cycle{Unit: subscription.Unit(c.Unit), Every: every, Anchor: c.Anchor}
}

// MatchRuleInput is a match rule as clients write it; unset numbers keep
// their defaults.
type MatchRuleInput struct {
	PayeePattern       string
	AmountTolerancePct domain.Optional[int]
	DateWindowDays     domain.Optional[int]
}

func (m MatchRuleInput) apply(r subscription.MatchRule) subscription.MatchRule {
	r.PayeePattern = m.PayeePattern
	if m.AmountTolerancePct.Set {
		r.AmountTolerancePct = m.AmountTolerancePct.Value
	}
	if m.DateWindowDays.Set {
		r.DateWindowDays = m.DateWindowDays.Value
	}
	return r
}

var defaultMatchRule = subscription.MatchRule{AmountTolerancePct: subscription.DefaultTolerancePct,
	DateWindowDays: subscription.DefaultDateWindowDays}

// parsePrice reads a price in any supported currency (AD-401).
func parsePrice(field string, in MoneyInput) (money.Money, error) {
	if err := money.ValidateCurrency(field+".currency", in.Currency); err != nil {
		return money.Money{}, err
	}
	m, err := money.Parse(field+".amount", in.Amount, in.Currency)
	if err != nil {
		return money.Money{}, err
	}
	if m.Amount <= 0 {
		return money.Money{}, domain.ValidationError{Field: field + ".amount", Message: "must be greater than zero"}
	}
	return m, nil
}

func cycleText(c subscription.Cycle) string {
	return fmt.Sprintf("%d %s from %s", c.Every, c.Unit, c.Anchor)
}

func subChanges(s subscription.Subscription) audit.Changes {
	c := audit.Changes{}.Set("name", s.Name).Set("vendor", s.Vendor).Set("cycle", cycleText(s.Cycle)).Set("status", string(s.Status)).
		Set("match_rule", fmt.Sprintf("%q ±%d%% ±%dd", s.Match.PayeePattern, s.Match.AmountTolerancePct, s.Match.DateWindowDays))
	offsets := make([]string, len(s.ReminderOffsets))
	for i, o := range s.ReminderOffsets {
		offsets[i] = fmt.Sprint(o)
	}
	c.Set("reminder_offsets", strings.Join(offsets, ","))
	for field, v := range map[string]string{"external_ref": s.ExternalRef, "trial_ends_at": s.TrialEndsAt.String(),
		"ends_at": s.EndsAt.String(), "wallet_id": string(s.WalletID), "envelope_id": string(s.EnvelopeID), "note": s.Note,
		"manage_url": s.ManageURL} {
		if v != "" {
			c.Set(field, v)
		}
	}
	return c
}

func priceText(p subscription.PricePoint) string { return formatMinor(p.Amount, p.Currency) }

// checkSubscriptionLinks checks, in the transaction of the write, that the
// wallet (when set and changed) is one the principal reaches and may write
// to, and that the envelope exists (a hidden one is fine).
func (d Deps) checkSubscriptionLinks(ctx context.Context, s HouseholdScope, p Principal, sub subscription.Subscription, walletChanged, envelopeChanged bool) error {
	if walletChanged && sub.WalletID != "" {
		if _, err := d.writableWallet(ctx, s, p, sub.WalletID); err != nil {
			return err
		}
	}
	if envelopeChanged {
		return d.checkEnvelope(ctx, s.Household.ID, "envelope_id", sub.EnvelopeID)
	}
	return nil
}

// ListVendors returns the known vendors (FR-301).
type ListVendors struct{}

func NewListVendors() *ListVendors { return &ListVendors{} }

func (uc *ListVendors) Do(context.Context, Principal) []subscription.Vendor {
	return append([]subscription.Vendor{}, subscription.Vendors...)
}

type CreateSubscriptionInput struct {
	Name        string
	Vendor      string
	ExternalRef string
	// Price is in any supported currency; it takes effect from the anchor.
	Price       MoneyInput
	Cycle       CycleInput
	TrialEndsAt domain.Date
	// Status is trial when TrialEndsAt is set, else active, by default.
	Status          string
	EndsAt          domain.Date
	WalletID        string
	EnvelopeID      string
	ReminderOffsets domain.Optional[[]int] // unset: three days before
	Match           *MatchRuleInput
	Note            string
	ManageURL       string
}

// CreateSubscription records a subscription and its charges from the day
// it is recorded, less its date window (FR-300, FR-304).
type CreateSubscription struct{ d Deps }

func NewCreateSubscription(d Deps) *CreateSubscription { return &CreateSubscription{d: d} }

func (uc *CreateSubscription) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateSubscriptionInput) (SubscriptionView, error) {
	var view SubscriptionView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		now := uc.d.Clock.Now()
		today := today(s.Household, now)
		sub := subscription.Subscription{ID: uc.d.IDs.NewID(), HouseholdID: householdID, Name: in.Name, Vendor: in.Vendor,
			ExternalRef: in.ExternalRef, Cycle: in.Cycle.cycle(), TrialEndsAt: in.TrialEndsAt, Status: subscription.Status(in.Status),
			WalletID: domain.ID(strings.TrimSpace(in.WalletID)), EnvelopeID: domain.ID(strings.TrimSpace(in.EnvelopeID)),
			ReminderOffsets: subscription.DefaultReminderOffsets, Match: defaultMatchRule, Note: in.Note, ManageURL: in.ManageURL,
			CreatedBy: p.Actor(), CreatedAt: now, UpdatedAt: now}
		if sub.Status == "" {
			sub.Status = subscription.Active
			if !in.TrialEndsAt.IsZero() {
				sub.Status = subscription.Trial
			}
		}
		if sub.Status == subscription.Cancelled {
			sub.CancelledAt, sub.EndsAt = now, in.EndsAt
			if sub.EndsAt.IsZero() && sub.Cycle.Validate() == nil {
				sub.EndsAt = sub.Cycle.NextDue(today)
			}
		}
		if in.ReminderOffsets.Set {
			sub.ReminderOffsets = in.ReminderOffsets.Value
		}
		if in.Match != nil {
			sub.Match = in.Match.apply(sub.Match)
		}
		price, priceErr := parsePrice("price", in.Price)
		if priceErr == nil {
			sub.Prices = []subscription.PricePoint{{ID: uc.d.IDs.NewID(), EffectiveFrom: sub.Cycle.Anchor, Amount: price.Amount,
				Currency: price.Currency.Code, CreatedAt: now}}
		}
		sub = sub.Normalize()
		if err := errors.Join(priceErr, sub.Validate()); err != nil {
			return err
		}
		if err := uc.d.checkSubscriptionLinks(ctx, s, p, sub, true, true); err != nil {
			return err
		}
		if err := uc.d.Subscriptions.Create(ctx, sub); err != nil {
			return err
		}
		changes := subChanges(sub).Set("price", priceText(sub.Prices[0]))
		if err := uc.d.record(ctx, householdID, p.Actor(), "subscription.created", audit.ResourceSubscription, sub.ID, changes); err != nil {
			return err
		}
		start := today.AddDays(-sub.Match.DateWindowDays)
		if _, err := uc.d.syncCharges(ctx, sub, start, start, today); err != nil {
			return err
		}
		view = subscriptionView(sub, today)
		return nil
	})
	return view, err
}

type ListSubscriptionsInput struct {
	Status   string
	Vendor   string
	WalletID string
}

// ListSubscriptions returns the subscriptions the principal sees, by name.
type ListSubscriptions struct{ d Deps }

func NewListSubscriptions(d Deps) *ListSubscriptions { return &ListSubscriptions{d: d} }

func (uc *ListSubscriptions) Do(ctx context.Context, p Principal, householdID domain.ID, in ListSubscriptionsInput) ([]SubscriptionView, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return nil, err
	}
	status := subscription.Status(in.Status)
	if in.Status != "" && !status.Valid() {
		return nil, domain.ValidationError{Field: "status", Message: "must be trial, active, paused or cancelled"}
	}
	if in.WalletID != "" {
		if _, err := uc.d.visibleWallet(ctx, s, p, domain.ID(in.WalletID)); err != nil {
			return nil, err
		}
	}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return nil, err
	}
	today := today(s.Household, uc.d.Clock.Now())
	out := []SubscriptionView{}
	for _, sub := range subs {
		if (in.Status != "" && sub.Status != status) || (in.Vendor != "" && sub.Vendor != in.Vendor) ||
			(in.WalletID != "" && string(sub.WalletID) != in.WalletID) {
			continue
		}
		out = append(out, subscriptionView(sub, today))
	}
	return out, nil
}

// GetSubscription returns one subscription the principal sees.
type GetSubscription struct{ d Deps }

func NewGetSubscription(d Deps) *GetSubscription { return &GetSubscription{d: d} }

func (uc *GetSubscription) Do(ctx context.Context, p Principal, householdID, id domain.ID) (SubscriptionView, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return SubscriptionView{}, err
	}
	sub, err := uc.d.visibleSubscription(ctx, s, p, id)
	if err != nil {
		return SubscriptionView{}, err
	}
	return subscriptionView(sub, today(s.Household, uc.d.Clock.Now())), nil
}

// ListSubscriptionPrices returns the price history, oldest first.
type ListSubscriptionPrices struct{ d Deps }

func NewListSubscriptionPrices(d Deps) *ListSubscriptionPrices { return &ListSubscriptionPrices{d: d} }

func (uc *ListSubscriptionPrices) Do(ctx context.Context, p Principal, householdID, id domain.ID) ([]subscription.PricePoint, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return nil, err
	}
	sub, err := uc.d.visibleSubscription(ctx, s, p, id)
	if err != nil {
		return nil, err
	}
	return sub.Prices, nil
}

type UpdateSubscriptionInput struct {
	Name            domain.Optional[string]
	Vendor          domain.Optional[string]
	ExternalRef     domain.Optional[string]
	Cycle           domain.Optional[CycleInput]
	TrialEndsAt     domain.Optional[domain.Date] // zero clears
	Status          domain.Optional[string]
	EndsAt          domain.Optional[domain.Date]
	WalletID        domain.Optional[string] // "" clears
	EnvelopeID      domain.Optional[string] // "" clears
	ReminderOffsets domain.Optional[[]int]
	Match           domain.Optional[MatchRuleInput]
	Note            domain.Optional[string]
	ManageURL       domain.Optional[string]
}

// UpdateSubscription changes a subscription and makes its future expected
// charges follow (FR-305). Cancelling sets the time and, unless given, the
// end of access to the next term (AD-411).
type UpdateSubscription struct{ d Deps }

func NewUpdateSubscription(d Deps) *UpdateSubscription { return &UpdateSubscription{d: d} }

func (uc *UpdateSubscription) Do(ctx context.Context, p Principal, householdID, id domain.ID, in UpdateSubscriptionInput) (SubscriptionView, error) {
	var view SubscriptionView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		old, err := uc.d.visibleSubscription(ctx, s, p, id)
		if err != nil {
			return err
		}
		now := uc.d.Clock.Now()
		today := today(s.Household, now)
		sub := old
		set := func(o domain.Optional[string], dst *string) {
			if o.Set {
				*dst = o.Value
			}
		}
		set(in.Name, &sub.Name)
		set(in.Vendor, &sub.Vendor)
		set(in.ExternalRef, &sub.ExternalRef)
		set(in.Note, &sub.Note)
		set(in.ManageURL, &sub.ManageURL)
		if in.Cycle.Set {
			sub.Cycle = in.Cycle.Value.cycle()
		}
		if in.TrialEndsAt.Set {
			sub.TrialEndsAt = in.TrialEndsAt.Value
		}
		if in.WalletID.Set {
			sub.WalletID = domain.ID(strings.TrimSpace(in.WalletID.Value))
		}
		if in.EnvelopeID.Set {
			sub.EnvelopeID = domain.ID(strings.TrimSpace(in.EnvelopeID.Value))
		}
		if in.ReminderOffsets.Set {
			sub.ReminderOffsets = in.ReminderOffsets.Value
		}
		if in.Match.Set {
			sub.Match = in.Match.Value.apply(sub.Match)
		}
		if in.Status.Set {
			sub.Status = subscription.Status(in.Status.Value)
		}
		if sub.Status == subscription.Cancelled {
			if old.Status != subscription.Cancelled {
				sub.CancelledAt, sub.EndsAt = now, domain.Date{}
			}
			if in.EndsAt.Set {
				sub.EndsAt = in.EndsAt.Value
			}
			if sub.EndsAt.IsZero() && sub.Cycle.Validate() == nil {
				sub.EndsAt = sub.Cycle.NextDue(today)
			}
		} else if in.EndsAt.Set && !in.EndsAt.Value.IsZero() {
			return domain.ValidationError{Field: "ends_at", Message: "is set by cancelling the subscription"}
		}
		sub.UpdatedAt = now
		sub = sub.Normalize()
		if err := sub.Validate(); err != nil {
			return err
		}
		if err := uc.d.checkSubscriptionLinks(ctx, s, p, sub, sub.WalletID != old.WalletID, sub.EnvelopeID != old.EnvelopeID); err != nil {
			return err
		}
		if err := uc.d.Subscriptions.Update(ctx, sub); err != nil {
			return err
		}
		if changes := diffChanges(subChanges(old), subChanges(sub)); len(changes) > 0 {
			if err := uc.d.record(ctx, householdID, p.Actor(), "subscription.updated", audit.ResourceSubscription, id, changes); err != nil {
				return err
			}
		}
		if _, err := uc.d.syncCharges(ctx, sub, today, today, today); err != nil {
			return err
		}
		view = subscriptionView(sub, today)
		return nil
	})
	return view, err
}

// DeleteSubscription removes a subscription with its prices and charges;
// the transactions that paid it stay.
type DeleteSubscription struct{ d Deps }

func NewDeleteSubscription(d Deps) *DeleteSubscription { return &DeleteSubscription{d: d} }

func (uc *DeleteSubscription) Do(ctx context.Context, p Principal, householdID, id domain.ID) error {
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		old, err := uc.d.visibleSubscription(ctx, s, p, id)
		if err != nil {
			return err
		}
		if err := uc.d.Subscriptions.Delete(ctx, householdID, id); err != nil {
			return err
		}
		changes := audit.Changes{}
		for field, c := range subChanges(old) {
			changes[field] = audit.Change{From: c.To}
		}
		return uc.d.record(ctx, householdID, p.Actor(), "subscription.deleted", audit.ResourceSubscription, id, changes)
	})
}

type ChangeSubscriptionPriceInput struct {
	EffectiveFrom domain.Date // zero for today
	Price         MoneyInput
}

// ChangeSubscriptionPrice records a new price from a day on (FR-303): a
// new point of the history (the same day replaces the point), the expected
// charges from that day or today, whichever is earlier, follow, and the
// event subscription.price_changed goes to the outbox.
type ChangeSubscriptionPrice struct{ d Deps }

func NewChangeSubscriptionPrice(d Deps) *ChangeSubscriptionPrice {
	return &ChangeSubscriptionPrice{d: d}
}

func (uc *ChangeSubscriptionPrice) Do(ctx context.Context, p Principal, householdID, id domain.ID, in ChangeSubscriptionPriceInput) (SubscriptionView, error) {
	var view SubscriptionView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsWrite)
		if err != nil {
			return err
		}
		sub, err := uc.d.visibleSubscription(ctx, s, p, id)
		if err != nil {
			return err
		}
		now := uc.d.Clock.Now()
		today := today(s.Household, now)
		price, err := parsePrice("price", in.Price)
		if err != nil {
			return err
		}
		from := in.EffectiveFrom
		if from.IsZero() {
			from = today
		}
		point := subscription.PricePoint{ID: uc.d.IDs.NewID(), EffectiveFrom: from, Amount: price.Amount, Currency: price.Currency.Code,
			CreatedAt: now}
		if err := errors.Join(point.Validate("price"), validDate("effective_from", from)); err != nil {
			return err
		}
		old := sub.PriceOn(from)
		if err := uc.d.Subscriptions.PutPrice(ctx, householdID, id, point); err != nil {
			return err
		}
		if sub, err = uc.d.Subscriptions.ByID(ctx, householdID, id); err != nil {
			return err
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "subscription.price_changed", audit.ResourceSubscription, id,
			audit.Changes{"effective_from": {To: from.String()}, "price": {From: priceText(old), To: priceText(point)}}); err != nil {
			return err
		}
		syncFrom := from
		if from.After(today) {
			syncFrom = today
		}
		if _, err := uc.d.syncCharges(ctx, sub, syncFrom, today, today); err != nil {
			return err
		}
		facts := map[string]string{"price_id": string(point.ID), "effective_from": from.String(),
			"new_amount": decimal(point.Amount, point.Currency), "new_currency": point.Currency,
			"old_amount": decimal(old.Amount, old.Currency), "old_currency": old.Currency}
		if _, err := uc.d.emit(ctx, sub, event.SubscriptionPriceChanged, "price:"+string(point.ID), facts); err != nil {
			return err
		}
		view = subscriptionView(sub, today)
		return nil
	})
	return view, err
}

// validDate checks a date of the input is within 2000–2100.
func validDate(field string, d domain.Date) error {
	if y := d.Time().Year(); y < 2000 || y > 2100 {
		return domain.ValidationError{Field: field, Message: "must be between 2000 and 2100"}
	}
	return nil
}
