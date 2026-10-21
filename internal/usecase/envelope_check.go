package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/money"
	"github.com/paramah/denar/internal/domain/transaction"
)

// checkEnvelope checks that id, when set, is an envelope of the household;
// a hidden envelope is fine (FR-220). Without an envelope repository
// nothing is checked. Call it inside the transaction of the write.
func (d Deps) checkEnvelope(ctx context.Context, householdID domain.ID, field string, id domain.ID) error {
	if id == "" || d.Envelopes == nil {
		return nil
	}
	_, err := d.Envelopes.ByID(ctx, householdID, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ValidationError{Field: field, Message: "is not an envelope of this household"}
	}
	return err
}

// checkTxEnvelopes checks the envelope of a transaction and of its split
// lines.
func (d Deps) checkTxEnvelopes(ctx context.Context, t transaction.Transaction) error {
	errs := []error{d.checkEnvelope(ctx, t.HouseholdID, "envelope_id", t.EnvelopeID)}
	for i, s := range t.Splits {
		errs = append(errs, d.checkEnvelope(ctx, t.HouseholdID, fmt.Sprintf("splits[%d].envelope_id", i), s.EnvelopeID))
	}
	return errors.Join(errs...)
}

// envelopeExists reports whether the household has the envelope; without
// an envelope repository every envelope exists.
func (d Deps) envelopeExists(ctx context.Context, householdID, id domain.ID) (bool, error) {
	if d.Envelopes == nil {
		return true, nil
	}
	_, err := d.Envelopes.ByID(ctx, householdID, id)
	return ignoreNotFound(err)
}

// parseBaseMoney reads an amount that must be in the household's base
// currency, as every amount of the budget is.
func parseBaseMoney(field string, in MoneyInput, base string) (money.Money, error) {
	if in.Currency != base {
		return money.Money{}, domain.ValidationError{Field: field + ".currency", Message: "must be " + base + ", the base currency of the household"}
	}
	return money.Parse(field+".amount", in.Amount, base)
}
