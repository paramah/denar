package subscription

import (
	"slices"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// ChargeStatus is where an expected charge stands.
type ChargeStatus string

const (
	// Expected: due, not seen paid yet.
	Expected ChargeStatus = "expected"
	// Matched: linked to a transaction recorded otherwise.
	Matched ChargeStatus = "matched"
	// Missed: not seen paid within the date window.
	Missed ChargeStatus = "missed"
	// Skipped: will not be paid (a free month, a refund).
	Skipped ChargeStatus = "skipped"
	// Posted: a transaction was recorded from it.
	Posted ChargeStatus = "posted"
)

// ChargeStatuses in a stable order.
var ChargeStatuses = []ChargeStatus{Expected, Matched, Missed, Skipped, Posted}

func (s ChargeStatus) Valid() bool { return slices.Contains(ChargeStatuses, s) }

// Charge is one term a subscription is expected to be charged on. The
// amount is in the currency of the price on the due date.
type Charge struct {
	ID             domain.ID
	HouseholdID    domain.ID
	SubscriptionID domain.ID
	DueDate        domain.Date
	Amount         int64
	Currency       string
	Status         ChargeStatus
	// TransactionID is set exactly when the charge is matched or posted.
	TransactionID domain.ID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Open reports whether the charge still waits for a payment.
func (c Charge) Open() bool { return c.Status == Expected || c.Status == Missed }

// Linked reports whether the charge is tied to a transaction.
func (c Charge) Linked() bool { return c.Status == Matched || c.Status == Posted }

// MissedOn is the first day a charge without a payment counts as missed:
// the day after its date window ends.
func (c Charge) MissedOn(window int) domain.Date { return c.DueDate.AddDays(window + 1) }

// OpenStatus is the status an unlinked charge goes back to on the day:
// missed once its window has passed, expected before.
func (c Charge) OpenStatus(window int, today domain.Date) ChargeStatus {
	if today.Before(c.MissedOn(window)) {
		return Expected
	}
	return Missed
}
