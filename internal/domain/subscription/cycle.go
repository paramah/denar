package subscription

import (
	"errors"
	"math/big"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Unit is the unit of a billing cycle.
type Unit string

const (
	Day   Unit = "day"
	Week  Unit = "week"
	Month Unit = "month"
	Year  Unit = "year"
)

// Units are the cycle units in a stable order.
var Units = []Unit{Day, Week, Month, Year}

func (u Unit) Valid() bool { return u == Day || u == Week || u == Month || u == Year }

// MaxEvery is the longest step of a cycle, in its units.
const MaxEvery = 365

// maxTerms bounds Between, so that a daily cycle over a long range cannot
// make an unbounded list.
const maxTerms = 1000

// Cycle is how often a subscription is billed: every Every units, counted
// from Anchor, the day of one billing (usually the first).
type Cycle struct {
	Unit   Unit
	Every  int
	Anchor domain.Date
}

// Validate checks the cycle; the anchor must be within 2000–2100.
func (c Cycle) Validate() error {
	var errs []error
	if !c.Unit.Valid() {
		errs = append(errs, domain.ValidationError{Field: "cycle.unit", Message: "must be day, week, month or year"})
	}
	if c.Every < 1 || c.Every > MaxEvery {
		errs = append(errs, domain.ValidationError{Field: "cycle.every", Message: "must be between 1 and 365"})
	}
	if err := validateDay("cycle.anchor", c.Anchor, true); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// validateDay checks a date is set (when required) and within 2000–2100.
func validateDay(field string, d domain.Date, required bool) error {
	if d.IsZero() {
		if required {
			return domain.ValidationError{Field: field, Message: "is required"}
		}
		return nil
	}
	if y := d.Time().Year(); y < 2000 || y > 2100 {
		return domain.ValidationError{Field: field, Message: "must be between 2000 and 2100"}
	}
	return nil
}

// stepMonths is the step of a month or year cycle in months.
func (c Cycle) stepMonths() int {
	if c.Unit == Year {
		return 12 * c.Every
	}
	return c.Every
}

// stepDays is the step of a day or week cycle in days.
func (c Cycle) stepDays() int {
	if c.Unit == Week {
		return 7 * c.Every
	}
	return c.Every
}

func (c Cycle) byMonths() bool { return c.Unit == Month || c.Unit == Year }

// Occurrence returns the k-th term (k ≥ 0) counted from the anchor. For
// months and years a day past the end of the month moves to its last day;
// later terms go back to the anchor's day, as each is counted from the
// anchor (no drift).
func (c Cycle) Occurrence(k int) domain.Date {
	if c.byMonths() {
		return addMonthsClamped(c.Anchor, k*c.stepMonths())
	}
	return c.Anchor.AddDays(k * c.stepDays())
}

// addMonthsClamped moves d by n months, keeping its day or the last day of
// the month when shorter.
func addMonthsClamped(d domain.Date, n int) domain.Date {
	y, m, dd := d.Time().Date()
	total := y*12 + int(m-1) + n
	ty, tm := total/12, time.Month(total%12+1)
	last := domain.NewDate(ty, tm+1, 0).Time().Day()
	return domain.NewDate(ty, tm, min(dd, last))
}

// OnOrAfter returns the first term on or after d.
func (c Cycle) OnOrAfter(d domain.Date) domain.Date {
	if !d.After(c.Anchor) {
		return c.Anchor
	}
	var k int
	if c.byMonths() {
		ay, am, _ := c.Anchor.Time().Date()
		dy, dm, _ := d.Time().Date()
		k = ((dy-ay)*12 + int(dm-am)) / c.stepMonths()
	} else {
		days := int(d.Time().Sub(c.Anchor.Time()) / (24 * time.Hour))
		k = days / c.stepDays()
	}
	for k > 0 && !c.Occurrence(k-1).Before(d) {
		k--
	}
	for c.Occurrence(k).Before(d) {
		k++
	}
	return c.Occurrence(k)
}

// NextDue returns the first term after the day.
func (c Cycle) NextDue(after domain.Date) domain.Date { return c.OnOrAfter(after.AddDays(1)) }

// Between returns the terms within [from, to], at most 1000.
func (c Cycle) Between(from, to domain.Date) []domain.Date {
	out := []domain.Date{}
	for d := c.OnOrAfter(from); !d.After(to) && len(out) < maxTerms; d = c.NextDue(d) {
		out = append(out, d)
	}
	return out
}

// PerYear is how many terms fall in a year on average: 365.25 days a year.
func (c Cycle) PerYear() *big.Rat {
	every := int64(max(c.Every, 1))
	switch c.Unit {
	case Day:
		return big.NewRat(36525, 100*every)
	case Week:
		return big.NewRat(36525, 700*every)
	case Year:
		return big.NewRat(1, every)
	}
	return big.NewRat(12, every)
}
