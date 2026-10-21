package notification

import (
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
)

// DeliveryStatus is where a delivery stands.
type DeliveryStatus string

const (
	Pending DeliveryStatus = "pending"
	Sent    DeliveryStatus = "sent"
	Failed  DeliveryStatus = "failed"
)

// MaxAttempts is how many times a delivery is tried before it fails.
const MaxAttempts = 6

// Backoff bounds.
const (
	backoffBase = time.Minute
	backoffMax  = time.Hour
)

// Backoff is the wait after the attempts-th failed attempt: one minute,
// doubling each time, at most an hour.
func Backoff(attempts int) time.Duration {
	if attempts < 1 {
		return 0
	}
	d := backoffBase
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= backoffMax {
			return backoffMax
		}
	}
	return d
}

// Delivery is one message waiting to go, or gone, to one channel of one
// person. It keeps its own copy of the content.
type Delivery struct {
	ID          domain.ID
	EventID     domain.ID
	UserID      domain.ID
	HouseholdID domain.ID
	ChannelID   domain.ID
	Type        event.Type
	Title       string
	Body        string
	// URL is absolute: the base URL and the notification's link.
	URL        string
	Data       map[string]string
	OccurredAt time.Time

	Status        DeliveryStatus
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SentAt        time.Time
}

// Succeeded records a successful attempt.
func (d Delivery) Succeeded(now time.Time) Delivery {
	d.Attempts++
	d.Status, d.SentAt, d.UpdatedAt, d.LastError = Sent, now, now, ""
	return d
}

// Failure records a failed attempt: the delivery is tried again after the
// backoff, or fails for good after MaxAttempts.
func (d Delivery) Failure(now time.Time, reason string) Delivery {
	d.Attempts++
	d.LastError, d.UpdatedAt = reason, now
	if d.Attempts >= MaxAttempts {
		d.Status = Failed
		return d
	}
	d.NextAttemptAt = now.Add(Backoff(d.Attempts))
	return d
}

// Abandon fails the delivery at once, for a reason no retry can fix.
func (d Delivery) Abandon(now time.Time, reason string) Delivery {
	d.Status, d.LastError, d.UpdatedAt = Failed, reason, now
	return d
}
