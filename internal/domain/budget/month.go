// Package budget models the envelope budget (YNAB style) in a household's
// base currency: envelope groups, envelopes with goals, monthly
// assignments, and the pure computations of a budget month — available
// money carried over from month to month, overspending and Ready to
// Assign.
package budget

import (
	"fmt"
	"time"

	"github.com/paramah/denar/internal/domain"
)

// Month is a budget month. It is labelled by the calendar month it starts
// in: with a household's month start day of 10, month 2026-10 runs from
// 2026-10-10 to 2026-11-09. The zero value means no month.
type Month struct {
	y int
	m time.Month
}

// MonthLayout is how months are written: 2006-01.
const MonthLayout = "2006-01"

// Years a month may fall in.
const (
	MinYear = 2000
	MaxYear = 2100
)

// NewMonth returns the month; out-of-range months normalise as time.Date
// does.
func NewMonth(year int, month time.Month) Month {
	t := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return Month{y: t.Year(), m: t.Month()}
}

// ParseMonth reads a month written as 2006-01 between MinYear and MaxYear;
// anything else is a domain.ValidationError on field.
func ParseMonth(field, s string) (Month, error) {
	bad := domain.ValidationError{Field: field, Message: fmt.Sprintf("must be a month written as YYYY-MM between %d-01 and %d-12", MinYear, MaxYear)}
	if len(s) != len(MonthLayout) {
		return Month{}, bad
	}
	t, err := time.Parse(MonthLayout, s)
	if err != nil || t.Year() < MinYear || t.Year() > MaxYear {
		return Month{}, bad
	}
	return Month{y: t.Year(), m: t.Month()}, nil
}

// MonthOf returns the budget month the day falls in for a household whose
// months start on startDay (1 to 28).
func MonthOf(d domain.Date, startDay int) Month {
	y, m, day := d.Time().Date()
	if day < startDay {
		return NewMonth(y, m-1)
	}
	return Month{y: y, m: m}
}

func (m Month) IsZero() bool { return m.y == 0 }

// String writes the month as 2006-01, or "" for the zero month.
func (m Month) String() string {
	if m.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", m.y, int(m.m))
}

// Start is the first day of the month.
func (m Month) Start(startDay int) domain.Date { return domain.NewDate(m.y, m.m, startDay) }

// End is the last day of the month.
func (m Month) End(startDay int) domain.Date { return m.Next().Start(startDay).AddDays(-1) }

func (m Month) Next() Month { return NewMonth(m.y, m.m+1) }

func (m Month) Prev() Month { return NewMonth(m.y, m.m-1) }

func (m Month) index() int { return m.y*12 + int(m.m) - 1 }

// Compare returns -1, 0 or +1 as m is before, equal to or after o.
func (m Month) Compare(o Month) int {
	switch a, b := m.index(), o.index(); {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (m Month) Before(o Month) bool { return m.Compare(o) < 0 }

func (m Month) After(o Month) bool { return m.Compare(o) > 0 }

// MonthsUntil returns how many months o is after m (negative when before).
func (m Month) MonthsUntil(o Month) int { return o.index() - m.index() }
