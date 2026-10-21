package budget

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/paramah/denar/internal/domain"
)

func fields(err error) []string {
	var out []string
	var walk func(error)
	walk = func(err error) {
		switch e := err.(type) {
		case domain.ValidationError:
			out = append(out, e.Field)
		case interface{ Unwrap() []error }:
			for _, c := range e.Unwrap() {
				walk(c)
			}
		}
	}
	walk(err)
	slices.Sort(out)
	return out
}

func TestGroupValidate(t *testing.T) {
	g := Group{Name: "  Rachunki "}.Normalize()
	if g.Name != "Rachunki" || g.Validate() != nil {
		t.Errorf("group = %+v, %v", g, g.Validate())
	}
	if got := fields(Group{Name: " ", Position: -1}.Normalize().Validate()); !slices.Equal(got, []string{"name", "position"}) {
		t.Errorf("fields = %v", got)
	}
	if got := fields(Group{Name: strings.Repeat("ż", 101), Position: 10001}.Validate()); !slices.Equal(got, []string{"name", "position"}) {
		t.Errorf("fields = %v", got)
	}
}

func TestEnvelopeValidate(t *testing.T) {
	ok := []Envelope{
		{Name: "Jedzenie", GroupID: "g1"},
		{Name: "Czynsz", GroupID: "g1", Goal: Goal{Type: GoalMonthly, Amount: 250000}},
		{Name: "Wakacje", GroupID: "g1", Goal: Goal{Type: GoalTargetByDate, Amount: 1, TargetDate: day("2027-06-01")}},
		{Name: "Max", GroupID: "g1", Position: 10000, Goal: Goal{Type: GoalMonthly, Amount: MaxGoalAmount}},
	}
	for _, e := range ok {
		if err := e.Normalize().Validate(); err != nil {
			t.Errorf("%+v: %v", e, err)
		}
	}
	tests := []struct {
		e    Envelope
		want []string
	}{
		{Envelope{}, []string{"group_id", "name"}},
		{Envelope{Name: "x", GroupID: "g", Goal: Goal{Type: "weekly"}}, []string{"goal.type"}},
		{Envelope{Name: "x", GroupID: "g", Goal: Goal{Type: GoalMonthly}}, []string{"goal.amount.amount"}},
		{Envelope{Name: "x", GroupID: "g", Goal: Goal{Type: GoalMonthly, Amount: MaxGoalAmount + 1}}, []string{"goal.amount.amount"}},
		{Envelope{Name: "x", GroupID: "g", Goal: Goal{Type: GoalMonthly, Amount: 1, TargetDate: day("2027-01-01")}}, []string{"goal.target_date"}},
		{Envelope{Name: "x", GroupID: "g", Goal: Goal{Type: GoalTargetByDate, Amount: -5}}, []string{"goal.amount.amount", "goal.target_date"}},
	}
	for _, tt := range tests {
		err := tt.e.Normalize().Validate()
		if !errors.Is(err, domain.ErrInvalid) || !slices.Equal(fields(err), tt.want) {
			t.Errorf("%+v: fields = %v, want %v", tt.e, fields(err), tt.want)
		}
	}
	// No goal drops leftovers.
	if e := (Envelope{Goal: Goal{Amount: 5}}).Normalize(); e.Goal != (Goal{}) {
		t.Errorf("goal = %+v", e.Goal)
	}
}

func TestSort(t *testing.T) {
	gs := []Group{{ID: "3", Name: "b", Position: 1}, {ID: "2", Name: "B", Position: 0}, {ID: "1", Name: "a", Position: 0}, {ID: "0", Name: "b", Position: 0}}
	SortGroups(gs)
	var ids []domain.ID
	for _, g := range gs {
		ids = append(ids, g.ID)
	}
	if !slices.Equal(ids, []domain.ID{"1", "0", "2", "3"}) {
		t.Errorf("groups = %v", ids)
	}
	es := []Envelope{{ID: "2", Name: "z"}, {ID: "1", Name: "Z", Position: -1}, {ID: "3", Name: "a"}}
	SortEnvelopes(es)
	if es[0].ID != "1" || es[1].ID != "3" || es[2].ID != "2" {
		t.Errorf("envelopes = %+v", es)
	}
}
