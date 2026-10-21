package usecase

import (
	"context"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/user"
)

// MemberView is a membership with the member's account.
type MemberView struct {
	Membership membership.Membership
	User       user.User
}

// ListMembers returns the members of a household to any of its members.
type ListMembers struct{ d Deps }

func NewListMembers(d Deps) *ListMembers { return &ListMembers{d: d} }

func (uc *ListMembers) Do(ctx context.Context, p Principal, householdID domain.ID) ([]MemberView, error) {
	if err := requireHuman(p); err != nil {
		return nil, err
	}
	if _, err := uc.d.access().household(ctx, p, householdID); err != nil {
		return nil, err
	}
	ms, err := uc.d.Memberships.ListByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	return memberViews(ctx, uc.d, ms)
}

func memberViews(ctx context.Context, d Deps, ms []membership.Membership) ([]MemberView, error) {
	ids := make([]domain.ID, len(ms))
	for i, m := range ms {
		ids[i] = m.UserID
	}
	us, err := d.Users.ByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[domain.ID]user.User, len(us))
	for _, u := range us {
		byID[u.ID] = u
	}
	out := make([]MemberView, 0, len(ms))
	for _, m := range ms {
		out = append(out, MemberView{Membership: m, User: byID[m.UserID]})
	}
	return out, nil
}

// UpdateMemberRole changes a member's role, within the rules of
// membership.CheckRoleChange and never leaving the household without an
// owner (FR-022, FR-024). A creator who drops to viewer narrows their
// bots at once, as access reads the role on every request.
type UpdateMemberRole struct{ d Deps }

func NewUpdateMemberRole(d Deps) *UpdateMemberRole { return &UpdateMemberRole{d: d} }

func (uc *UpdateMemberRole) Do(ctx context.Context, p Principal, householdID, userID domain.ID, role membership.Role) (MemberView, error) {
	if err := requireHuman(p); err != nil {
		return MemberView{}, err
	}
	if err := membership.ValidateRole("role", role); err != nil {
		return MemberView{}, err
	}
	var view MemberView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleAdmin); err != nil {
			return err
		}
		members, err := uc.d.Memberships.ListByHousehold(ctx, householdID)
		if err != nil {
			return err
		}
		target, ok := findMember(members, userID)
		if !ok {
			return domain.ErrNotFound
		}
		if err := membership.CheckRoleChange(s.Role, target.Role, role); err != nil {
			return err
		}
		if err := membership.CheckLastOwner(members, userID, role, false); err != nil {
			return err
		}
		if target.Role != role {
			if err := uc.d.Memberships.UpdateRole(ctx, householdID, userID, role); err != nil {
				return err
			}
			if err := uc.d.record(ctx, householdID, p.Actor(), "membership.role_changed", audit.ResourceMembership, userID,
				audit.Changes{}.Diff("role", string(target.Role), string(role))); err != nil {
				return err
			}
		}
		target.Role = role
		views, err := memberViews(ctx, uc.d, []membership.Membership{target})
		if err != nil {
			return err
		}
		view = views[0]
		return nil
	})
	return view, err
}

func findMember(ms []membership.Membership, userID domain.ID) (membership.Membership, bool) {
	for _, m := range ms {
		if m.UserID == userID {
			return m, true
		}
	}
	return membership.Membership{}, false
}

// RemoveMember takes a member out of a household, or lets a member leave.
// The bots the member created there are disabled in the same transaction
// (US-004).
type RemoveMember struct{ d Deps }

func NewRemoveMember(d Deps) *RemoveMember { return &RemoveMember{d: d} }

func (uc *RemoveMember) Do(ctx context.Context, p Principal, householdID, userID domain.ID) error {
	if err := requireHuman(p); err != nil {
		return err
	}
	return uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		members, err := uc.d.Memberships.ListByHousehold(ctx, householdID)
		if err != nil {
			return err
		}
		target, ok := findMember(members, userID)
		if !ok {
			return domain.ErrNotFound
		}
		if err := membership.CheckRemoval(p.User.ID, s.Role, userID, target.Role); err != nil {
			return err
		}
		if err := membership.CheckLastOwner(members, userID, "", true); err != nil {
			return err
		}
		if err := uc.d.Memberships.Delete(ctx, householdID, userID); err != nil {
			return err
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "membership.removed", audit.ResourceMembership, userID,
			audit.Changes{}.Diff("role", string(target.Role), nil)); err != nil {
			return err
		}
		return disableBotsOf(ctx, uc.d, householdID, userID, p.Actor())
	})
}

// disableBotsOf disables the bots of a creator in a household and records
// each. Call it in a transaction.
func disableBotsOf(ctx context.Context, d Deps, householdID, creatorID domain.ID, actor domain.Actor) error {
	disabled, err := d.Bots.DisableByCreator(ctx, householdID, creatorID)
	if err != nil {
		return err
	}
	for _, b := range disabled {
		if err := d.record(ctx, householdID, actor, "bot.disabled", audit.ResourceBot, b.ID,
			audit.Changes{}.Diff("disabled", false, true)); err != nil {
			return err
		}
	}
	return nil
}
