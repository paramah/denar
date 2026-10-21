package domain

import "time"

// Date is a calendar day without a time zone, such as the day of a
// transaction. The zero value means no date.
type Date struct {
	t time.Time // midnight UTC
}

// DateLayout is how dates are written: 2006-01-02.
const DateLayout = time.DateOnly

// NewDate returns the day; out-of-range values normalise as time.Date does.
func NewDate(year int, month time.Month, day int) Date {
	return Date{t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate reads a date written as 2006-01-02.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, err
	}
	return Date{t: t}, nil
}

// DateOf returns the day t falls on in loc.
func DateOf(t time.Time, loc *time.Location) Date {
	y, m, d := t.In(loc).Date()
	return NewDate(y, m, d)
}

// IsZero reports whether the date is unset.
func (d Date) IsZero() bool { return d.t.IsZero() }

// String writes the date as 2006-01-02, or "" for the zero date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(DateLayout)
}

// Time returns midnight UTC of the day.
func (d Date) Time() time.Time { return d.t }

// AddDays returns the date n days later (earlier for a negative n).
func (d Date) AddDays(n int) Date { return Date{t: d.t.AddDate(0, 0, n)} }

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int { return d.t.Compare(o.t) }

func (d Date) Before(o Date) bool { return d.t.Before(o.t) }

func (d Date) After(o Date) bool { return d.t.After(o.t) }
