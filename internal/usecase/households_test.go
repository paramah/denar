package usecase_test

import (
	"slices"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestCreateHousehold(t *testing.T) {
	w := newWorld(t)
	ada := w.addUser("ada")
	got, err := usecase.NewCreateHousehold(w.deps).Do(ctx, human(ada), usecase.CreateHouseholdInput{
		Name: " Dom ", BaseCurrency: "pln", Timezone: "Europe/Warsaw", MonthStartDay: 10})
	noErr(t, err)
	if got.Role != membership.RoleOwner || got.Household.Name != "Dom" || got.Household.BaseCurrency != "PLN" || got.Household.CreatedBy != "ada" {
		t.Errorf("view = %+v", got)
	}
	m, err := w.Memberships.Get(ctx, got.Household.ID, "ada")
	noErr(t, err)
	if m.Role != membership.RoleOwner {
		t.Errorf("role = %s", m.Role)
	}
	entries := w.Audit.All()
	if len(entries) != 1 || entries[0].Action != "household.created" || entries[0].Actor.ID != "ada" || entries[0].Changes["base_currency"].To != "PLN" {
		t.Errorf("audit = %+v", entries)
	}

	_, err = usecase.NewCreateHousehold(w.deps).Do(ctx, human(ada), usecase.CreateHouseholdInput{Name: "", BaseCurrency: "XXX", Timezone: "Nowhere", MonthStartDay: 31})
	wantErr(t, err, domain.ErrInvalid)

	b := w.addBot("b1", got.Household.ID, ada, bot.ScopeWalletsWrite)
	_, err = usecase.NewCreateHousehold(w.deps).Do(ctx, asBot(b), usecase.CreateHouseholdInput{Name: "X", BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 1})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
}

func TestHouseholdIsolation(t *testing.T) {
	f := newFamily(t)
	get := usecase.NewGetHousehold(f.deps)
	_, err := get.Do(ctx, human(f.outsider), "h1")
	wantErr(t, err, domain.ErrNotFound)
	_, err = get.Do(ctx, human(f.owner), "nope")
	wantErr(t, err, domain.ErrNotFound)
	v, err := get.Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if v.Role != membership.RoleViewer {
		t.Errorf("role = %s", v.Role)
	}

	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	v, err = get.Do(ctx, asBot(b), "h1")
	noErr(t, err)
	if v.Role != membership.RoleMember {
		t.Errorf("bot sees creator role %s", v.Role)
	}
	_, err = get.Do(ctx, asBot(b), "h2")
	wantErr(t, err, domain.ErrNotFound)

	list, err := usecase.NewListHouseholds(f.deps).Do(ctx, human(f.owner))
	noErr(t, err)
	if len(list) != 1 || list[0].Household.ID != "h1" {
		t.Errorf("owner lists %+v", list)
	}
	f.addMember("h2", f.owner, membership.RoleViewer)
	list, err = usecase.NewListHouseholds(f.deps).Do(ctx, human(f.owner))
	noErr(t, err)
	if len(list) != 2 || list[1].Household.ID != "h2" || list[1].Role != membership.RoleViewer {
		t.Errorf("owner lists %+v", list)
	}
	list, err = usecase.NewListHouseholds(f.deps).Do(ctx, asBot(b))
	noErr(t, err)
	if len(list) != 1 || list[0].Household.ID != "h1" {
		t.Errorf("bot lists %+v", list)
	}
}

func TestUpdateHousehold(t *testing.T) {
	f := newFamily(t)
	uc := usecase.NewUpdateHousehold(f.deps)
	in := usecase.UpdateHouseholdInput{Name: domain.Some("Nowy"), Timezone: domain.Some("UTC"), MonthStartDay: domain.Some(15)}
	for _, r := range []membership.Role{membership.RoleMember, membership.RoleViewer} {
		_, err := uc.Do(ctx, human(f.byRole(r)), "h1", in)
		wantForbidden(t, err, domain.ForbiddenRole)
	}
	_, err := uc.Do(ctx, human(f.outsider), "h1", in)
	wantErr(t, err, domain.ErrNotFound)
	b := f.addBot("b1", "h1", f.owner, bot.Scopes...)
	_, err = uc.Do(ctx, asBot(b), "h1", in)
	wantForbidden(t, err, domain.ForbiddenHumanOnly)

	got, err := uc.Do(ctx, human(f.admin), "h1", in)
	noErr(t, err)
	if got.Household.Name != "Nowy" || got.Household.Timezone != "UTC" || got.Household.MonthStartDay != 15 || got.Household.BaseCurrency != "PLN" {
		t.Errorf("updated = %+v", got)
	}
	stored, _ := f.Households.ByID(ctx, "h1")
	if stored.Name != "Nowy" {
		t.Errorf("stored = %+v", stored)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "household.updated" || last.Changes["name"].From != "Dom h1" || last.Changes["month_start_day"].To != 15 {
		t.Errorf("audit = %+v", last)
	}

	_, err = uc.Do(ctx, human(f.owner), "h1", usecase.UpdateHouseholdInput{MonthStartDay: domain.Some(29)})
	wantErr(t, err, domain.ErrInvalid)
	// An update that changes nothing writes no entry.
	n := len(f.Audit.All())
	_, err = uc.Do(ctx, human(f.owner), "h1", usecase.UpdateHouseholdInput{Name: domain.Some("Nowy")})
	noErr(t, err)
	if len(f.Audit.All()) != n {
		t.Error("no-op update audited")
	}
}

func TestDeleteHousehold(t *testing.T) {
	f := newFamily(t)
	f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	uc := usecase.NewDeleteHousehold(f.deps)
	wantForbidden(t, uc.Do(ctx, human(f.admin), "h1"), domain.ForbiddenRole)
	wantErr(t, uc.Do(ctx, human(f.outsider), "h1"), domain.ErrNotFound)
	noErr(t, uc.Do(ctx, human(f.owner), "h1"))
	_, err := f.Households.ByID(ctx, "h1")
	wantErr(t, err, domain.ErrNotFound)
	if _, err := f.Bots.ByID(ctx, "b1"); err == nil {
		t.Error("bot survives its household")
	}
	for _, e := range f.Audit.All() {
		if e.HouseholdID == "h1" {
			t.Errorf("audit survives: %+v", e)
		}
	}
}

func TestGetMe(t *testing.T) {
	f := newFamily(t)
	me, err := usecase.NewGetMe(f.deps).Do(ctx, human(f.member))
	noErr(t, err)
	if me.User == nil || me.User.ID != "member" || me.User.PasswordHash != "" || len(me.Households) != 1 || me.Households[0].Role != membership.RoleMember || me.Bot != nil {
		t.Errorf("me = %+v", me)
	}
	b := f.addBot("b1", "h1", f.viewer, bot.ScopeWalletsRead, bot.ScopeWalletsWrite)
	me, err = usecase.NewGetMe(f.deps).Do(ctx, asBot(b))
	noErr(t, err)
	if me.User != nil || me.Bot == nil || me.Bot.Bot.TokenHash != "" || me.Bot.CreatorRole != membership.RoleViewer ||
		!slices.Equal(me.Bot.EffectiveScopes, []bot.Scope{bot.ScopeWalletsRead}) {
		t.Errorf("bot me = %+v", me.Bot)
	}
}

var _ = audit.ResourceBot
