package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/payee"
)

// PayeeRepository keeps payees in memory. Deleting one takes it off the
// transactions of the TransactionRepository it is linked to, as the
// database's ON DELETE SET NULL does.
type PayeeRepository struct {
	store        *Store
	payees       map[domain.ID]payee.Payee
	transactions *TransactionRepository
}

func NewPayeeRepository(s *Store) *PayeeRepository {
	r := &PayeeRepository{store: s, payees: map[domain.ID]payee.Payee{}}
	s.Register(r)
	return r
}

func (r *PayeeRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.payees)
	return func() { r.payees = saved }
}

func (r *PayeeRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.payees, func(_ domain.ID, p payee.Payee) bool { return p.HouseholdID == id })
}

func clonePayee(p payee.Payee) payee.Payee {
	p.Patterns = slices.Clone(p.Patterns)
	if p.Patterns == nil {
		p.Patterns = []string{}
	}
	return p
}

func (r *PayeeRepository) checkName(p payee.Payee) error {
	for _, other := range r.payees {
		if other.ID != p.ID && other.HouseholdID == p.HouseholdID && strings.EqualFold(other.Name, p.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *PayeeRepository) Create(ctx context.Context, p payee.Payee) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.payees[p.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkName(p); err == nil {
			r.payees[p.ID] = clonePayee(p)
		}
	})
	return err
}

func (r *PayeeRepository) Update(ctx context.Context, p payee.Payee) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.payees[p.ID]
		if !ok || old.HouseholdID != p.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkName(p); err != nil {
			return
		}
		old.Name, old.Patterns, old.DefaultEnvelopeID, old.UpdatedAt = p.Name, p.Patterns, p.DefaultEnvelopeID, p.UpdatedAt
		r.payees[p.ID] = clonePayee(old)
	})
	return err
}

func (r *PayeeRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if p, ok := r.payees[id]; !ok || p.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		delete(r.payees, id)
		if r.transactions != nil {
			r.transactions.clearPayee(id)
		}
	})
	return err
}

func (r *PayeeRepository) ByID(ctx context.Context, householdID, id domain.ID) (p payee.Payee, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if p, ok = r.payees[id]; !ok || p.HouseholdID != householdID {
			p, err = payee.Payee{}, domain.ErrNotFound
		}
	})
	return clonePayee(p), err
}

func (r *PayeeRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]payee.Payee, error) {
	out := []payee.Payee{}
	r.store.Do(ctx, func() {
		for _, p := range r.payees {
			if p.HouseholdID == householdID {
				out = append(out, clonePayee(p))
			}
		}
	})
	slices.SortFunc(out, func(a, b payee.Payee) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

// name returns the payee's name; call it while the store is held.
func (r *PayeeRepository) name(id domain.ID) string { return r.payees[id].Name }
