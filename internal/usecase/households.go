package usecase

import (
	"context"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/membership"
)

// HouseholdView is a household with the role the caller has in it (for a
// bot: its creator's current role).
type HouseholdView struct {
	Household household.Household
	Role      membership.Role
}

type CreateHouseholdInput struct {
	Name          string
	BaseCurrency  string
	Timezone      string
	MonthStartDay int
}

// CreateHousehold creates a household and makes its creator the owner
// (FR-020). Any signed-in person may (decision D-006); bots may not.
type CreateHousehold struct{ d Deps }

func NewCreateHousehold(d Deps) *CreateHousehold { return &CreateHousehold{d: d} }

func (uc *CreateHousehold) Do(ctx context.Context, p Principal, in CreateHouseholdInput) (HouseholdView, error) {
	if err := requireHuman(p); err != nil {
		return HouseholdView{}, err
	}
	h := household.Household{
		Name: in.Name, BaseCurrency: in.BaseCurrency, Timezone: in.Timezone, MonthStartDay: in.MonthStartDay,
	}.Normalize()
	if err := h.Validate(); err != nil {
		return HouseholdView{}, err
	}
	h.ID, h.CreatedBy, h.CreatedAt = uc.d.IDs.NewID(), p.User.ID, uc.d.Clock.Now()
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.d.Households.Create(ctx, h); err != nil {
			return err
		}
		if err := uc.d.Memberships.Create(ctx, membership.Membership{
			HouseholdID: h.ID, UserID: p.User.ID, Role: membership.RoleOwner, CreatedAt: h.CreatedAt,
		}); err != nil {
			return err
		}
		return uc.d.record(ctx, h.ID, p.Actor(), "household.created", audit.ResourceHousehold, h.ID, audit.Changes{}.
			Set("name", h.Name).Set("base_currency", h.BaseCurrency).Set("timezone", h.Timezone).
			Set("month_start_day", h.MonthStartDay))
	})
	if err != nil {
		return HouseholdView{}, err
	}
	return HouseholdView{Household: h, Role: membership.RoleOwner}, nil
}

// ListHouseholds returns the households the person is a member of, oldest
// membership first. A bot gets its own household.
type ListHouseholds struct{ d Deps }

func NewListHouseholds(d Deps) *ListHouseholds { return &ListHouseholds{d: d} }

func (uc *ListHouseholds) Do(ctx context.Context, p Principal) ([]HouseholdView, error) {
	if p.IsBot() {
		s, err := uc.d.access().household(ctx, p, p.Bot.HouseholdID)
		if err != nil {
			return nil, err
		}
		return []HouseholdView{{Household: s.Household, Role: s.Role}}, nil
	}
	return myHouseholds(ctx, uc.d, p.User.ID)
}

func myHouseholds(ctx context.Context, d Deps, userID domain.ID) ([]HouseholdView, error) {
	ms, err := d.Memberships.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]domain.ID, len(ms))
	for i, m := range ms {
		ids[i] = m.HouseholdID
	}
	hs, err := d.Households.ByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]HouseholdView, 0, len(ms))
	for _, m := range ms {
		if i := slices.IndexFunc(hs, func(h household.Household) bool { return h.ID == m.HouseholdID }); i >= 0 {
			out = append(out, HouseholdView{Household: hs[i], Role: m.Role})
		}
	}
	return out, nil
}

// GetHousehold returns one household to a member or to its own bot.
type GetHousehold struct{ d Deps }

func NewGetHousehold(d Deps) *GetHousehold { return &GetHousehold{d: d} }

func (uc *GetHousehold) Do(ctx context.Context, p Principal, id domain.ID) (HouseholdView, error) {
	s, err := uc.d.access().household(ctx, p, id)
	if err != nil {
		return HouseholdView{}, err
	}
	return HouseholdView{Household: s.Household, Role: s.Role}, nil
}

type UpdateHouseholdInput struct {
	Name          domain.Optional[string]
	Timezone      domain.Optional[string]
	MonthStartDay domain.Optional[int]
}

// UpdateHousehold changes the settings of a household; owners and admins
// only. The base currency cannot change (D-007).
type UpdateHousehold struct{ d Deps }

func NewUpdateHousehold(d Deps) *UpdateHousehold { return &UpdateHousehold{d: d} }

func (uc *UpdateHousehold) Do(ctx context.Context, p Principal, id domain.ID, in UpdateHouseholdInput) (HouseholdView, error) {
	if err := requireHuman(p); err != nil {
		return HouseholdView{}, err
	}
	var view HouseholdView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, id)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleAdmin); err != nil {
			return err
		}
		old, h := s.Household, s.Household
		if in.Name.Set {
			h.Name = in.Name.Value
		}
		if in.Timezone.Set {
			h.Timezone = in.Timezone.Value
		}
		if in.MonthStartDay.Set {
			h.MonthStartDay = in.MonthStartDay.Value
		}
		h = h.Normalize()
		if err := h.Validate(); err != nil {
			return err
		}
		changes := audit.Changes{}.Diff("name", old.Name, h.Name).Diff("timezone", old.Timezone, h.Timezone).
			Diff("month_start_day", old.MonthStartDay, h.MonthStartDay)
		view = HouseholdView{Household: h, Role: s.Role}
		if len(changes) == 0 {
			return nil
		}
		if err := uc.d.Households.Update(ctx, h); err != nil {
			return err
		}
		return uc.d.record(ctx, h.ID, p.Actor(), "household.updated", audit.ResourceHousehold, h.ID, changes)
	})
	return view, err
}

// DeleteHousehold removes a household and everything in it; owners only.
// Its audit trail goes with it, so the deletion is logged instead.
type DeleteHousehold struct{ d Deps }

func NewDeleteHousehold(d Deps) *DeleteHousehold { return &DeleteHousehold{d: d} }

func (uc *DeleteHousehold) Do(ctx context.Context, p Principal, id domain.ID) error {
	if err := requireHuman(p); err != nil {
		return err
	}
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, id)
		if err != nil {
			return err
		}
		if err := requireRole(s, membership.RoleOwner); err != nil {
			return err
		}
		return uc.d.Households.Delete(ctx, id)
	})
	if err == nil {
		uc.d.log().InfoContext(ctx, "audit", "event", "household.deleted", "household_id", string(id), "actor_id", string(p.User.ID))
	}
	return err
}
