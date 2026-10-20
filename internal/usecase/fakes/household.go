package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/household"
)

// householdPart is state that goes with a deleted household. The method
// runs while the store is held.
type householdPart interface {
	dropHousehold(id domain.ID)
}

// HouseholdRepository keeps households in memory. Delete also drops what
// the repositories passed to Cascade hold of the household, as the
// database's foreign keys do.
type HouseholdRepository struct {
	store      *Store
	households map[domain.ID]household.Household
	cascade    []householdPart
}

func NewHouseholdRepository(s *Store) *HouseholdRepository {
	r := &HouseholdRepository{store: s, households: map[domain.ID]household.Household{}}
	s.Register(r)
	return r
}

// Cascade registers repositories whose rows belong to households.
func (r *HouseholdRepository) Cascade(parts ...householdPart) { r.cascade = append(r.cascade, parts...) }

func (r *HouseholdRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.households)
	return func() { r.households = saved }
}

func (r *HouseholdRepository) Create(ctx context.Context, h household.Household) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.households[h.ID]; ok {
			err = domain.ErrConflict
			return
		}
		r.households[h.ID] = h
	})
	return err
}

func (r *HouseholdRepository) Update(ctx context.Context, h household.Household) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.households[h.ID]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		old.Name, old.Timezone, old.MonthStartDay = h.Name, h.Timezone, h.MonthStartDay
		r.households[h.ID] = old
	})
	return err
}

func (r *HouseholdRepository) Delete(ctx context.Context, id domain.ID) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.households[id]; !ok {
			err = domain.ErrNotFound
			return
		}
		delete(r.households, id)
		for _, p := range r.cascade {
			p.dropHousehold(id)
		}
	})
	return err
}

func (r *HouseholdRepository) ByID(ctx context.Context, id domain.ID) (h household.Household, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if h, ok = r.households[id]; !ok {
			err = domain.ErrNotFound
		}
	})
	return h, err
}

func (r *HouseholdRepository) ByIDs(ctx context.Context, ids []domain.ID) (out []household.Household, err error) {
	r.store.Do(ctx, func() {
		out = []household.Household{}
		for _, h := range r.households {
			if slices.Contains(ids, h.ID) {
				out = append(out, h)
			}
		}
	})
	slices.SortFunc(out, func(a, b household.Household) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}
