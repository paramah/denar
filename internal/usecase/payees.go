package usecase

import (
	"context"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/payee"
)

// ListPayees returns the household's payees by name.
type ListPayees struct{ d Deps }

func NewListPayees(d Deps) *ListPayees { return &ListPayees{d: d} }

func (uc *ListPayees) Do(ctx context.Context, p Principal, householdID domain.ID) ([]payee.Payee, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return nil, err
	}
	if err := requireScope(s, bot.ScopeTransactionsRead); err != nil {
		return nil, err
	}
	return uc.d.Payees.ListByHousehold(ctx, householdID)
}

type CreatePayeeInput struct {
	Name              string
	Patterns          []string
	DefaultEnvelopeID string
}

func payeeChanges(p payee.Payee) audit.Changes {
	return audit.Changes{}.Set("name", p.Name).Set("patterns", strings.Join(p.Patterns, " | ")).
		Set("default_envelope_id", string(p.DefaultEnvelopeID))
}

// CreatePayee adds a payee (FR-140).
type CreatePayee struct{ d Deps }

func NewCreatePayee(d Deps) *CreatePayee { return &CreatePayee{d: d} }

func (uc *CreatePayee) Do(ctx context.Context, p Principal, householdID domain.ID, in CreatePayeeInput) (payee.Payee, error) {
	var out payee.Payee
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		now := uc.d.Clock.Now()
		pe := payee.Payee{ID: uc.d.IDs.NewID(), HouseholdID: householdID, Name: in.Name, Patterns: in.Patterns,
			DefaultEnvelopeID: domain.ID(strings.TrimSpace(in.DefaultEnvelopeID)), CreatedAt: now, UpdatedAt: now}.Normalize()
		if err := pe.Validate(); err != nil {
			return err
		}
		if err := uc.d.Payees.Create(ctx, pe); err != nil {
			return err
		}
		out = pe
		return uc.d.record(ctx, householdID, p.Actor(), "payee.created", audit.ResourcePayee, pe.ID, payeeChanges(pe))
	})
	return out, err
}

type UpdatePayeeInput struct {
	Name              domain.Optional[string]
	Patterns          domain.Optional[[]string]
	DefaultEnvelopeID domain.Optional[string] // "" removes it
}

// UpdatePayee changes a payee.
type UpdatePayee struct{ d Deps }

func NewUpdatePayee(d Deps) *UpdatePayee { return &UpdatePayee{d: d} }

func (uc *UpdatePayee) Do(ctx context.Context, p Principal, householdID, id domain.ID, in UpdatePayeeInput) (payee.Payee, error) {
	var out payee.Payee
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		old, err := uc.d.Payees.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		pe := old
		if in.Name.Set {
			pe.Name = in.Name.Value
		}
		if in.Patterns.Set {
			pe.Patterns = in.Patterns.Value
		}
		if in.DefaultEnvelopeID.Set {
			pe.DefaultEnvelopeID = domain.ID(strings.TrimSpace(in.DefaultEnvelopeID.Value))
		}
		pe = pe.Normalize()
		if err := pe.Validate(); err != nil {
			return err
		}
		out = pe
		before, after := payeeChanges(old), payeeChanges(pe)
		changes := audit.Changes{}
		for field := range after {
			changes.Diff(field, before[field].To, after[field].To)
		}
		if len(changes) == 0 {
			return nil
		}
		pe.UpdatedAt = uc.d.Clock.Now()
		out = pe
		if err := uc.d.Payees.Update(ctx, pe); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "payee.updated", audit.ResourcePayee, pe.ID, changes)
	})
	return out, err
}

// DeletePayee removes a payee; its transactions keep no payee.
type DeletePayee struct{ d Deps }

func NewDeletePayee(d Deps) *DeletePayee { return &DeletePayee{d: d} }

func (uc *DeletePayee) Do(ctx context.Context, p Principal, householdID, id domain.ID) error {
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeTransactionsWrite); err != nil {
			return err
		}
		old, err := uc.d.Payees.ByID(ctx, householdID, id)
		if err != nil {
			return plainNotFound(err)
		}
		if err := uc.d.Payees.Delete(ctx, householdID, id); err != nil {
			return err
		}
		return uc.d.record(ctx, householdID, p.Actor(), "payee.deleted", audit.ResourcePayee, id, audit.Changes{"name": {From: old.Name}})
	})
}
