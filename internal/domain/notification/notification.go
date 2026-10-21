package notification

import (
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
)

// Notification is one message in the app for one person, made from an
// event of a household.
type Notification struct {
	ID          domain.ID
	UserID      domain.ID
	HouseholdID domain.ID
	EventID     domain.ID
	Type        event.Type
	Title       string
	Body        string
	// Link is the path of the web UI the notification is about, such as
	// /h/<household>/subscriptions/<id>.
	Link string
	// WalletID is the wallet of the event, "" without one; a bot limited
	// to wallets sees only notifications of its wallets.
	WalletID     domain.ID
	ResourceType string
	ResourceID   domain.ID
	CreatedAt    time.Time
	// ReadAt is zero while unread.
	ReadAt time.Time
}

// Read reports whether the notification has been read.
func (n Notification) Read() bool { return !n.ReadAt.IsZero() }

// ReadRetention is how long read notifications are kept.
const ReadRetention = 180 * 24 * time.Hour

// DeliveryRetention is how long sent and failed deliveries are kept.
const DeliveryRetention = 30 * 24 * time.Hour

// EventRetention is how long dispatched events are kept at least: their
// dedupe keys stop reminders from being sent twice (stage 004, AD-406).
const EventRetention = 400 * 24 * time.Hour
