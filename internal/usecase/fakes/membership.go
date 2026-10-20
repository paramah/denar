package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

type memberKey struct{ household, user domain.ID }

// MembershipRepository keeps memberships in memory.
type MembershipRepository struct {
	store   *Store
	members map[memberKey]membership.Membership
}

func NewMembershipRepository(s *Store) *MembershipRepository {
	r := &MembershipRepository{store: s, members: map[memberKey]membership.Membership{}}
	s.Register(r)
	return r
}

func (r *MembershipRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.members)
	return func() { r.members = saved }
}

func (r *MembershipRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.members, func(k memberKey, _ membership.Membership) bool { return k.household == id })
}

func (r *MembershipRepository) Create(ctx context.Context, m membership.Membership) (err error) {
	r.store.Do(ctx, func() {
		k := memberKey{m.HouseholdID, m.UserID}
		if _, ok := r.members[k]; ok {
			err = domain.ConflictError{Reason: domain.ConflictAlreadyMember}
			return
		}
		r.members[k] = m
	})
	return err
}

func (r *MembershipRepository) UpdateRole(ctx context.Context, householdID, userID domain.ID, role membership.Role) (err error) {
	r.store.Do(ctx, func() {
		k := memberKey{householdID, userID}
		m, ok := r.members[k]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		m.Role = role
		r.members[k] = m
	})
	return err
}

func (r *MembershipRepository) Delete(ctx context.Context, householdID, userID domain.ID) (err error) {
	r.store.Do(ctx, func() {
		k := memberKey{householdID, userID}
		if _, ok := r.members[k]; !ok {
			err = domain.ErrNotFound
			return
		}
		delete(r.members, k)
	})
	return err
}

func (r *MembershipRepository) Get(ctx context.Context, householdID, userID domain.ID) (m membership.Membership, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if m, ok = r.members[memberKey{householdID, userID}]; !ok {
			err = domain.ErrNotFound
		}
	})
	return m, err
}

func (r *MembershipRepository) list(ctx context.Context, match func(membership.Membership) bool, tie func(membership.Membership) domain.ID) []membership.Membership {
	out := []membership.Membership{}
	r.store.Do(ctx, func() {
		for _, m := range r.members {
			if match(m) {
				out = append(out, m)
			}
		}
	})
	slices.SortFunc(out, func(a, b membership.Membership) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(tie(a), tie(b)))
	})
	return out
}

func (r *MembershipRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Membership, error) {
	return r.list(ctx, func(m membership.Membership) bool { return m.HouseholdID == householdID },
		func(m membership.Membership) domain.ID { return m.UserID }), nil
}

func (r *MembershipRepository) ListByUser(ctx context.Context, userID domain.ID) ([]membership.Membership, error) {
	return r.list(ctx, func(m membership.Membership) bool { return m.UserID == userID },
		func(m membership.Membership) domain.ID { return m.HouseholdID }), nil
}
