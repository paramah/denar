package subscription

import (
	"cmp"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/money"
)

// Limits of a subscription.
const (
	MaxNameLength        = 100
	MaxExternalRefLength = 200
	MaxNoteLength        = 1000
	MaxURLLength         = 500
	MaxPatternLength     = 100
	MaxReminders         = 5
	MaxReminderOffset    = 60
	MaxTolerancePct      = 50
	MaxDateWindowDays    = 15
	// MaxAmount is the largest price, in minor units.
	MaxAmount int64 = 1_000_000_000_000_000
	// DefaultTolerancePct and DefaultDateWindowDays are the match rule of a
	// subscription that sets none.
	DefaultTolerancePct   = 10
	DefaultDateWindowDays = 3
)

// DefaultReminderOffsets are the reminders of a new subscription that sets
// none: three days before.
var DefaultReminderOffsets = []int{3}

// Status is where a subscription is in its life.
type Status string

const (
	// Trial: free until TrialEndsAt; paid terms start then.
	Trial Status = "trial"
	// Active: paid every term.
	Active Status = "active"
	// Paused: no terms are expected.
	Paused Status = "paused"
	// Cancelled: no terms from EndsAt on; access lasts until then.
	Cancelled Status = "cancelled"
)

// Statuses in a stable order.
var Statuses = []Status{Trial, Active, Paused, Cancelled}

func (s Status) Valid() bool { return slices.Contains(Statuses, s) }

// PricePoint is the price from a day on. A price change adds a point; the
// earlier ones stay.
type PricePoint struct {
	ID            domain.ID
	EffectiveFrom domain.Date
	Amount        int64 // > 0, minor units of Currency
	Currency      string
	CreatedAt     time.Time
}

// Validate checks the point; field is where its input came from.
func (p PricePoint) Validate(field string) error {
	var errs []error
	if err := money.ValidateCurrency(field+".currency", p.Currency); err != nil {
		errs = append(errs, err)
	}
	if p.Amount <= 0 || p.Amount > MaxAmount {
		errs = append(errs, domain.ValidationError{Field: field + ".amount", Message: "must be greater than zero"})
	}
	return errors.Join(errs...)
}

// MatchRule says how close a transaction must be to a charge.
type MatchRule struct {
	// PayeePattern occurs, ignoring case, in the payee or note of matching
	// transactions; empty means the vendor's and the subscription's name.
	PayeePattern string
	// AmountTolerancePct is how far, in percent, the amount may be off.
	AmountTolerancePct int
	// DateWindowDays is how many days before or after the due date the
	// transaction may be, and how long a charge waits before it is missed.
	DateWindowDays int
}

// Subscription is a recurring cost.
type Subscription struct {
	ID          domain.ID
	HouseholdID domain.ID
	Name        string
	Vendor      string // key of Vendors
	ExternalRef string // the vendor's id, for future connectors
	Cycle       Cycle
	TrialEndsAt domain.Date
	Status      Status
	CancelledAt time.Time
	// EndsAt is the first day not paid for of a cancelled subscription.
	EndsAt     domain.Date
	WalletID   domain.ID // the wallet it is paid from, if any
	EnvelopeID domain.ID
	// ReminderOffsets are days before a due date to remind on, descending.
	ReminderOffsets []int
	Match           MatchRule
	Note            string
	ManageURL       string
	// Prices are ordered by EffectiveFrom, ascending; there is at least one.
	Prices    []PricePoint
	CreatedBy domain.Actor
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Normalize trims the text, orders the reminders and the prices, and fills
// in the default match rule values a zero rule leaves out.
func (s Subscription) Normalize() Subscription {
	s.Name = strings.TrimSpace(s.Name)
	s.Vendor = strings.TrimSpace(s.Vendor)
	s.ExternalRef = strings.TrimSpace(s.ExternalRef)
	s.Note = strings.TrimSpace(s.Note)
	s.ManageURL = strings.TrimSpace(s.ManageURL)
	s.Match.PayeePattern = strings.TrimSpace(s.Match.PayeePattern)
	offsets := slices.Clone(s.ReminderOffsets)
	slices.Sort(offsets)
	offsets = slices.Compact(offsets)
	slices.Reverse(offsets)
	if offsets == nil {
		offsets = []int{}
	}
	s.ReminderOffsets = offsets
	s.Prices = slices.Clone(s.Prices)
	SortPrices(s.Prices)
	if s.Status != Cancelled {
		s.CancelledAt, s.EndsAt = time.Time{}, domain.Date{}
	}
	return s
}

// SortPrices orders price points by the day they take effect.
func SortPrices(ps []PricePoint) {
	slices.SortStableFunc(ps, func(a, b PricePoint) int { return a.EffectiveFrom.Compare(b.EffectiveFrom) })
}

// Validate checks a normalised subscription.
func (s Subscription) Validate() error {
	var errs []error
	add := func(field, msg string) { errs = append(errs, domain.ValidationError{Field: field, Message: msg}) }
	if n := utf8.RuneCountInString(s.Name); n == 0 || n > MaxNameLength {
		add("name", "must be 1 to 100 characters")
	}
	if _, ok := LookupVendor(s.Vendor); !ok {
		add("vendor", "must be one of the known vendors")
	}
	if utf8.RuneCountInString(s.ExternalRef) > MaxExternalRefLength {
		add("external_ref", "must be at most 200 characters")
	}
	if err := s.Cycle.Validate(); err != nil {
		errs = append(errs, err)
	}
	if !s.Status.Valid() {
		add("status", "must be trial, active, paused or cancelled")
	}
	if err := validateDay("trial_ends_at", s.TrialEndsAt, s.Status == Trial); err != nil {
		errs = append(errs, err)
	}
	if err := validateDay("ends_at", s.EndsAt, s.Status == Cancelled); err != nil {
		errs = append(errs, err)
	}
	if len(s.ReminderOffsets) > MaxReminders {
		add("reminder_offsets", "must hold at most 5 values")
	}
	for _, o := range s.ReminderOffsets {
		if o < 0 || o > MaxReminderOffset {
			add("reminder_offsets", "must hold days between 0 and 60")
			break
		}
	}
	if utf8.RuneCountInString(s.Match.PayeePattern) > MaxPatternLength {
		add("match_rule.payee_pattern", "must be at most 100 characters")
	}
	if s.Match.AmountTolerancePct < 0 || s.Match.AmountTolerancePct > MaxTolerancePct {
		add("match_rule.amount_tolerance_pct", "must be between 0 and 50")
	}
	if s.Match.DateWindowDays < 0 || s.Match.DateWindowDays > MaxDateWindowDays {
		add("match_rule.date_window_days", "must be between 0 and 15")
	}
	if utf8.RuneCountInString(s.Note) > MaxNoteLength {
		add("note", "must be at most 1000 characters")
	}
	if s.ManageURL != "" && !validURL(s.ManageURL) {
		add("manage_url", "must be an http or https URL of at most 500 characters")
	}
	if len(s.Prices) == 0 {
		add("price", "is required")
	}
	for _, p := range s.Prices {
		if err := p.Validate("price"); err != nil {
			errs = append(errs, err)
		}
		if err := validateDay("effective_from", p.EffectiveFrom, true); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validURL(s string) bool {
	if len(s) > MaxURLLength {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// PriceOn is the price on the day: the last point in effect by then, or
// the first point for a day before it.
func (s Subscription) PriceOn(d domain.Date) PricePoint {
	if len(s.Prices) == 0 {
		return PricePoint{}
	}
	p := s.Prices[0]
	for _, q := range s.Prices[1:] {
		if q.EffectiveFrom.After(d) {
			break
		}
		p = q
	}
	return p
}

// PriceBefore is the point in effect before p, if any.
func (s Subscription) PriceBefore(p PricePoint) (PricePoint, bool) {
	var prev PricePoint
	found := false
	for _, q := range s.Prices {
		if !q.EffectiveFrom.Before(p.EffectiveFrom) {
			break
		}
		prev, found = q, true
	}
	return prev, found
}

// Planned is a term the subscription is expected to be charged on.
type Planned struct {
	DueDate  domain.Date
	Amount   int64
	Currency string
}

// Expects reports whether a term on the day is to be paid: every term of
// an active subscription, those from the end of a trial, those before
// EndsAt of a cancelled one, none of a paused one.
func (s Subscription) Expects(due domain.Date) bool {
	switch s.Status {
	case Active:
		return true
	case Trial:
		return !due.Before(s.TrialEndsAt)
	case Cancelled:
		return due.Before(s.EndsAt)
	}
	return false
}

// Plan returns the terms within [from, to] to be paid, with the price of
// each day.
func (s Subscription) Plan(from, to domain.Date) []Planned {
	out := []Planned{}
	if s.Status == Paused || len(s.Prices) == 0 {
		return out
	}
	for _, d := range s.Cycle.Between(from, to) {
		if !s.Expects(d) {
			continue
		}
		p := s.PriceOn(d)
		out = append(out, Planned{DueDate: d, Amount: p.Amount, Currency: p.Currency})
	}
	return out
}

// ReminderDue returns the reminder to send today for a due date: the
// smallest offset whose day (due − offset) has come, while the due date
// has not passed. Larger offsets whose day passed unsent are not sent late.
func (s Subscription) ReminderDue(due, today domain.Date) (int, bool) {
	if due.Before(today) {
		return 0, false
	}
	best, ok := 0, false
	for _, o := range s.ReminderOffsets {
		if !due.AddDays(-o).After(today) && (!ok || o < best) {
			best, ok = o, true
		}
	}
	return best, ok
}

// SortByName orders subscriptions by name, ignoring case, then by id.
func SortByName(ss []Subscription) {
	slices.SortFunc(ss, func(a, b Subscription) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
}

// NextCharge is the first term on or after the day that is to be paid; ok
// is false when there is none (paused, or cancelled before it).
func (s Subscription) NextCharge(from domain.Date) (domain.Date, bool) {
	switch s.Status {
	case Paused:
		return domain.Date{}, false
	case Trial:
		if from.Before(s.TrialEndsAt) {
			from = s.TrialEndsAt
		}
	}
	d := s.Cycle.OnOrAfter(from)
	if !s.Expects(d) {
		return domain.Date{}, false
	}
	return d, true
}
