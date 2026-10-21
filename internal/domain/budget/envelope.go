package budget

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/paramah/denar/internal/domain"
)

// Limits of groups, envelopes and goals.
const (
	MaxNameLength = 100
	MaxPosition   = 10000
	// MaxGoalAmount is the largest goal, in minor units: 10^13 PLN.
	MaxGoalAmount int64 = 1_000_000_000_000_000
)

// Group is a named set of envelopes, such as "Bills".
type Group struct {
	ID          domain.ID
	HouseholdID domain.ID
	Name        string
	Position    int // order among the groups, ascending
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (g Group) Normalize() Group {
	g.Name = strings.TrimSpace(g.Name)
	return g
}

func (g Group) Validate() error {
	return errors.Join(validateName(g.Name), validatePosition(g.Position))
}

// GoalType says what an envelope aims for.
type GoalType string

const (
	// GoalNone: the envelope has no goal.
	GoalNone GoalType = ""
	// GoalMonthly: assign Amount every month.
	GoalMonthly GoalType = "monthly"
	// GoalTargetByDate: have Amount available by TargetDate.
	GoalTargetByDate GoalType = "target_by_date"
)

func (t GoalType) Valid() bool { return t == GoalNone || t == GoalMonthly || t == GoalTargetByDate }

// Goal is what an envelope aims for, in minor units of the base currency.
type Goal struct {
	Type       GoalType
	Amount     int64
	TargetDate domain.Date // target_by_date only
}

// IsZero reports whether there is no goal.
func (g Goal) IsZero() bool { return g.Type == GoalNone }

// Validate checks the goal; no goal at all is valid.
func (g Goal) Validate() error {
	if !g.Type.Valid() {
		return domain.ValidationError{Field: "goal.type", Message: "must be monthly or target_by_date"}
	}
	if g.IsZero() {
		return nil
	}
	var errs []error
	if g.Amount <= 0 || g.Amount > MaxGoalAmount {
		errs = append(errs, domain.ValidationError{Field: "goal.amount.amount", Message: "must be greater than zero and at most 10^13"})
	}
	switch {
	case g.Type == GoalTargetByDate && g.TargetDate.IsZero():
		errs = append(errs, domain.ValidationError{Field: "goal.target_date", Message: "is required for a target_by_date goal"})
	case g.Type == GoalMonthly && !g.TargetDate.IsZero():
		errs = append(errs, domain.ValidationError{Field: "goal.target_date", Message: "is only for a target_by_date goal"})
	}
	return errors.Join(errs...)
}

// Envelope is a budget category of a household. Its money is in the base
// currency.
type Envelope struct {
	ID          domain.ID
	HouseholdID domain.ID
	GroupID     domain.ID
	Name        string
	Position    int // order within the group, ascending
	// Hidden envelopes stay in the computations; auto-assign skips them.
	Hidden    bool
	Goal      Goal
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (e Envelope) Normalize() Envelope {
	e.Name = strings.TrimSpace(e.Name)
	if e.Goal.Type == GoalNone {
		e.Goal = Goal{}
	}
	return e
}

func (e Envelope) Validate() error {
	var errs []error
	errs = append(errs, validateName(e.Name), validatePosition(e.Position), e.Goal.Validate())
	if e.GroupID == "" {
		errs = append(errs, domain.ValidationError{Field: "group_id", Message: "is required"})
	}
	return errors.Join(errs...)
}

// Assignment is the money given to an envelope for a month, in minor
// units of the base currency. It may be negative.
type Assignment struct {
	HouseholdID domain.ID
	EnvelopeID  domain.ID
	Month       Month
	Amount      int64
	UpdatedAt   time.Time
}

func validateName(name string) error {
	if n := utf8.RuneCountInString(name); n == 0 || n > MaxNameLength {
		return domain.ValidationError{Field: "name", Message: "must be 1 to 100 characters"}
	}
	return nil
}

func validatePosition(p int) error {
	if p < 0 || p > MaxPosition {
		return domain.ValidationError{Field: "position", Message: "must be between 0 and 10000"}
	}
	return nil
}

func byName(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) }

// SortGroups orders groups by position, then name ignoring case, then id.
func SortGroups(gs []Group) {
	slices.SortFunc(gs, func(a, b Group) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), byName(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
}

// SortEnvelopes orders envelopes by position, then name ignoring case,
// then id. Grouping is the caller's.
func SortEnvelopes(es []Envelope) {
	slices.SortFunc(es, func(a, b Envelope) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), byName(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
}
