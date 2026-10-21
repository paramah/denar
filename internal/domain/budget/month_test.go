package budget

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func month(s string) Month {
	m, err := ParseMonth("month", s)
	if err != nil {
		panic(err)
	}
	return m
}

func TestParseMonth(t *testing.T) {
	for _, s := range []string{"2026-10", "2000-01", "2100-12"} {
		m, err := ParseMonth("month", s)
		if err != nil || m.String() != s {
			t.Errorf("ParseMonth(%q) = %v, %v", s, m, err)
		}
	}
	for _, s := range []string{"", "2026-13", "2026-00", "1999-12", "2101-01", "2026-1", "26-10", "2026-10-01", "2026/10", " 2026-10"} {
		if _, err := ParseMonth("month", s); err == nil {
			t.Errorf("ParseMonth(%q) accepted", s)
		}
	}
	if !(Month{}).IsZero() || month("2026-10").IsZero() || (Month{}).String() != "" {
		t.Error("zero month")
	}
}

func TestMonthOfAndRange(t *testing.T) {
	tests := []struct {
		date     string
		startDay int
		want     string
		start    string
		end      string
	}{
		{"2026-10-01", 1, "2026-10", "2026-10-01", "2026-10-31"},
		{"2026-10-31", 1, "2026-10", "2026-10-01", "2026-10-31"},
		{"2026-02-28", 1, "2026-02", "2026-02-01", "2026-02-28"},
		{"2028-02-29", 1, "2028-02", "2028-02-01", "2028-02-29"},
		{"2026-10-10", 10, "2026-10", "2026-10-10", "2026-11-09"},
		{"2026-10-09", 10, "2026-09", "2026-09-10", "2026-10-09"},
		{"2026-01-05", 10, "2025-12", "2025-12-10", "2026-01-09"},
		{"2026-12-28", 28, "2026-12", "2026-12-28", "2027-01-27"},
		{"2026-03-27", 28, "2026-02", "2026-02-28", "2026-03-27"},
	}
	for _, tt := range tests {
		m := MonthOf(day(tt.date), tt.startDay)
		if m.String() != tt.want {
			t.Errorf("MonthOf(%s, %d) = %s, want %s", tt.date, tt.startDay, m, tt.want)
		}
		if got := m.Start(tt.startDay).String(); got != tt.start {
			t.Errorf("%s.Start(%d) = %s, want %s", m, tt.startDay, got, tt.start)
		}
		if got := m.End(tt.startDay).String(); got != tt.end {
			t.Errorf("%s.End(%d) = %s, want %s", m, tt.startDay, got, tt.end)
		}
	}
}

func TestMonthArithmetic(t *testing.T) {
	m := month("2026-12")
	if m.Next().String() != "2027-01" || m.Prev().String() != "2026-11" || month("2026-01").Prev().String() != "2025-12" {
		t.Error("Next/Prev")
	}
	if !m.After(month("2026-11")) || !month("2026-11").Before(m) || m.Compare(month("2026-12")) != 0 {
		t.Error("Compare")
	}
	tests := []struct {
		from, to string
		want     int
	}{
		{"2026-10", "2026-10", 0}, {"2026-10", "2026-12", 2}, {"2026-10", "2027-01", 3}, {"2026-10", "2026-08", -2},
	}
	for _, tt := range tests {
		if got := month(tt.from).MonthsUntil(month(tt.to)); got != tt.want {
			t.Errorf("%s.MonthsUntil(%s) = %d, want %d", tt.from, tt.to, got, tt.want)
		}
	}
}
