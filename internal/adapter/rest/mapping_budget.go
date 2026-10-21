package rest

import (
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/paramah/denar/internal/adapter/rest/api"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/usecase"
)

func apiEnvelopeGroup(g budget.Group) api.EnvelopeGroup {
	return api.EnvelopeGroup{Id: string(g.ID), Name: g.Name, Position: g.Position, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

func apiGoal(g budget.Goal, base string) nullable.Nullable[api.Goal] {
	if g.IsZero() {
		return nullable.NewNullNullable[api.Goal]()
	}
	out := api.Goal{Type: api.GoalType(g.Type), Amount: minor(g.Amount, base), TargetDate: nullable.NewNullNullable[openapi_types.Date]()}
	if !g.TargetDate.IsZero() {
		out.TargetDate.Set(apiDate(g.TargetDate))
	}
	return nullable.NewNullableWithValue(out)
}

func apiEnvelope(e budget.Envelope, base string) api.Envelope {
	return api.Envelope{Id: string(e.ID), GroupId: string(e.GroupID), Name: e.Name, Position: e.Position, Hidden: e.Hidden,
		Goal: apiGoal(e.Goal, base), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}

func goalInput(g *api.GoalInput) *usecase.GoalInput {
	if g == nil {
		return nil
	}
	return &usecase.GoalInput{Type: string(g.Type), Amount: moneyInput(g.Amount), TargetDate: fromAPIDate(g.TargetDate)}
}

func apiBudgetMonth(v usecase.BudgetMonthView) api.BudgetMonth {
	c := v.Currency
	m := func(amount int64) api.Money { return minor(amount, c) }
	groups := make([]api.BudgetGroup, len(v.Groups))
	for i, g := range v.Groups {
		envelopes := make([]api.BudgetEnvelope, len(g.Envelopes))
		for j, l := range g.Envelopes {
			status := nullable.NewNullNullable[api.GoalStatus]()
			if l.Goal != nil {
				status.Set(api.GoalStatus{Needed: m(l.Goal.Needed), Underfunded: m(l.Goal.Underfunded), Funded: l.Goal.Funded,
					Progress: l.Goal.Progress})
			}
			e := l.Envelope
			envelopes[j] = api.BudgetEnvelope{Id: string(e.ID), GroupId: string(e.GroupID), Name: e.Name, Position: e.Position,
				Hidden: e.Hidden, Goal: apiGoal(e.Goal, c), GoalStatus: status, Assigned: m(l.Assigned), Activity: m(l.Activity),
				CarriedOver: m(l.CarriedIn), Available: m(l.Available)}
		}
		groups[i] = api.BudgetGroup{Id: string(g.Group.ID), Name: g.Group.Name, Position: g.Group.Position, Assigned: m(g.Assigned),
			Activity: m(g.Activity), Available: m(g.Available), Envelopes: envelopes}
	}
	return api.BudgetMonth{
		Month: v.Month.String(), Start: apiDate(v.Start), End: apiDate(v.End), Currency: c,
		ReadyToAssign: m(v.ReadyToAssign), AssignedInFuture: m(v.AssignedInFuture), OverspentLastMonth: m(v.OverspentLastMonth),
		Income: m(v.Income), Uncategorized: m(v.Uncategorized), Transfers: m(v.Transfers),
		Assigned: m(v.Assigned), Activity: m(v.Activity), Available: m(v.Available), Underfunded: m(v.Underfunded),
		Incomplete: v.Incomplete, PendingTransactions: v.PendingTransactions, Groups: groups,
	}
}
