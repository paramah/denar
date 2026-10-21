package usecase

import (
	"cmp"
	"context"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
)

// Windows of the subscription summary, in days.
const (
	summaryUpcomingDays     = 30
	summaryTrialsDays       = 14
	summaryPriceChangesDays = 90
	maxUpcoming             = 1000
)

// CostGroup is the cost of a set of subscriptions in the base currency.
type CostGroup struct {
	Key   string // the vendor key, or the envelope id ("" without one)
	Count int
	Cost  subscription.Cost
}

// UpcomingRenewal is an expected charge due soon, with an estimate in the
// base currency at today's rate (BaseKnown false without one).
type UpcomingRenewal struct {
	Charge     ChargeView
	BaseAmount int64
	BaseKnown  bool
}

// TrialEnding is a trial ending soon and what it will cost after.
type TrialEnding struct {
	Subscription subscription.Subscription
	EndsAt       domain.Date
	Price        subscription.PricePoint
}

// PriceChange is a price point other than a subscription's first, with
// the price before it.
type PriceChange struct {
	Subscription subscription.Subscription
	Price        subscription.PricePoint
	Previous     subscription.PricePoint
}

// SubscriptionSummary is what the subscriptions a principal sees cost and
// what comes next (FR-312). Totals count active and trial subscriptions at
// today's price and rate; one whose rate is unknown is left out and makes
// the summary incomplete.
type SubscriptionSummary struct {
	Currency          string
	Total             subscription.Cost
	Incomplete        bool
	MissingCurrencies []string
	Counts            map[subscription.Status]int
	ByVendor          []CostGroup
	ByEnvelope        []CostGroup
	Upcoming          []UpcomingRenewal
	TrialsEnding      []TrialEnding
	PriceChanges      []PriceChange
}

// baseRates converts at today's rates, each currency looked up once.
type baseRates struct {
	cv    *converter
	today domain.Date
	base  money.Currency
}

func (b baseRates) cost(ctx context.Context, s subscription.Subscription) (subscription.Cost, bool, error) {
	p := s.PriceOn(b.today)
	rate, err := b.cv.rate(ctx, p.Currency, b.today)
	if err != nil || rate == nil {
		return subscription.Cost{}, false, err
	}
	c, err := subscription.CostOf(p, s.Cycle, rate, b.base)
	if err != nil {
		return subscription.Cost{}, false, nil
	}
	return c, true, nil
}

func (b baseRates) amount(ctx context.Context, amount int64, currency string) (int64, bool, error) {
	return b.cv.convert(ctx, amount, currency, b.base.Code, b.today)
}

func addCost(a, b subscription.Cost) (subscription.Cost, error) {
	m, ok1 := money.AddInt64(a.Monthly, b.Monthly)
	y, ok2 := money.AddInt64(a.Yearly, b.Yearly)
	if !ok1 || !ok2 {
		return subscription.Cost{}, money.ErrOverflow
	}
	return subscription.Cost{Monthly: m, Yearly: y}, nil
}

func (d Deps) upcoming(ctx context.Context, s HouseholdScope, subs []subscription.Subscription, rates baseRates, days int) ([]UpcomingRenewal, error) {
	byID := map[domain.ID]subscription.Subscription{}
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	open, err := d.Charges.ListOpen(ctx, s.Household.ID, rates.today, rates.today.AddDays(days), maxUpcoming)
	if err != nil {
		return nil, err
	}
	out := []UpcomingRenewal{}
	for _, c := range open {
		sub, ok := byID[c.SubscriptionID]
		if !ok || c.Status != subscription.Expected {
			continue
		}
		base, known, err := rates.amount(ctx, c.Amount, c.Currency)
		if err != nil {
			return nil, err
		}
		out = append(out, UpcomingRenewal{Charge: chargeView(c, sub), BaseAmount: base, BaseKnown: known})
	}
	return out, nil
}

func (d Deps) baseRates(s HouseholdScope) baseRates {
	base, _ := money.LookupCurrency(s.Household.BaseCurrency)
	return baseRates{cv: d.converter(s.Household), today: today(s.Household, d.Clock.Now()), base: base}
}

// GetSubscriptionSummary returns the summary of the subscriptions the
// principal sees.
type GetSubscriptionSummary struct{ d Deps }

func NewGetSubscriptionSummary(d Deps) *GetSubscriptionSummary { return &GetSubscriptionSummary{d: d} }

func (uc *GetSubscriptionSummary) Do(ctx context.Context, p Principal, householdID domain.ID) (SubscriptionSummary, error) {
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	rates := uc.d.baseRates(s)
	today := rates.today
	out := SubscriptionSummary{Currency: s.Household.BaseCurrency, Counts: map[subscription.Status]int{}, MissingCurrencies: []string{},
		TrialsEnding: []TrialEnding{}, PriceChanges: []PriceChange{}}
	for _, st := range subscription.Statuses {
		out.Counts[st] = 0
	}
	vendors, envelopes := map[string]*CostGroup{}, map[string]*CostGroup{}
	group := func(m map[string]*CostGroup, key string, c subscription.Cost) error {
		g, ok := m[key]
		if !ok {
			g = &CostGroup{Key: key}
			m[key] = g
		}
		g.Count++
		var err error
		g.Cost, err = addCost(g.Cost, c)
		return err
	}
	for _, sub := range subs {
		out.Counts[sub.Status]++
		for _, pp := range sub.Prices[1:] {
			if !pp.EffectiveFrom.Before(today.AddDays(-summaryPriceChangesDays)) {
				prev, _ := sub.PriceBefore(pp)
				out.PriceChanges = append(out.PriceChanges, PriceChange{Subscription: sub, Price: pp, Previous: prev})
			}
		}
		if sub.Status == subscription.Trial && !sub.TrialEndsAt.Before(today) && !sub.TrialEndsAt.After(today.AddDays(summaryTrialsDays)) {
			out.TrialsEnding = append(out.TrialsEnding, TrialEnding{Subscription: sub, EndsAt: sub.TrialEndsAt, Price: sub.PriceOn(sub.TrialEndsAt)})
		}
		if sub.Status != subscription.Active && sub.Status != subscription.Trial {
			continue
		}
		cost, ok, err := rates.cost(ctx, sub)
		if err != nil {
			return SubscriptionSummary{}, err
		}
		if !ok {
			out.Incomplete = true
			if cur := sub.PriceOn(today).Currency; !slices.Contains(out.MissingCurrencies, cur) {
				out.MissingCurrencies = append(out.MissingCurrencies, cur)
			}
			continue
		}
		if out.Total, err = addCost(out.Total, cost); err != nil {
			return SubscriptionSummary{}, err
		}
		if err := group(vendors, sub.Vendor, cost); err != nil {
			return SubscriptionSummary{}, err
		}
		if err := group(envelopes, string(sub.EnvelopeID), cost); err != nil {
			return SubscriptionSummary{}, err
		}
	}
	slices.Sort(out.MissingCurrencies)
	out.ByVendor, out.ByEnvelope = costGroups(vendors), costGroups(envelopes)
	slices.SortFunc(out.TrialsEnding, func(a, b TrialEnding) int { return a.EndsAt.Compare(b.EndsAt) })
	slices.SortFunc(out.PriceChanges, func(a, b PriceChange) int {
		return cmp.Or(b.Price.EffectiveFrom.Compare(a.Price.EffectiveFrom), cmp.Compare(a.Subscription.Name, b.Subscription.Name))
	})
	if out.Upcoming, err = uc.d.upcoming(ctx, s, subs, rates, summaryUpcomingDays); err != nil {
		return SubscriptionSummary{}, err
	}
	return out, nil
}

// costGroups orders groups by monthly cost, the dearest first, then key.
func costGroups(m map[string]*CostGroup) []CostGroup {
	out := make([]CostGroup, 0, len(m))
	for _, g := range m {
		out = append(out, *g)
	}
	slices.SortFunc(out, func(a, b CostGroup) int {
		return cmp.Or(cmp.Compare(b.Cost.Monthly, a.Cost.Monthly), cmp.Compare(a.Key, b.Key))
	})
	return out
}

// ListUpcomingRenewals returns the expected charges of the subscriptions
// the principal sees due within the next days (FR-313), 7 by default.
type ListUpcomingRenewals struct{ d Deps }

func NewListUpcomingRenewals(d Deps) *ListUpcomingRenewals { return &ListUpcomingRenewals{d: d} }

func (uc *ListUpcomingRenewals) Do(ctx context.Context, p Principal, householdID domain.ID, days domain.Optional[int]) ([]UpcomingRenewal, error) {
	n := 7
	if days.Set {
		if days.Value < 1 || days.Value > 365 {
			return nil, domain.ValidationError{Field: "days", Message: "must be between 1 and 365"}
		}
		n = days.Value
	}
	s, err := uc.d.subscriptionScope(ctx, p, householdID, bot.ScopeSubscriptionsRead)
	if err != nil {
		return nil, err
	}
	subs, err := uc.d.visibleSubscriptions(ctx, s, p)
	if err != nil {
		return nil, err
	}
	return uc.d.upcoming(ctx, s, subs, uc.d.baseRates(s), n)
}
