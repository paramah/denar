package budget

import (
	"slices"
	"testing"
)

func TestGoalStatus(t *testing.T) {
	monthly := Goal{Type: GoalMonthly, Amount: 50000}
	trip := Goal{Type: GoalTargetByDate, Amount: 120000, TargetDate: day("2027-03-15")}
	tests := []struct {
		name  string
		g     Goal
		e     EnvelopeMonth
		month string
		start int
		want  GoalStatus
	}{
		{"no goal", Goal{}, EnvelopeMonth{Assigned: 5}, "2026-10", 1, GoalStatus{}},
		{"monthly nothing yet", monthly, EnvelopeMonth{}, "2026-10", 1, GoalStatus{Needed: 50000, Underfunded: 50000}},
		{"monthly partly", monthly, EnvelopeMonth{Assigned: 20000, Available: 20000}, "2026-10", 1, GoalStatus{Needed: 50000, Underfunded: 30000, Progress: 40}},
		{"monthly done, carry does not count", monthly, EnvelopeMonth{Assigned: 50000, CarriedIn: 9000, Available: 59000}, "2026-10", 1,
			GoalStatus{Needed: 50000, Funded: true, Progress: 100}},
		{"monthly over", monthly, EnvelopeMonth{Assigned: 70000}, "2026-10", 1, GoalStatus{Needed: 50000, Funded: true, Progress: 100}},
		{"monthly negative assignment", monthly, EnvelopeMonth{Assigned: -100}, "2026-10", 1, GoalStatus{Needed: 50000, Underfunded: 50100}},
		// 2026-10 .. 2027-03 is six months: 120000 / 6.
		{"target from scratch", trip, EnvelopeMonth{}, "2026-10", 1, GoalStatus{Needed: 20000, Underfunded: 20000}},
		{"target with savings", trip, EnvelopeMonth{CarriedIn: 30000, Available: 30000}, "2026-10", 1, GoalStatus{Needed: 15000, Underfunded: 15000, Progress: 25}},
		{"target assigned this month", trip, EnvelopeMonth{Assigned: 15000, CarriedIn: 30000, Available: 45000}, "2026-10", 1,
			GoalStatus{Needed: 15000, Funded: true, Progress: 37}},
		{"target rounds up", Goal{Type: GoalTargetByDate, Amount: 100, TargetDate: day("2026-12-01")}, EnvelopeMonth{}, "2026-10", 1,
			GoalStatus{Needed: 34, Underfunded: 34}},
		{"target in its month", trip, EnvelopeMonth{CarriedIn: 100000, Available: 100000}, "2027-03", 1, GoalStatus{Needed: 20000, Underfunded: 20000, Progress: 83}},
		{"target past due asks for the rest", trip, EnvelopeMonth{CarriedIn: 100000, Available: 100000}, "2027-07", 1, GoalStatus{Needed: 20000, Underfunded: 20000, Progress: 83}},
		{"target reached", trip, EnvelopeMonth{CarriedIn: 130000, Available: 130000}, "2026-11", 1, GoalStatus{Funded: true, Progress: 100}},
		{"target spending counts", trip, EnvelopeMonth{CarriedIn: 30000, Activity: -30000}, "2026-10", 1, GoalStatus{Needed: 20000, Underfunded: 20000}},
		// With months starting on the 20th, 2027-03-15 falls in 2027-02: five months from 2026-10.
		{"target month by start day", trip, EnvelopeMonth{}, "2026-10", 20, GoalStatus{Needed: 24000, Underfunded: 24000}},
	}
	for _, tt := range tests {
		if got := Status(tt.g, tt.e, month(tt.month), tt.start); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
	big := Goal{Type: GoalMonthly, Amount: MaxGoalAmount}
	if s := Status(big, EnvelopeMonth{Assigned: MaxGoalAmount - 1}, month("2026-10"), 1); s.Progress != 99 {
		t.Errorf("large progress = %d", s.Progress)
	}
}

func TestAutoAssign(t *testing.T) {
	cands := []Candidate{{"a", 300}, {"b", 0}, {"c", 500}, {"d", 100}}
	tests := []struct {
		rta  int64
		want []Allocation
	}{
		{0, []Allocation{}},
		{-50, []Allocation{}},
		{200, []Allocation{{"a", 200}}},
		{700, []Allocation{{"a", 300}, {"c", 400}}},
		{5000, []Allocation{{"a", 300}, {"c", 500}, {"d", 100}}},
	}
	for _, tt := range tests {
		if got := AutoAssign(cands, tt.rta); !slices.Equal(got, tt.want) {
			t.Errorf("AutoAssign(%d) = %v, want %v", tt.rta, got, tt.want)
		}
	}
}
