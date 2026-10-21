package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
)

// EnvelopeGroupRepository keeps envelope groups in memory. A group with
// envelopes of the EnvelopeRepository linked to it cannot be deleted.
type EnvelopeGroupRepository struct {
	store     *Store
	groups    map[domain.ID]budget.Group
	envelopes *EnvelopeRepository
}

func NewEnvelopeGroupRepository(s *Store) *EnvelopeGroupRepository {
	r := &EnvelopeGroupRepository{store: s, groups: map[domain.ID]budget.Group{}}
	s.Register(r)
	return r
}

func (r *EnvelopeGroupRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.groups)
	return func() { r.groups = saved }
}

func (r *EnvelopeGroupRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.groups, func(_ domain.ID, g budget.Group) bool { return g.HouseholdID == id })
}

func (r *EnvelopeGroupRepository) checkName(g budget.Group) error {
	for _, o := range r.groups {
		if o.ID != g.ID && o.HouseholdID == g.HouseholdID && strings.EqualFold(o.Name, g.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *EnvelopeGroupRepository) Create(ctx context.Context, g budget.Group) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.groups[g.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkName(g); err == nil {
			r.groups[g.ID] = g
		}
	})
	return err
}

func (r *EnvelopeGroupRepository) Update(ctx context.Context, g budget.Group) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.groups[g.ID]
		if !ok || old.HouseholdID != g.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkName(g); err != nil {
			return
		}
		old.Name, old.Position, old.UpdatedAt = g.Name, g.Position, g.UpdatedAt
		r.groups[g.ID] = old
	})
	return err
}

func (r *EnvelopeGroupRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if g, ok := r.groups[id]; !ok || g.HouseholdID != householdID {
			err = domain.ErrNotFound
			return
		}
		if r.envelopes != nil && r.envelopes.inGroup(id) {
			err = domain.ConflictError{Reason: domain.ConflictGroupNotEmpty}
			return
		}
		delete(r.groups, id)
	})
	return err
}

func (r *EnvelopeGroupRepository) ByID(ctx context.Context, householdID, id domain.ID) (g budget.Group, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if g, ok = r.groups[id]; !ok || g.HouseholdID != householdID {
			g, err = budget.Group{}, domain.ErrNotFound
		}
	})
	return g, err
}

func (r *EnvelopeGroupRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Group, error) {
	out := []budget.Group{}
	r.store.Do(ctx, func() {
		for _, g := range r.groups {
			if g.HouseholdID == householdID {
				out = append(out, g)
			}
		}
	})
	budget.SortGroups(out)
	return out, nil
}

// has reports whether the household has the group; call it while the
// store is held.
func (r *EnvelopeGroupRepository) has(householdID, id domain.ID) bool {
	g, ok := r.groups[id]
	return ok && g.HouseholdID == householdID
}

// EnvelopeRepository keeps envelopes in memory, linked to their groups;
// deleting one drops its assignments in the AssignmentRepository linked to
// it.
type EnvelopeRepository struct {
	store       *Store
	envelopes   map[domain.ID]budget.Envelope
	groups      *EnvelopeGroupRepository
	assignments *AssignmentRepository
}

func NewEnvelopeRepository(s *Store, groups *EnvelopeGroupRepository) *EnvelopeRepository {
	r := &EnvelopeRepository{store: s, envelopes: map[domain.ID]budget.Envelope{}, groups: groups}
	groups.envelopes = r
	s.Register(r)
	return r
}

func (r *EnvelopeRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.envelopes)
	return func() { r.envelopes = saved }
}

func (r *EnvelopeRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.envelopes, func(_ domain.ID, e budget.Envelope) bool { return e.HouseholdID == id })
}

func (r *EnvelopeRepository) inGroup(groupID domain.ID) bool {
	for _, e := range r.envelopes {
		if e.GroupID == groupID {
			return true
		}
	}
	return false
}

func (r *EnvelopeRepository) has(householdID, id domain.ID) bool {
	e, ok := r.envelopes[id]
	return ok && e.HouseholdID == householdID
}

func (r *EnvelopeRepository) check(e budget.Envelope) error {
	if !r.groups.has(e.HouseholdID, e.GroupID) {
		return domain.ErrNotFound
	}
	for _, o := range r.envelopes {
		if o.ID != e.ID && o.HouseholdID == e.HouseholdID && strings.EqualFold(o.Name, e.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *EnvelopeRepository) Create(ctx context.Context, e budget.Envelope) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.envelopes[e.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.check(e); err == nil {
			r.envelopes[e.ID] = e
		}
	})
	return err
}

func (r *EnvelopeRepository) Update(ctx context.Context, e budget.Envelope) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.envelopes[e.ID]
		if !ok || old.HouseholdID != e.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.check(e); err != nil {
			return
		}
		old.GroupID, old.Name, old.Position, old.Hidden, old.Goal, old.UpdatedAt = e.GroupID, e.Name, e.Position, e.Hidden, e.Goal, e.UpdatedAt
		r.envelopes[e.ID] = old
	})
	return err
}

func (r *EnvelopeRepository) Delete(ctx context.Context, householdID, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if !r.has(householdID, id) {
			err = domain.ErrNotFound
			return
		}
		delete(r.envelopes, id)
		if r.assignments != nil {
			r.assignments.dropEnvelope(id)
		}
	})
	return err
}

func (r *EnvelopeRepository) ByID(ctx context.Context, householdID, id domain.ID) (e budget.Envelope, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if e, ok = r.envelopes[id]; !ok || e.HouseholdID != householdID {
			e, err = budget.Envelope{}, domain.ErrNotFound
		}
	})
	return e, err
}

func (r *EnvelopeRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Envelope, error) {
	out := []budget.Envelope{}
	r.store.Do(ctx, func() {
		for _, e := range r.envelopes {
			if e.HouseholdID == householdID {
				out = append(out, e)
			}
		}
	})
	budget.SortEnvelopes(out)
	return out, nil
}

type assignmentKey struct {
	envelope domain.ID
	month    budget.Month
}

// AssignmentRepository keeps assignments in memory, for envelopes of the
// EnvelopeRepository linked to it.
type AssignmentRepository struct {
	store       *Store
	assignments map[assignmentKey]budget.Assignment
	envelopes   *EnvelopeRepository
}

func NewAssignmentRepository(s *Store, envelopes *EnvelopeRepository) *AssignmentRepository {
	r := &AssignmentRepository{store: s, assignments: map[assignmentKey]budget.Assignment{}, envelopes: envelopes}
	envelopes.assignments = r
	s.Register(r)
	return r
}

func (r *AssignmentRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.assignments)
	return func() { r.assignments = saved }
}

func (r *AssignmentRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.assignments, func(_ assignmentKey, a budget.Assignment) bool { return a.HouseholdID == id })
}

func (r *AssignmentRepository) dropEnvelope(id domain.ID) {
	maps.DeleteFunc(r.assignments, func(k assignmentKey, _ budget.Assignment) bool { return k.envelope == id })
}

func (r *AssignmentRepository) Set(ctx context.Context, a budget.Assignment) (err error) {
	r.store.Do(ctx, func() {
		if !r.envelopes.has(a.HouseholdID, a.EnvelopeID) {
			err = domain.ErrNotFound
			return
		}
		k := assignmentKey{a.EnvelopeID, a.Month}
		if a.Amount == 0 {
			delete(r.assignments, k)
			return
		}
		r.assignments[k] = a
	})
	return err
}

func (r *AssignmentRepository) Get(ctx context.Context, householdID, envelopeID domain.ID, m budget.Month) (a budget.Assignment, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if a, ok = r.assignments[assignmentKey{envelopeID, m}]; !ok || a.HouseholdID != householdID {
			a = budget.Assignment{HouseholdID: householdID, EnvelopeID: envelopeID, Month: m}
		}
	})
	return a, nil
}

func (r *AssignmentRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Assignment, error) {
	out := []budget.Assignment{}
	r.store.Do(ctx, func() {
		for _, a := range r.assignments {
			if a.HouseholdID == householdID {
				out = append(out, a)
			}
		}
	})
	slices.SortFunc(out, func(a, b budget.Assignment) int {
		return cmp.Or(a.Month.Compare(b.Month), cmp.Compare(a.EnvelopeID, b.EnvelopeID))
	})
	return out, nil
}
