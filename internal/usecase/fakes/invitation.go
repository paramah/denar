package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

// InvitationRepository keeps invitations in memory.
type InvitationRepository struct {
	store       *Store
	invitations map[domain.ID]membership.Invitation
}

func NewInvitationRepository(s *Store) *InvitationRepository {
	r := &InvitationRepository{store: s, invitations: map[domain.ID]membership.Invitation{}}
	s.Register(r)
	return r
}

func (r *InvitationRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.invitations)
	return func() { r.invitations = saved }
}

func (r *InvitationRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.invitations, func(_ domain.ID, i membership.Invitation) bool { return i.HouseholdID == id })
}

func (r *InvitationRepository) Create(ctx context.Context, i membership.Invitation) (err error) {
	r.store.Do(ctx, func() {
		for _, other := range r.invitations {
			if other.ID == i.ID || other.TokenHash == i.TokenHash {
				err = domain.ErrConflict
				return
			}
		}
		r.invitations[i.ID] = i
	})
	return err
}

func (r *InvitationRepository) Update(ctx context.Context, i membership.Invitation) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.invitations[i.ID]
		if !ok {
			err = domain.ErrNotFound
			return
		}
		old.AcceptedAt, old.AcceptedBy, old.RevokedAt = i.AcceptedAt, i.AcceptedBy, i.RevokedAt
		r.invitations[i.ID] = old
	})
	return err
}

func (r *InvitationRepository) find(ctx context.Context, match func(membership.Invitation) bool) []membership.Invitation {
	out := []membership.Invitation{}
	r.store.Do(ctx, func() {
		for _, i := range r.invitations {
			if match(i) {
				out = append(out, i)
			}
		}
	})
	slices.SortFunc(out, func(a, b membership.Invitation) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
	})
	return out
}

func (r *InvitationRepository) one(ctx context.Context, match func(membership.Invitation) bool) (membership.Invitation, error) {
	found := r.find(ctx, match)
	if len(found) == 0 {
		return membership.Invitation{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *InvitationRepository) ByID(ctx context.Context, id domain.ID) (membership.Invitation, error) {
	return r.one(ctx, func(i membership.Invitation) bool { return i.ID == id })
}

func (r *InvitationRepository) ByTokenHash(ctx context.Context, hash string) (membership.Invitation, error) {
	return r.one(ctx, func(i membership.Invitation) bool { return hash != "" && i.TokenHash == hash })
}

func (r *InvitationRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Invitation, error) {
	return r.find(ctx, func(i membership.Invitation) bool { return i.HouseholdID == householdID }), nil
}

func (r *InvitationRepository) PendingByEmail(ctx context.Context, email string, now time.Time) ([]membership.Invitation, error) {
	return r.find(ctx, func(i membership.Invitation) bool {
		return i.Email == email && i.AcceptedAt.IsZero() && i.RevokedAt.IsZero() && i.ExpiresAt.After(now)
	}), nil
}
