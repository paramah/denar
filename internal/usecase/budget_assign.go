package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/money"
)

// How an assignment changed, for the audit log.
const (
	viaSet        = "set"
	viaMove       = "move"
	viaAutoAssign = "auto_assign"
)

// setAssignment stores the envelope's assignment for the month and records
// it. Call it in a transaction.
func (d Deps) setAssignment(ctx context.Context, s HouseholdScope, actor domain.Actor, envelopeID domain.ID, m budget.Month, amount int64, via string) error {
	old, err := d.Assignments.Get(ctx, s.Household.ID, envelopeID, m)
	if err != nil {
		return err
	}
	if old.Amount == amount {
		return nil
	}
	if err := d.Assignments.Set(ctx, budget.Assignment{HouseholdID: s.Household.ID, EnvelopeID: envelopeID, Month: m, Amount: amount,
		UpdatedAt: d.Clock.Now()}); err != nil {
		return plainNotFound(err)
	}
	base := s.Household.BaseCurrency
	changes := audit.Changes{}.Set("month", m.String()).Set("via", via)
	changes["assigned"] = audit.Change{From: formatMinor(old.Amount, base), To: formatMinor(amount, base)}
	return d.record(ctx, s.Household.ID, actor, "assignment.set", audit.ResourceAssignment, envelopeID, changes)
}

// addAssignment adds delta to the envelope's assignment for the month.
func (d Deps) addAssignment(ctx context.Context, s HouseholdScope, actor domain.Actor, envelopeID domain.ID, m budget.Month, delta int64, via string) error {
	old, err := d.Assignments.Get(ctx, s.Household.ID, envelopeID, m)
	if err != nil {
		return err
	}
	sum, ok := money.AddInt64(old.Amount, delta)
	if !ok {
		return domain.ValidationError{Field: "amount.amount", Message: "makes the assignment too large"}
	}
	return d.setAssignment(ctx, s, actor, envelopeID, m, sum, via)
}

// envelopeOf returns an envelope of the household; another one is
// domain.ErrNotFound.
func (d Deps) envelopeOf(ctx context.Context, householdID, id domain.ID) (budget.Envelope, error) {
	e, err := d.Envelopes.ByID(ctx, householdID, id)
	return e, plainNotFound(err)
}

type SetAssignmentInput struct {
	Month  string
	Amount MoneyInput // base currency; zero removes the assignment
}

// SetAssignment sets what an envelope is given for a month (FR-211) and
// returns the month.
type SetAssignment struct{ d Deps }

func NewSetAssignment(d Deps) *SetAssignment { return &SetAssignment{d: d} }

func (uc *SetAssignment) Do(ctx context.Context, p Principal, householdID, envelopeID domain.ID, in SetAssignmentInput) (BudgetMonthView, error) {
	var view BudgetMonthView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		m, monthErr := uc.d.parseMonth(s.Household, in.Month)
		amount, amountErr := parseBaseMoney("amount", in.Amount, s.Household.BaseCurrency)
		if err := errors.Join(monthErr, amountErr); err != nil {
			return err
		}
		if _, err := uc.d.envelopeOf(ctx, householdID, envelopeID); err != nil {
			return err
		}
		if err := uc.d.setAssignment(ctx, s, p.Actor(), envelopeID, m, amount.Amount, viaSet); err != nil {
			return err
		}
		view, err = uc.d.budgetMonth(ctx, s.Household, m)
		return err
	})
	return view, err
}

type MoveBetweenEnvelopesInput struct {
	Month          string
	FromEnvelopeID string
	ToEnvelopeID   string
	Amount         MoneyInput // base currency, above zero
}

// MoveBetweenEnvelopes moves assigned money from one envelope to another in
// a month: two assignments in one database transaction (FR-212). The
// source may go below zero, as in YNAB (AD-310).
type MoveBetweenEnvelopes struct{ d Deps }

func NewMoveBetweenEnvelopes(d Deps) *MoveBetweenEnvelopes { return &MoveBetweenEnvelopes{d: d} }

func (uc *MoveBetweenEnvelopes) Do(ctx context.Context, p Principal, householdID domain.ID, in MoveBetweenEnvelopesInput) (BudgetMonthView, error) {
	var view BudgetMonthView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		from, to := domain.ID(strings.TrimSpace(in.FromEnvelopeID)), domain.ID(strings.TrimSpace(in.ToEnvelopeID))
		m, monthErr := uc.d.parseMonth(s.Household, in.Month)
		amount, amountErr := parseBaseMoney("amount", in.Amount, s.Household.BaseCurrency)
		if amountErr == nil && amount.Sign() <= 0 {
			amountErr = domain.ValidationError{Field: "amount.amount", Message: "must be greater than zero"}
		}
		var sameErr error
		if from == to {
			sameErr = domain.ValidationError{Field: "to_envelope_id", Message: "must differ from from_envelope_id"}
		}
		if err := errors.Join(monthErr, amountErr, sameErr); err != nil {
			return err
		}
		for _, id := range []domain.ID{from, to} {
			if _, err := uc.d.envelopeOf(ctx, householdID, id); err != nil {
				return err
			}
		}
		if err := uc.d.addAssignment(ctx, s, p.Actor(), from, m, -amount.Amount, viaMove); err != nil {
			return err
		}
		if err := uc.d.addAssignment(ctx, s, p.Actor(), to, m, amount.Amount, viaMove); err != nil {
			return err
		}
		view, err = uc.d.budgetMonth(ctx, s.Household, m)
		return err
	})
	return view, err
}

// AutoAssignResult is what auto-assign gave and the month after it.
type AutoAssignResult struct {
	Assigned []budget.Allocation
	Month    BudgetMonthView
}

// AutoAssign funds the underfunded goals of the envelopes that are not
// hidden from Ready to Assign, in the order of groups and envelopes
// (FR-213).
type AutoAssign struct{ d Deps }

func NewAutoAssign(d Deps) *AutoAssign { return &AutoAssign{d: d} }

func (uc *AutoAssign) Do(ctx context.Context, p Principal, householdID domain.ID, month string) (AutoAssignResult, error) {
	var out AutoAssignResult
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.budgetScope(ctx, p, householdID, bot.ScopeBudgetWrite)
		if err != nil {
			return err
		}
		m, err := budget.ParseMonth("month", month)
		if err != nil {
			return err
		}
		before, err := uc.d.budgetMonth(ctx, s.Household, m)
		if err != nil {
			return err
		}
		var candidates []budget.Candidate
		for _, g := range before.Groups {
			for _, l := range g.Envelopes {
				if !l.Envelope.Hidden && l.Goal != nil && l.Goal.Underfunded > 0 {
					candidates = append(candidates, budget.Candidate{EnvelopeID: l.Envelope.ID, Underfunded: l.Goal.Underfunded})
				}
			}
		}
		out.Assigned = budget.AutoAssign(candidates, before.ReadyToAssign)
		for _, a := range out.Assigned {
			if err := uc.d.addAssignment(ctx, s, p.Actor(), a.EnvelopeID, m, a.Amount, viaAutoAssign); err != nil {
				return err
			}
		}
		out.Month, err = uc.d.budgetMonth(ctx, s.Household, m)
		return err
	})
	return out, err
}
