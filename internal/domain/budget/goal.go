package budget

import "github.com/paramah/denar/internal/domain"

// GoalStatus is how far an envelope is with its goal in a month.
type GoalStatus struct {
	// Needed is what the goal asks to assign this month.
	Needed int64
	// Underfunded is what is still missing of Needed (≥ 0).
	Underfunded int64
	Funded      bool
	// Progress is a whole percentage, 0 to 100.
	Progress int
}

// Status returns the goal's status for the envelope's month m. A monthly
// goal asks for its amount to be assigned every month. A target-by-date
// goal spreads what is still missing of its amount, rounded up, evenly over
// the months left up to the month of its date (one month once the date is
// reached or past); what counts as gathered is the available money without
// this month's assignment.
func Status(g Goal, e EnvelopeMonth, m Month, startDay int) GoalStatus {
	var s GoalStatus
	switch g.Type {
	case GoalMonthly:
		s.Needed = g.Amount
		s.Underfunded = max(0, g.Amount-e.Assigned)
		s.Progress = percent(e.Assigned, g.Amount)
	case GoalTargetByDate:
		left := max(1, m.MonthsUntil(MonthOf(g.TargetDate, startDay))+1)
		gathered := e.Available - e.Assigned
		missing := max(0, g.Amount-gathered)
		s.Needed = ceilDiv(missing, int64(left))
		s.Underfunded = max(0, s.Needed-e.Assigned)
		s.Progress = percent(e.Available, g.Amount)
	default:
		return GoalStatus{}
	}
	s.Funded = s.Underfunded == 0
	return s
}

// percent returns part/whole as a whole percentage clamped to 0..100;
// whole is positive and at most MaxGoalAmount, so part*100 cannot
// overflow once part is clamped to whole.
func percent(part, whole int64) int {
	if part <= 0 {
		return 0
	}
	if part >= whole {
		return 100
	}
	return int(part * 100 / whole)
}

func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a-1)/b + 1
}

// Candidate is an envelope auto-assign may fund.
type Candidate struct {
	EnvelopeID  domain.ID
	Underfunded int64
}

// Allocation is money auto-assign gives an envelope.
type Allocation struct {
	EnvelopeID domain.ID
	Amount     int64
}

// AutoAssign funds the candidates in their order, each with as much of its
// underfunding as Ready to Assign still covers, until nothing is left.
func AutoAssign(candidates []Candidate, readyToAssign int64) []Allocation {
	out := []Allocation{}
	for _, c := range candidates {
		if readyToAssign <= 0 {
			break
		}
		give := min(c.Underfunded, readyToAssign)
		if give <= 0 {
			continue
		}
		out = append(out, Allocation{EnvelopeID: c.EnvelopeID, Amount: give})
		readyToAssign -= give
	}
	return out
}
