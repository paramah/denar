package usecase_test

import (
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestListMembers(t *testing.T) {
	f := newFamily(t)
	list, err := usecase.NewListMembers(f.deps).Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if len(list) != 4 || list[0].User.ID != "owner" || list[0].Membership.Role != membership.RoleOwner || list[3].User.Email != "viewer@example.com" {
		t.Errorf("members = %+v", list)
	}
	_, err = usecase.NewListMembers(f.deps).Do(ctx, human(f.outsider), "h1")
	wantErr(t, err, domain.ErrNotFound)
	b := f.addBot("b1", "h1", f.owner, bot.Scopes...)
	_, err = usecase.NewListMembers(f.deps).Do(ctx, asBot(b), "h1")
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
}

func TestUpdateMemberRole(t *testing.T) {
	f := newFamily(t)
	uc := usecase.NewUpdateMemberRole(f.deps)

	got, err := uc.Do(ctx, human(f.admin), "h1", "member", membership.RoleViewer)
	noErr(t, err)
	if got.Membership.Role != membership.RoleViewer || got.User.ID != "member" {
		t.Errorf("view = %+v", got)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "membership.role_changed" || last.Changes["role"].From != "member" || last.Changes["role"].To != "viewer" {
		t.Errorf("audit = %+v", last)
	}

	_, err = uc.Do(ctx, human(f.admin), "h1", "owner", membership.RoleMember)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.admin), "h1", "viewer", membership.RoleOwner)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.viewer), "h1", "member", membership.RoleViewer)
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.owner), "h1", "owner", membership.RoleAdmin)
	wantConflict(t, err, domain.ConflictLastOwner)
	_, err = uc.Do(ctx, human(f.owner), "h1", "nobody", membership.RoleAdmin)
	wantErr(t, err, domain.ErrNotFound)
	_, err = uc.Do(ctx, human(f.owner), "h1", "admin", "boss")
	wantErr(t, err, domain.ErrInvalid)
	_, err = uc.Do(ctx, human(f.outsider), "h1", "admin", membership.RoleViewer)
	wantErr(t, err, domain.ErrNotFound)

	// With a second owner, the first may step down.
	_, err = uc.Do(ctx, human(f.owner), "h1", "admin", membership.RoleOwner)
	noErr(t, err)
	_, err = uc.Do(ctx, human(f.owner), "h1", "owner", membership.RoleMember)
	noErr(t, err)
}

func TestRemoveMemberDisablesTheirBots(t *testing.T) {
	f := newFamily(t)
	f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	f.addBot("b2", "h1", f.member, bot.ScopeReportsRead)
	f.addBot("b3", "h1", f.admin, bot.ScopeReportsRead)
	uc := usecase.NewRemoveMember(f.deps)

	wantForbidden(t, uc.Do(ctx, human(f.viewer), "h1", "member"), domain.ForbiddenRole)
	wantForbidden(t, uc.Do(ctx, human(f.admin), "h1", "owner"), domain.ForbiddenRole)
	wantConflict(t, uc.Do(ctx, human(f.owner), "h1", "owner"), domain.ConflictLastOwner)
	wantErr(t, uc.Do(ctx, human(f.owner), "h1", "nobody"), domain.ErrNotFound)

	noErr(t, uc.Do(ctx, human(f.admin), "h1", "member"))
	_, err := f.Memberships.Get(ctx, "h1", "member")
	wantErr(t, err, domain.ErrNotFound)
	for id, want := range map[domain.ID]bool{"b1": true, "b2": true, "b3": false} {
		b, _ := f.Bots.ByID(ctx, id)
		if b.Disabled != want {
			t.Errorf("%s disabled = %v", id, b.Disabled)
		}
	}
	got := f.actions()
	want := []string{"membership.removed", "bot.disabled", "bot.disabled"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("audit = %v", got)
	}

	// A viewer may leave on their own.
	noErr(t, uc.Do(ctx, human(f.viewer), "h1", "viewer"))
}

func TestRemoveMemberIsAtomic(t *testing.T) {
	f := newFamily(t)
	f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	f.Audit.AppendErr = errAudit
	err := usecase.NewRemoveMember(f.deps).Do(ctx, human(f.owner), "h1", "member")
	wantErr(t, err, errAudit)
	if _, err := f.Memberships.Get(ctx, "h1", "member"); err != nil {
		t.Error("membership removed although the audit entry failed")
	}
	if b, _ := f.Bots.ByID(ctx, "b1"); b.Disabled {
		t.Error("bot disabled although the transaction failed")
	}
}
