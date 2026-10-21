package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/budget"
)

// budgetScope opens the household for a budget operation with the scope.
func (d Deps) budgetScope(ctx context.Context, p Principal, householdID domain.ID, scope bot.Scope) (HouseholdScope, error) {
	s, err := d.access().household(ctx, p, householdID)
	if err != nil {
		return HouseholdScope{}, err
	}
	return s, requireScope(s, scope)
}

// ListEnvelopeGroups returns the household's envelope groups in order.
type ListEnvelopeGroups struct{ d Deps }

func NewListEnvelopeGroups(d Deps) *ListEnvelopeGroups { return &ListEnvelopeGroups{d: d} }

func (uc *ListEnvelopeGroups) Do(ctx context.Context, p Principal, householdID domain.ID) ([]budget.Group, error) {
	if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetRead); err != nil {
		return nil, err
	}
	return uc.d.EnvelopeGroups.ListByHousehold(ctx, householdID)
}

type CreateEnvelopeGroupInput struct {
	Name     string
	Position int
}

func groupChanges(g budget.Group) audit.Changes {
	return audit.Changes{}.Set("name", g.Name).Set("position", g.Position)
}

// CreateEnvelopeGroup adds an envelope group (FR-210).
type CreateEnvelopeGroup struct{ d Deps }

func NewCreateEnvelopeGroup(d Deps) *CreateEnvelopeGroup { return &CreateEnvelopeGroup{d: d} }

func (uc *CreateEnvelopeGroup) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateEnvelopeGroupInput) (budget.Group, error) {
	var out budget.Group
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite); err != nil {
			return err
		}
		now := uc.d.Clock.Now()
		g := budget.Group{ID: uc.d.IDs.NewID(), HouseholdID: householdID, Name: in.Name, Position: in.Position,
			CreatedAt: now, UpdatedAt: now}.Normalize()
		if err := g.Validate(); err != nil {
			return err
		}
		if err := uc.d.EnvelopeGroups.Create(ctx, g); err != nil {
			return err
		}
		out = g
		return uc.d.record(ctx, householdID, p.Actor(), "envelope_group.created", audit.ResourceEnvelopeGroup, g.ID, groupChanges(g))
	})
	return out, err
}

type UpdateEnvelopeGroupInput struct {
	Name     domain.Optional[string]
	Position domain.Optional[int]
}

// UpdateEnvelopeGroup renames or moves a group.
type UpdateEnvelopeGroup struct{ d Deps }

func NewUpdateEnvelopeGroup(d Deps) *UpdateEnvelopeGroup { return &UpdateEnvelopeGroup{d: d} }

func (uc *UpdateEnvelopeGroup) Do(ctx context.Context, p Principal, householdID, id domain.ID, in UpdateEnvelopeGroupInput) (budget.Group, error) {
	var out budget.Group
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite); err != nil {
			return err
		}
		old, err := uc.d.EnvelopeGroups.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		g := old
		if in.Name.Set {
			g.Name = in.Name.Value
		}
		if in.Position.Set {
			g.Position = in.Position.Value
		}
		g = g.Normalize()
		if err := g.Validate(); err != nil {
			return err
		}
		out = g
		changes := diffChanges(groupChanges(old), groupChanges(g))
		if len(changes) == 0 {
			return nil
		}
		g.UpdatedAt = uc.d.Clock.Now()
		out = g
		if err := uc.d.EnvelopeGroups.Update(ctx, g); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "envelope_group.updated", audit.ResourceEnvelopeGroup, g.ID, changes)
	})
	return out, err
}

// diffChanges keeps the fields whose value differs.
func diffChanges(before, after audit.Changes) audit.Changes {
	changes := audit.Changes{}
	for field := range after {
		changes.Diff(field, before[field].To, after[field].To)
	}
	return changes
}

// DeleteEnvelopeGroup removes an empty group.
type DeleteEnvelopeGroup struct{ d Deps }

func NewDeleteEnvelopeGroup(d Deps) *DeleteEnvelopeGroup { return &DeleteEnvelopeGroup{d: d} }

func (uc *DeleteEnvelopeGroup) Do(ctx context.Context, p Principal, householdID, id domain.ID) error {
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite); err != nil {
			return err
		}
		old, err := uc.d.EnvelopeGroups.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		if err := uc.d.EnvelopeGroups.Delete(ctx, householdID, id); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "envelope_group.deleted", audit.ResourceEnvelopeGroup, id,
			audit.Changes{"name": {From: old.Name}})
	})
}

type ListEnvelopesInput struct {
	IncludeHidden bool
}

// ListEnvelopes returns the household's envelopes in order.
type ListEnvelopes struct{ d Deps }

func NewListEnvelopes(d Deps) *ListEnvelopes { return &ListEnvelopes{d: d} }

func (uc *ListEnvelopes) Do(ctx context.Context, p Principal, householdID domain.ID, in ListEnvelopesInput) ([]budget.Envelope, error) {
	if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetRead); err != nil {
		return nil, err
	}
	all, err := uc.d.Envelopes.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := all[:0:0]
	for _, e := range all {
		if in.IncludeHidden || !e.Hidden {
			out = append(out, e)
		}
	}
	return out, nil
}

// GetEnvelope returns one envelope.
type GetEnvelope struct{ d Deps }

func NewGetEnvelope(d Deps) *GetEnvelope { return &GetEnvelope{d: d} }

func (uc *GetEnvelope) Do(ctx context.Context, p Principal, householdID, id domain.ID) (budget.Envelope, error) {
	if _, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetRead); err != nil {
		return budget.Envelope{}, err
	}
	e, err := uc.d.Envelopes.ByID(ctx, householdID, id)
	return e, plainNotFound(err)
}

// GoalInput is a goal as clients write it; the amount is in the base
// currency.
type GoalInput struct {
	Type       string
	Amount     MoneyInput
	TargetDate domain.Date
}

func parseGoal(in *GoalInput, base string) (budget.Goal, error) {
	if in == nil {
		return budget.Goal{}, nil
	}
	g := budget.Goal{Type: budget.GoalType(in.Type), TargetDate: in.TargetDate}
	if g.Type == budget.GoalNone {
		return budget.Goal{}, domain.ValidationError{Field: "goal.type", Message: "must be monthly or target_by_date"}
	}
	m, err := parseBaseMoney("goal.amount", in.Amount, base)
	if err != nil {
		return budget.Goal{}, err
	}
	g.Amount = m.Amount
	return g, g.Validate()
}

func goalText(g budget.Goal, base string) string {
	switch g.Type {
	case budget.GoalMonthly:
		return "monthly " + formatMinor(g.Amount, base)
	case budget.GoalTargetByDate:
		return "target_by_date " + formatMinor(g.Amount, base) + " by " + g.TargetDate.String()
	}
	return ""
}

func envelopeChanges(e budget.Envelope, base string) audit.Changes {
	return audit.Changes{}.Set("name", e.Name).Set("group_id", string(e.GroupID)).Set("position", e.Position).
		Set("hidden", e.Hidden).Set("goal", goalText(e.Goal, base))
}

// groupError turns a missing group into a validation error of the field.
func groupError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ValidationError{Field: "group_id", Message: "is not an envelope group of this household"}
	}
	return err
}

type CreateEnvelopeInput struct {
	GroupID  string
	Name     string
	Position int
	Hidden   bool
	Goal     *GoalInput
}

// CreateEnvelope adds an envelope to a group (FR-210).
type CreateEnvelope struct{ d Deps }

func NewCreateEnvelope(d Deps) *CreateEnvelope { return &CreateEnvelope{d: d} }

func (uc *CreateEnvelope) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateEnvelopeInput) (budget.Envelope, error) {
	var out budget.Envelope
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		base := s.Household.BaseCurrency
		goal, goalErr := parseGoal(in.Goal, base)
		now := uc.d.Clock.Now()
		e := budget.Envelope{ID: uc.d.IDs.NewID(), HouseholdID: householdID, GroupID: domain.ID(strings.TrimSpace(in.GroupID)),
			Name: in.Name, Position: in.Position, Hidden: in.Hidden, Goal: goal, CreatedAt: now, UpdatedAt: now}.Normalize()
		if err := errors.Join(goalErr, e.Validate()); err != nil {
			return err
		}
		if err := uc.d.Envelopes.Create(ctx, e); err != nil {
			return groupError(err)
		}
		out = e
		return uc.d.record(ctx, householdID, p.Actor(), "envelope.created", audit.ResourceEnvelope, e.ID, envelopeChanges(e, base))
	})
	return out, err
}

type UpdateEnvelopeInput struct {
	GroupID  domain.Optional[string]
	Name     domain.Optional[string]
	Position domain.Optional[int]
	Hidden   domain.Optional[bool]
	Goal     domain.Optional[*GoalInput] // nil removes the goal
}

// UpdateEnvelope changes an envelope.
type UpdateEnvelope struct{ d Deps }

func NewUpdateEnvelope(d Deps) *UpdateEnvelope { return &UpdateEnvelope{d: d} }

func (uc *UpdateEnvelope) Do(ctx context.Context, p Principal, householdID, id domain.ID, in UpdateEnvelopeInput) (budget.Envelope, error) {
	var out budget.Envelope
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		base := s.Household.BaseCurrency
		old, err := uc.d.Envelopes.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		e := old
		var goalErr error
		if in.GroupID.Set {
			e.GroupID = domain.ID(strings.TrimSpace(in.GroupID.Value))
		}
		if in.Name.Set {
			e.Name = in.Name.Value
		}
		if in.Position.Set {
			e.Position = in.Position.Value
		}
		if in.Hidden.Set {
			e.Hidden = in.Hidden.Value
		}
		if in.Goal.Set {
			e.Goal, goalErr = parseGoal(in.Goal.Value, base)
		}
		e = e.Normalize()
		if err := errors.Join(goalErr, e.Validate()); err != nil {
			return err
		}
		out = e
		changes := diffChanges(envelopeChanges(old, base), envelopeChanges(e, base))
		if len(changes) == 0 {
			return nil
		}
		e.UpdatedAt = uc.d.Clock.Now()
		out = e
		if err := uc.d.Envelopes.Update(ctx, e); err != nil {
			return groupError(err)
		}
		return uc.d.record(ctx, householdID, p.Actor(), "envelope.updated", audit.ResourceEnvelope, e.ID, changes)
	})
	return out, err
}

// DeleteEnvelope removes an envelope no transaction uses, with its
// assignments; payees that default to it default to none (AD-304).
type DeleteEnvelope struct{ d Deps }

func NewDeleteEnvelope(d Deps) *DeleteEnvelope { return &DeleteEnvelope{d: d} }

func (uc *DeleteEnvelope) Do(ctx context.Context, p Principal, householdID, id domain.ID) error {
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		old, err := uc.d.Envelopes.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		used, err := uc.d.Transactions.CountByEnvelope(ctx, householdID, id)
		if err != nil {
			return err
		}
		if used > 0 {
			return domain.ConflictError{Reason: domain.ConflictEnvelopeInUse}
		}
		payees, err := uc.d.Payees.ListByHousehold(ctx, householdID)
		if err != nil {
			return err
		}
		for _, pe := range payees {
			if pe.DefaultEnvelopeID != id {
				continue
			}
			pe.DefaultEnvelopeID, pe.UpdatedAt = "", uc.d.Clock.Now()
			if err := uc.d.Payees.Update(ctx, pe); err != nil {
				return err
			}
			if err := uc.d.record(ctx, householdID, p.Actor(), "payee.updated", audit.ResourcePayee, pe.ID,
				audit.Changes{"default_envelope_id": {From: string(id)}}); err != nil {
				return err
			}
		}
		assignments, err := uc.d.Assignments.ListByHousehold(ctx, householdID)
		if err != nil {
			return err
		}
		var assigned []string
		for _, a := range assignments {
			if a.EnvelopeID == id {
				assigned = append(assigned, fmt.Sprintf("%s:%s", a.Month, formatMinor(a.Amount, s.Household.BaseCurrency)))
			}
		}
		if err := uc.d.Envelopes.Delete(ctx, householdID, id); err != nil {
			return err
		}
		changes := audit.Changes{"name": {From: old.Name}, "group_id": {From: string(old.GroupID)}}
		if len(assigned) > 0 {
			changes["assignments"] = audit.Change{From: strings.Join(assigned, ", ")}
		}
		return uc.d.record(ctx, householdID, p.Actor(), "envelope.deleted", audit.ResourceEnvelope, id, changes)
	})
}
