package usecase

import (
	"context"
	"strconv"

	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/event"
	"git.eiac.dev/tools/denar/internal/domain/household"
)

// defaultGoalDueDays is how many days ahead a goal is reminded of.
const defaultGoalDueDays = 7

func (d Deps) goalDueDays() int {
	if d.GoalDueDays > 0 {
		return d.GoalDueDays
	}
	return defaultGoalDueDays
}

// CheckBudgets is the job that appends the envelope events (FR-500…
// FR-502): envelope.overspent once per envelope and month, and
// envelope.goal_due once per envelope and target date. It returns how many
// events it stored.
type CheckBudgets struct{ d Deps }

func NewCheckBudgets(d Deps) *CheckBudgets { return &CheckBudgets{d: d} }

func (uc *CheckBudgets) Do(ctx context.Context) (int, error) {
	if uc.d.Events == nil {
		return 0, nil
	}
	ids, err := uc.d.Households.ListIDs(ctx)
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, id := range ids {
		err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			h, err := uc.d.Households.ByID(ctx, id)
			if found, err := ignoreNotFound(err); err != nil || !found {
				return err
			}
			n, err := uc.d.checkBudget(ctx, h)
			stored += n
			return err
		})
		if err != nil {
			return stored, err
		}
	}
	return stored, nil
}

func (d Deps) checkBudget(ctx context.Context, h household.Household) (int, error) {
	envelopes, err := d.Envelopes.ListByHousehold(ctx, h.ID)
	if err != nil || len(envelopes) == 0 {
		return 0, err
	}
	today := today(h, d.Clock.Now())
	m := budget.MonthOf(today, h.MonthStartDay)
	v, err := d.budgetMonth(ctx, h, m)
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, g := range v.Groups {
		for _, line := range g.Envelopes {
			e := line.Envelope
			facts := map[string]string{"envelope_id": string(e.ID), "envelope_name": e.Name, "month": m.String(),
				"available": decimal(line.Available, h.BaseCurrency), "currency": h.BaseCurrency, "wallet_id": ""}
			if line.Available < 0 {
				ok, err := d.emitEnvelope(ctx, h, e, event.EnvelopeOverspent, "overspent:"+string(e.ID)+":"+m.String(), facts)
				if err != nil {
					return stored, err
				}
				if ok {
					stored++
				}
			}
			goal := e.Goal
			if goal.Type != budget.GoalTargetByDate || goal.TargetDate.Before(today) ||
				goal.TargetDate.After(today.AddDays(d.goalDueDays())) || line.Available >= goal.Amount {
				continue
			}
			days := int(goal.TargetDate.Time().Sub(today.Time()).Hours() / 24)
			facts["target_date"] = goal.TargetDate.String()
			facts["goal_amount"] = decimal(goal.Amount, h.BaseCurrency)
			facts["missing"] = decimal(goal.Amount-max(line.Available, 0), h.BaseCurrency)
			facts["days_left"] = strconv.Itoa(days)
			ok, err := d.emitEnvelope(ctx, h, e, event.EnvelopeGoalDue, "goal_due:"+string(e.ID)+":"+goal.TargetDate.String(), facts)
			if err != nil {
				return stored, err
			}
			if ok {
				stored++
			}
		}
	}
	return stored, nil
}

func (d Deps) emitEnvelope(ctx context.Context, h household.Household, e budget.Envelope, typ event.Type, key string,
	facts map[string]string) (bool, error) {
	payload := make(map[string]string, len(facts))
	for k, v := range facts {
		payload[k] = v
	}
	return d.Events.Append(ctx, event.Event{ID: d.IDs.NewID(), HouseholdID: h.ID, Type: typ, ResourceType: event.ResourceEnvelope,
		ResourceID: e.ID, DedupeKey: key, Payload: payload, OccurredAt: d.Clock.Now()})
}
