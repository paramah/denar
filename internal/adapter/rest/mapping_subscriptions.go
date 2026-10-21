package rest

import (
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"git.eiac.dev/tools/denar/internal/adapter/rest/api"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func nullableDate(d domain.Date) nullable.Nullable[openapi_types.Date] {
	if d.IsZero() {
		return nullable.NewNullNullable[openapi_types.Date]()
	}
	return nullable.NewNullableWithValue(apiDate(d))
}

// optionalNullableDate reads a date that may be absent, null (the zero
// date) or a value.
func optionalNullableDate(n nullable.Nullable[openapi_types.Date]) domain.Optional[domain.Date] {
	if !n.IsSpecified() {
		return domain.Optional[domain.Date]{}
	}
	if n.IsNull() {
		return domain.Some(domain.Date{})
	}
	d := n.MustGet()
	return domain.Some(fromAPIDate(&d))
}

func apiPrice(p subscription.PricePoint) api.Money { return minor(p.Amount, p.Currency) }

func apiSubscription(v usecase.SubscriptionView) api.Subscription {
	s := v.Subscription
	offsets := s.ReminderOffsets
	if offsets == nil {
		offsets = []int{}
	}
	c := v.CurrentPrice.Currency
	return api.Subscription{
		Id: string(s.ID), Name: s.Name, Vendor: s.Vendor, VendorName: v.VendorName, ExternalRef: nullableString(s.ExternalRef),
		Price: apiPrice(v.CurrentPrice), PriceEffectiveFrom: apiDate(v.CurrentPrice.EffectiveFrom),
		Cycle:       api.Cycle{Unit: api.CycleUnit(s.Cycle.Unit), Every: s.Cycle.Every, Anchor: apiDate(s.Cycle.Anchor)},
		TrialEndsAt: nullableDate(s.TrialEndsAt), Status: api.SubscriptionStatus(s.Status), CancelledAt: nullableTime(s.CancelledAt),
		EndsAt: nullableDate(s.EndsAt), WalletId: nullableID(s.WalletID), EnvelopeId: nullableID(s.EnvelopeID), ReminderOffsets: offsets,
		MatchRule: api.MatchRule{PayeePattern: s.Match.PayeePattern, AmountTolerancePct: s.Match.AmountTolerancePct,
			DateWindowDays: s.Match.DateWindowDays},
		Note: s.Note, ManageUrl: nullableString(s.ManageURL), NextDue: nullableDate(v.NextDue),
		Monthly: minor(v.Cost.Monthly, c), Yearly: minor(v.Cost.Yearly, c),
		CreatedBy: api.Actor{Kind: api.ActorKind(s.CreatedBy.Kind), Id: string(s.CreatedBy.ID)}, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func apiPricePoint(p subscription.PricePoint) api.PricePoint {
	return api.PricePoint{Id: string(p.ID), EffectiveFrom: apiDate(p.EffectiveFrom), Price: apiPrice(p), CreatedAt: p.CreatedAt}
}

func apiCharge(v usecase.ChargeView) api.Charge {
	c := v.Charge
	return api.Charge{Id: string(c.ID), SubscriptionId: string(c.SubscriptionID), SubscriptionName: v.SubscriptionName, Vendor: v.Vendor,
		WalletId: nullableID(v.WalletID), DueDate: apiDate(c.DueDate), Amount: minor(c.Amount, c.Currency),
		Status: api.ChargeStatus(c.Status), TransactionId: nullableID(c.TransactionID), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func apiUpcoming(u usecase.UpcomingRenewal, base string) api.UpcomingRenewal {
	out := api.UpcomingRenewal{Charge: apiCharge(u.Charge), BaseAmount: nullable.NewNullNullable[api.Money]()}
	if u.BaseKnown {
		out.BaseAmount.Set(minor(u.BaseAmount, base))
	}
	return out
}

func cycleInput(c api.CycleInput) usecase.CycleInput {
	return usecase.CycleInput{Unit: c.Unit, Every: deref(c.Every), Anchor: fromAPIDate(&c.Anchor)}
}

func matchRuleInput(m *api.MatchRuleInput) usecase.MatchRuleInput {
	return usecase.MatchRuleInput{PayeePattern: deref(m.PayeePattern), AmountTolerancePct: optional(m.AmountTolerancePct),
		DateWindowDays: optional(m.DateWindowDays)}
}

func apiSummary(s usecase.SubscriptionSummary) api.SubscriptionSummary {
	b := s.Currency
	m := func(amount int64) api.Money { return minor(amount, b) }
	out := api.SubscriptionSummary{Currency: b, Monthly: m(s.Total.Monthly), Yearly: m(s.Total.Yearly), Incomplete: s.Incomplete,
		MissingCurrencies: s.MissingCurrencies,
		Counts: api.SubscriptionCounts{Trial: s.Counts[subscription.Trial], Active: s.Counts[subscription.Active],
			Paused: s.Counts[subscription.Paused], Cancelled: s.Counts[subscription.Cancelled]},
		ByVendor: []api.VendorCost{}, ByEnvelope: []api.EnvelopeCost{}, Upcoming: []api.UpcomingRenewal{},
		TrialsEnding: []api.TrialEnding{}, PriceChanges: []api.PriceChangeEntry{}}
	if out.MissingCurrencies == nil {
		out.MissingCurrencies = []string{}
	}
	for _, g := range s.ByVendor {
		out.ByVendor = append(out.ByVendor, api.VendorCost{Vendor: g.Key, VendorName: subscription.VendorName(g.Key), Count: g.Count,
			Monthly: m(g.Cost.Monthly), Yearly: m(g.Cost.Yearly)})
	}
	for _, g := range s.ByEnvelope {
		out.ByEnvelope = append(out.ByEnvelope, api.EnvelopeCost{EnvelopeId: nullableString(g.Key), Count: g.Count,
			Monthly: m(g.Cost.Monthly), Yearly: m(g.Cost.Yearly)})
	}
	for _, u := range s.Upcoming {
		out.Upcoming = append(out.Upcoming, apiUpcoming(u, b))
	}
	for _, t := range s.TrialsEnding {
		out.TrialsEnding = append(out.TrialsEnding, api.TrialEnding{SubscriptionId: string(t.Subscription.ID), Name: t.Subscription.Name,
			Vendor: t.Subscription.Vendor, TrialEndsAt: apiDate(t.EndsAt), Price: apiPrice(t.Price)})
	}
	for _, c := range s.PriceChanges {
		out.PriceChanges = append(out.PriceChanges, api.PriceChangeEntry{SubscriptionId: string(c.Subscription.ID), Name: c.Subscription.Name,
			EffectiveFrom: apiDate(c.Price.EffectiveFrom), OldPrice: apiPrice(c.Previous), NewPrice: apiPrice(c.Price)})
	}
	return out
}
