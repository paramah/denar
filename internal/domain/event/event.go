// Package event models domain events: facts other parts of the system
// react to, such as a renewal coming up. Use cases append them to an
// outbox in the transaction of the change; the notifications of stage 005
// dispatch them.
package event

import (
	"time"

	"github.com/paramah/denar/internal/domain"
)

// Type names a kind of event.
type Type string

// Events of subscriptions (stage 004). Their payloads are described in
// .ai/sdd/specs/004-subscriptions/design.md §3.
const (
	// SubscriptionRenewalUpcoming: a term is due in days_before days.
	SubscriptionRenewalUpcoming Type = "subscription.renewal_upcoming"
	// SubscriptionTrialEnding: the trial ends in days_before days.
	SubscriptionTrialEnding Type = "subscription.trial_ending"
	// SubscriptionPriceChanged: a new price takes effect.
	SubscriptionPriceChanged Type = "subscription.price_changed"
	// SubscriptionChargeMissed: a term was not seen paid within its window.
	SubscriptionChargeMissed Type = "subscription.charge_missed"
)

// Events of the envelope budget (stage 005). Payload: envelope_id,
// envelope_name, month (YYYY-MM), available, currency, wallet_id (""); a
// goal adds target_date, goal_amount, missing and days_left.
const (
	// EnvelopeOverspent: the envelope is below zero in the month.
	EnvelopeOverspent Type = "envelope.overspent"
	// EnvelopeGoalDue: a target-by-date goal is due soon and not funded.
	EnvelopeGoalDue Type = "envelope.goal_due"
)

// Types are the known event types in a stable order.
var Types = []Type{SubscriptionRenewalUpcoming, SubscriptionTrialEnding, SubscriptionPriceChanged, SubscriptionChargeMissed,
	EnvelopeOverspent, EnvelopeGoalDue}

// Valid reports whether t is a known event type.
func (t Type) Valid() bool {
	for _, k := range Types {
		if k == t {
			return true
		}
	}
	return false
}

// Resource types of events.
const (
	ResourceSubscription = "subscription"
	ResourceEnvelope     = "envelope"
)

// Event is one fact of a household, waiting in the outbox until it is
// dispatched.
type Event struct {
	ID           domain.ID
	HouseholdID  domain.ID
	Type         Type
	ResourceType string
	ResourceID   domain.ID
	// DedupeKey is unique within the household: an event with a key the
	// outbox has already is not stored again.
	DedupeKey string
	// Payload holds the event's facts as strings: decimal amounts next to
	// their currency, dates as 2006-01-02.
	Payload    map[string]string
	OccurredAt time.Time
	// DispatchedAt is zero until the event is dispatched.
	DispatchedAt time.Time
}

// Dispatched reports whether the event has been dispatched.
func (e Event) Dispatched() bool { return !e.DispatchedAt.IsZero() }
