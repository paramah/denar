package domain

import (
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	d, err := ParseDate("2026-02-28")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.AddDays(1).String(); got != "2026-03-01" {
		t.Errorf("AddDays = %s", got)
	}
	if got := d.AddDays(-6).String(); got != "2026-02-22" {
		t.Errorf("AddDays(-6) = %s", got)
	}
	for _, bad := range []string{"", "2026-2-28", "2026-02-30", "28.02.2026", "2026-02-28T00:00:00Z"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) accepted", bad)
		}
	}
	if (Date{}).String() != "" || !(Date{}).IsZero() {
		t.Error("zero date")
	}
	warsaw, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	late := time.Date(2026, 10, 20, 23, 30, 0, 0, time.UTC)
	if got := DateOf(late, warsaw).String(); got != "2026-10-21" {
		t.Errorf("DateOf = %s", got)
	}
	if !NewDate(2026, 1, 1).Before(NewDate(2026, 1, 2)) || NewDate(2026, 1, 1).Compare(NewDate(2026, 1, 1)) != 0 {
		t.Error("compare")
	}
}
