package subscription

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
)

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func dates(ds []domain.Date) string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return strings.Join(out, " ")
}

func TestOccurrences(t *testing.T) {
	cases := []struct {
		name  string
		cycle Cycle
		want  string // the first occurrences
	}{
		{"monthly from the 31st", Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")},
			"2026-01-31 2026-02-28 2026-03-31 2026-04-30 2026-05-31 2026-06-30 2026-07-31"},
		{"monthly from the 31st in a leap year", Cycle{Unit: Month, Every: 1, Anchor: day("2028-01-31")},
			"2028-01-31 2028-02-29 2028-03-31"},
		{"monthly from the 30th", Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-30")},
			"2026-01-30 2026-02-28 2026-03-30"},
		{"every two months from the 31st", Cycle{Unit: Month, Every: 2, Anchor: day("2026-08-31")},
			"2026-08-31 2026-10-31 2026-12-31 2027-02-28 2027-04-30"},
		{"quarterly across the year", Cycle{Unit: Month, Every: 3, Anchor: day("2026-11-30")},
			"2026-11-30 2027-02-28 2027-05-30 2027-08-30"},
		{"yearly from 29 February", Cycle{Unit: Year, Every: 1, Anchor: day("2024-02-29")},
			"2024-02-29 2025-02-28 2026-02-28 2027-02-28 2028-02-29"},
		{"every two years", Cycle{Unit: Year, Every: 2, Anchor: day("2025-06-15")},
			"2025-06-15 2027-06-15 2029-06-15"},
		{"daily", Cycle{Unit: Day, Every: 1, Anchor: day("2026-12-30")}, "2026-12-30 2026-12-31 2027-01-01"},
		{"every ten days", Cycle{Unit: Day, Every: 10, Anchor: day("2026-02-25")}, "2026-02-25 2026-03-07 2026-03-17"},
		{"weekly", Cycle{Unit: Week, Every: 1, Anchor: day("2026-10-29")}, "2026-10-29 2026-11-05 2026-11-12"},
		{"every two weeks", Cycle{Unit: Week, Every: 2, Anchor: day("2026-12-24")}, "2026-12-24 2027-01-07 2027-01-21"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := len(strings.Fields(c.want))
			got := make([]domain.Date, n)
			for k := range n {
				got[k] = c.cycle.Occurrence(k)
			}
			if dates(got) != c.want {
				t.Errorf("occurrences = %s, want %s", dates(got), c.want)
			}
		})
	}
}

// TestNoDrift: the 48th monthly term from the 31st is still on the 31st,
// as it is counted from the anchor, not from the previous term.
func TestNoDrift(t *testing.T) {
	c := Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}
	if got := c.Occurrence(48); got != day("2030-01-31") {
		t.Errorf("occurrence 48 = %s", got)
	}
	d := c.Anchor
	for range 48 {
		d = c.NextDue(d)
	}
	if d != day("2030-01-31") {
		t.Errorf("48 × NextDue = %s", d)
	}
}

func TestOnOrAfterAndNextDue(t *testing.T) {
	cases := []struct {
		cycle                 Cycle
		from, onOrAfter, next string
	}{
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2025-12-01", "2026-01-31", "2026-01-31"},
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2026-01-31", "2026-01-31", "2026-02-28"},
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2026-02-28", "2026-02-28", "2026-03-31"},
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2026-03-01", "2026-03-31", "2026-03-31"},
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2026-10-21", "2026-10-31", "2026-10-31"},
		{Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}, "2026-10-31", "2026-10-31", "2026-11-30"},
		{Cycle{Unit: Month, Every: 3, Anchor: day("2026-01-15")}, "2026-04-16", "2026-07-15", "2026-07-15"},
		{Cycle{Unit: Year, Every: 1, Anchor: day("2024-02-29")}, "2026-03-01", "2027-02-28", "2027-02-28"},
		{Cycle{Unit: Week, Every: 2, Anchor: day("2026-10-01")}, "2026-10-14", "2026-10-15", "2026-10-15"},
		{Cycle{Unit: Week, Every: 2, Anchor: day("2026-10-01")}, "2026-10-15", "2026-10-15", "2026-10-29"},
		{Cycle{Unit: Day, Every: 7, Anchor: day("2026-10-01")}, "2027-10-01", "2027-10-07", "2027-10-07"},
	}
	for _, c := range cases {
		t.Run(c.cycle.Anchor.String()+"/"+string(c.cycle.Unit)+"/"+c.from, func(t *testing.T) {
			if got := c.cycle.OnOrAfter(day(c.from)); got != day(c.onOrAfter) {
				t.Errorf("OnOrAfter = %s, want %s", got, c.onOrAfter)
			}
			if got := c.cycle.NextDue(day(c.from)); got != day(c.next) {
				t.Errorf("NextDue = %s, want %s", got, c.next)
			}
		})
	}
}

// TestOnOrAfterAgainstWalk checks OnOrAfter against walking the terms one
// by one, for every day of three years and several cycles.
func TestOnOrAfterAgainstWalk(t *testing.T) {
	cycles := []Cycle{
		{Unit: Month, Every: 1, Anchor: day("2024-01-31")},
		{Unit: Month, Every: 5, Anchor: day("2024-03-30")},
		{Unit: Year, Every: 1, Anchor: day("2024-02-29")},
		{Unit: Week, Every: 3, Anchor: day("2024-01-03")},
		{Unit: Day, Every: 45, Anchor: day("2024-01-01")},
	}
	for _, c := range cycles {
		k := 0
		for d := day("2023-12-01"); d.Before(day("2027-01-01")); d = d.AddDays(1) {
			for c.Occurrence(k).Before(d) {
				k++
			}
			if got := c.OnOrAfter(d); got != c.Occurrence(k) {
				t.Fatalf("%+v OnOrAfter(%s) = %s, want %s", c, d, got, c.Occurrence(k))
			}
		}
	}
}

func TestBetween(t *testing.T) {
	c := Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")}
	if got := dates(c.Between(day("2026-10-21"), day("2027-01-19"))); got != "2026-10-31 2026-11-30 2026-12-31" {
		t.Errorf("between = %s", got)
	}
	if got := c.Between(day("2026-10-21"), day("2026-10-30")); len(got) != 0 {
		t.Errorf("between = %v", got)
	}
	if got := c.Between(day("2026-11-01"), day("2026-10-01")); len(got) != 0 {
		t.Errorf("reversed = %v", got)
	}
}

func TestPerYear(t *testing.T) {
	cases := []struct {
		c    Cycle
		want string
	}{
		{Cycle{Unit: Month, Every: 1}, "12/1"},
		{Cycle{Unit: Month, Every: 3}, "4/1"},
		{Cycle{Unit: Year, Every: 2}, "1/2"},
		{Cycle{Unit: Week, Every: 1}, "1461/28"},
		{Cycle{Unit: Day, Every: 1}, "1461/4"},
	}
	for _, c := range cases {
		if got := c.c.PerYear(); got.Cmp(mustRat(c.want)) != 0 {
			t.Errorf("%+v PerYear = %s, want %s", c.c, got, c.want)
		}
	}
}

func mustRat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic(s)
	}
	return r
}

func TestCycleValidate(t *testing.T) {
	ok := Cycle{Unit: Month, Every: 1, Anchor: day("2026-10-31")}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid cycle: %v", err)
	}
	for _, bad := range []Cycle{
		{Unit: "fortnight", Every: 1, Anchor: day("2026-10-31")},
		{Unit: Month, Every: 0, Anchor: day("2026-10-31")},
		{Unit: Month, Every: 366, Anchor: day("2026-10-31")},
		{Unit: Month, Every: 1},
		{Unit: Year, Every: 1, Anchor: domain.NewDate(1999, time.December, 31)},
	} {
		if bad.Validate() == nil {
			t.Errorf("%+v is valid", bad)
		}
	}
}
