package usecase

import (
	"context"
	"strconv"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
)

// subscriptionsActor makes the changes of the subscription jobs.
var subscriptionsActor = domain.Actor{Kind: domain.ActorSystem, ID: "subscriptions"}

// emit appends a subscription event to the outbox; call it in the
// transaction of the change (FR-314). Without an outbox nothing is kept.
func (d Deps) emit(ctx context.Context, s subscription.Subscription, typ event.Type, key string, facts map[string]string) (bool, error) {
	if d.Events == nil {
		return false, nil
	}
	payload := map[string]string{
		"subscription_id": string(s.ID), "subscription_name": s.Name, "vendor": s.Vendor,
		"vendor_name": subscription.VendorName(s.Vendor), "wallet_id": string(s.WalletID), "envelope_id": string(s.EnvelopeID),
	}
	for k, v := range facts {
		payload[k] = v
	}
	return d.Events.Append(ctx, event.Event{ID: d.IDs.NewID(), HouseholdID: s.HouseholdID, Type: typ,
		ResourceType: event.ResourceSubscription, ResourceID: s.ID, DedupeKey: key, Payload: payload, OccurredAt: d.Clock.Now()})
}

// decimal writes minor units of the currency as a decimal, such as "20.00".
func decimal(amount int64, code string) string {
	c, ok := money.LookupCurrency(code)
	if !ok {
		return strconv.FormatInt(amount, 10)
	}
	return money.Money{Amount: amount, Currency: c}.String()
}

func chargeFacts(c subscription.Charge) map[string]string {
	return map[string]string{"charge_id": string(c.ID), "due_date": c.DueDate.String(), "amount": decimal(c.Amount, c.Currency),
		"currency": c.Currency}
}
