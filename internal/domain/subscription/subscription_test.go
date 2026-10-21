package subscription

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/money"
)

func price(from string, amount int64, currency string) PricePoint {
	return PricePoint{ID: domain.ID("p-" + from), EffectiveFrom: day(from), Amount: amount, Currency: currency}
}

func openAI() Subscription {
	return Subscription{ID: "s1", HouseholdID: "h1", Name: "ChatGPT Plus", Vendor: "openai", Status: Active,
		Cycle:  Cycle{Unit: Month, Every: 1, Anchor: day("2026-01-31")},
		Prices: []PricePoint{price("2026-01-31", 2000, "USD")}, ReminderOffsets: []int{3},
		Match: MatchRule{AmountTolerancePct: 10, DateWindowDays: 3}}
}

func TestValidate(t *testing.T) {
	s := openAI().Normalize()
	if err := s.Validate(); err != nil {
		t.Fatalf("valid: %v", err)
	}
	bad := s
	bad.Name, bad.Vendor, bad.Status, bad.ManageURL = "", "myspace", "gone", "ftp://x"
	bad.ReminderOffsets = []int{1, 2, 3, 4, 5, 61}
	bad.Match = MatchRule{AmountTolerancePct: 51, DateWindowDays: 16}
	bad.Prices = []PricePoint{{EffectiveFrom: day("2026-01-01"), Amount: 0, Currency: "XXX"}}
	err := bad.Validate()
	for _, field := range []string{"name", "vendor", "status", "manage_url", "reminder_offsets", "match_rule.amount_tolerance_pct",
		"match_rule.date_window_days", "price.amount", "price.currency"} {
		if !hasField(err, field) {
			t.Errorf("no error on %s: %v", field, err)
		}
	}
	trial := s
	trial.Status = Trial
	if !hasField(trial.Validate(), "trial_ends_at") {
		t.Error("trial without an end")
	}
	cancelled := s
	cancelled.Status = Cancelled
	if !hasField(cancelled.Normalize().Validate(), "ends_at") {
		t.Error("cancelled without ends_at")
	}
}

func hasField(err error, field string) bool {
	var ve domain.ValidationError
	if errors.As(err, &ve) && ve.Field == field {
		return true
	}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range j.Unwrap() {
			if hasField(e, field) {
				return true
			}
		}
	}
	return false
}

func TestNormalize(t *testing.T) {
	s := openAI()
	s.Name, s.ReminderOffsets = "  ChatGPT  ", []int{1, 7, 1, 3}
	s.Prices = []PricePoint{price("2026-06-01", 2200, "USD"), price("2026-01-31", 2000, "USD")}
	s.CancelledAt, s.EndsAt = time.Now(), day("2026-12-01")
	n := s.Normalize()
	if n.Name != "ChatGPT" || len(n.ReminderOffsets) != 3 || n.ReminderOffsets[0] != 7 || n.ReminderOffsets[2] != 1 {
		t.Errorf("normalised = %+v", n)
	}
	if n.Prices[0].EffectiveFrom != day("2026-01-31") || !n.CancelledAt.IsZero() || !n.EndsAt.IsZero() {
		t.Errorf("prices/cancel = %+v", n)
	}
}

func TestPriceOn(t *testing.T) {
	s := openAI()
	s.Prices = []PricePoint{price("2026-01-31", 2000, "USD"), price("2026-06-01", 2200, "USD"), price("2027-01-01", 9900, "PLN")}
	for d, want := range map[string]int64{"2025-12-01": 2000, "2026-01-31": 2000, "2026-05-31": 2000, "2026-06-01": 2200,
		"2026-12-31": 2200, "2027-01-01": 9900, "2030-01-01": 9900} {
		if got := s.PriceOn(day(d)).Amount; got != want {
			t.Errorf("PriceOn(%s) = %d, want %d", d, got, want)
		}
	}
	prev, ok := s.PriceBefore(s.Prices[2])
	if !ok || prev.Amount != 2200 {
		t.Errorf("before = %+v %v", prev, ok)
	}
	if _, ok := s.PriceBefore(s.Prices[0]); ok {
		t.Error("before the first")
	}
}

func TestPlan(t *testing.T) {
	from, to := day("2026-10-21"), day("2027-01-19")
	plan := func(s Subscription) string {
		var ds []domain.Date
		for _, p := range s.Plan(from, to) {
			ds = append(ds, p.DueDate)
		}
		return dates(ds)
	}
	s := openAI()
	s.Prices = append(s.Prices, price("2026-12-01", 2500, "USD"))
	got := s.Plan(from, to)
	if dates([]domain.Date{got[0].DueDate, got[1].DueDate, got[2].DueDate}) != "2026-10-31 2026-11-30 2026-12-31" ||
		got[1].Amount != 2000 || got[2].Amount != 2500 || got[2].Currency != "USD" || len(got) != 3 {
		t.Errorf("active plan = %+v", got)
	}
	trial := openAI()
	trial.Status, trial.TrialEndsAt = Trial, day("2026-11-15")
	if p := plan(trial); p != "2026-11-30 2026-12-31" {
		t.Errorf("trial plan = %s", p)
	}
	trial.TrialEndsAt = day("2026-11-30")
	if p := plan(trial); p != "2026-11-30 2026-12-31" {
		t.Errorf("trial to a term = %s", p)
	}
	cancelled := openAI()
	cancelled.Status, cancelled.EndsAt = Cancelled, day("2026-11-30")
	if p := plan(cancelled); p != "2026-10-31" {
		t.Errorf("cancelled plan = %s", p)
	}
	paused := openAI()
	paused.Status = Paused
	if p := plan(paused); p != "" {
		t.Errorf("paused plan = %s", p)
	}
}

func TestReminderDue(t *testing.T) {
	s := openAI()
	s.ReminderOffsets = []int{7, 3, 0}
	due := day("2026-10-31")
	cases := []struct {
		today string
		want  int
		ok    bool
	}{
		{"2026-10-23", 0, false},
		{"2026-10-24", 7, true},
		{"2026-10-27", 7, true},
		{"2026-10-28", 3, true},
		{"2026-10-31", 0, true},
		{"2026-11-01", 0, false},
	}
	for _, c := range cases {
		got, ok := s.ReminderDue(due, day(c.today))
		if got != c.want || ok != c.ok {
			t.Errorf("today %s: %d %v, want %d %v", c.today, got, ok, c.want, c.ok)
		}
	}
	s.ReminderOffsets = nil
	if _, ok := s.ReminderDue(due, due); ok {
		t.Error("no offsets, yet a reminder")
	}
}

func TestCharge(t *testing.T) {
	c := Charge{DueDate: day("2026-10-31"), Status: Expected}
	if c.MissedOn(3) != day("2026-11-04") || c.OpenStatus(3, day("2026-11-03")) != Expected || c.OpenStatus(3, day("2026-11-04")) != Missed {
		t.Errorf("missed on %s", c.MissedOn(3))
	}
	if !c.Open() || c.Linked() {
		t.Error("expected is open")
	}
	c.Status = Posted
	if c.Open() || !c.Linked() {
		t.Error("posted is linked")
	}
}

func TestCostOf(t *testing.T) {
	pln, _ := money.LookupCurrency("PLN")
	usd, _ := money.LookupCurrency("USD")
	jpy, _ := money.LookupCurrency("JPY")
	month := Cycle{Unit: Month, Every: 1}
	cases := []struct {
		name          string
		p             PricePoint
		c             Cycle
		rate          string
		to            money.Currency
		month, yearly int64
	}{
		{"monthly USD in USD", price("2026-01-01", 2000, "USD"), month, "1", usd, 2000, 24000},
		{"monthly USD in PLN at 4.0123", price("2026-01-01", 2000, "USD"), month, "4.0123", pln, 8025, 96295},
		{"quarterly", price("2026-01-01", 3000, "PLN"), Cycle{Unit: Month, Every: 3}, "1", pln, 1000, 12000},
		{"yearly", price("2026-01-01", 11999, "PLN"), Cycle{Unit: Year, Every: 1}, "1", pln, 1000, 11999},
		{"weekly", price("2026-01-01", 1000, "PLN"), Cycle{Unit: Week, Every: 1}, "1", pln, 4348, 52179},
		{"JPY into PLN", price("2026-01-01", 1500, "JPY"), month, "0.0265", pln, 3975, 47700},
		{"PLN into JPY", price("2026-01-01", 1000, "PLN"), month, "37.5", jpy, 375, 4500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := new(big.Rat).SetString(c.rate)
			got, err := CostOf(c.p, c.c, r, c.to)
			if err != nil || got.Monthly != c.month || got.Yearly != c.yearly {
				t.Errorf("cost = %+v %v, want %d / %d", got, err, c.month, c.yearly)
			}
		})
	}
}

func TestMatchScore(t *testing.T) {
	s := openAI()
	s.WalletID = "w1"
	c := Charge{DueDate: day("2026-10-31"), Amount: 2000, Currency: "USD"}
	cases := []struct {
		name  string
		x     Candidate
		score int
		ok    bool
	}{
		{"exact, same day, payee, wallet", Candidate{WalletID: "w1", Date: day("2026-10-31"), Amount: 8000, Expected: 8000, Text: "OPENAI *CHATGPT SUBSCR"}, 100, true},
		{"exact, same day, no payee, wallet", Candidate{WalletID: "w1", Date: day("2026-10-31"), Amount: 8000, Expected: 8000}, 80, true},
		{"2% off of 10%, a day late, payee, other wallet", Candidate{WalletID: "w2", Date: day("2026-11-01"), Amount: 8160, Expected: 8000, Text: "OpenAI"}, 36 + 25 + 20, true},
		{"at the edge of the tolerance", Candidate{WalletID: "w2", Date: day("2026-10-31"), Amount: 7200, Expected: 8000}, 20 + 30, true},
		{"beyond the tolerance", Candidate{Date: day("2026-10-31"), Amount: 7199, Expected: 8000}, 0, false},
		{"at the edge of the window", Candidate{Date: day("2026-11-03"), Amount: 8000, Expected: 8000}, 40 + 15, true},
		{"beyond the window", Candidate{Date: day("2026-10-27"), Amount: 8000, Expected: 8000}, 0, false},
		{"rate unknown", Candidate{WalletID: "w1", Date: day("2026-10-31"), Amount: 8000, Text: "chatgpt plus"}, 30 + 20 + 10, true},
		{"not an expense", Candidate{Date: day("2026-10-31"), Amount: 0, Expected: 8000}, 0, false},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			score, ok := MatchScore(s, c, cs.x)
			if score != cs.score || ok != cs.ok {
				t.Errorf("score = %d %v, want %d %v", score, ok, cs.score, cs.ok)
			}
		})
	}
	exact := s
	exact.Match = MatchRule{PayeePattern: "netflix.com", AmountTolerancePct: 0, DateWindowDays: 0}
	if _, ok := MatchScore(exact, c, Candidate{Date: day("2026-10-31"), Amount: 8001, Expected: 8000}); ok {
		t.Error("zero tolerance takes an inexact amount")
	}
	if score, ok := MatchScore(exact, c, Candidate{Date: day("2026-10-31"), Amount: 8000, Expected: 8000, Text: "OpenAI"}); !ok || score != 70 {
		t.Errorf("the pattern replaces the names: %d %v", score, ok)
	}
	if _, ok := MatchScore(exact, c, Candidate{Date: day("2026-11-01"), Amount: 8000, Expected: 8000}); ok {
		t.Error("zero window takes another day")
	}
	big := Candidate{Date: day("2026-10-31"), Amount: 900_000_000_000_000_000, Expected: 900_000_000_000_000_000}
	if score, ok := MatchScore(s, c, big); !ok || score != 70 {
		t.Errorf("huge amounts: %d %v", score, ok)
	}
}

func TestVendors(t *testing.T) {
	if v, ok := LookupVendor("openai"); !ok || v.Name != "OpenAI" {
		t.Errorf("openai = %+v", v)
	}
	if _, ok := LookupVendor("OpenAI"); ok {
		t.Error("keys are lower case")
	}
	if VendorName("other") != "Inne" || VendorName("zzz") != "zzz" {
		t.Error("names")
	}
	seen := map[string]bool{}
	for _, v := range Vendors {
		if seen[v.Key] || v.Name == "" {
			t.Errorf("vendor %+v", v)
		}
		seen[v.Key] = true
	}
}
