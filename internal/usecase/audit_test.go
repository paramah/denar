package usecase_test

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/usecase"
)

func TestListAudit(t *testing.T) {
	f := newFamily(t)
	for _, name := range []string{"a", "b", "c"} {
		_, err := f.createBot(membership.RoleMember, name, "wallets:read")
		noErr(t, err)
	}
	_, err := usecase.NewUpdateMemberRole(f.deps).Do(ctx, human(f.owner), "h1", "viewer", membership.RoleMember)
	noErr(t, err)
	uc := usecase.NewListAudit(f.deps)

	page, err := uc.Do(ctx, human(f.admin), "h1", usecase.ListAuditInput{Limit: domain.Some(2)})
	noErr(t, err)
	if len(page.Items) != 2 || page.NextCursor == "" || page.Items[0].Action != "membership.role_changed" {
		t.Errorf("page = %+v", page)
	}
	rest, err := uc.Do(ctx, human(f.admin), "h1", usecase.ListAuditInput{Limit: domain.Some(2), Cursor: page.NextCursor})
	noErr(t, err)
	if len(rest.Items) != 2 || rest.NextCursor != "" {
		t.Errorf("rest = %+v", rest)
	}
	bots, err := uc.Do(ctx, human(f.owner), "h1", usecase.ListAuditInput{ResourceType: audit.ResourceBot, ActorKind: "user", ActorID: "member"})
	noErr(t, err)
	if len(bots.Items) != 3 {
		t.Errorf("by resource and actor = %d", len(bots.Items))
	}

	_, err = uc.Do(ctx, human(f.member), "h1", usecase.ListAuditInput{})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.outsider), "h1", usecase.ListAuditInput{})
	wantErr(t, err, domain.ErrNotFound)
	_, err = uc.Do(ctx, human(f.owner), "h1", usecase.ListAuditInput{ActorKind: "robot"})
	wantErr(t, err, domain.ErrInvalid)
	_, err = uc.Do(ctx, human(f.owner), "h1", usecase.ListAuditInput{Limit: domain.Some(500)})
	wantErr(t, err, domain.ErrInvalid)
	_, err = uc.Do(ctx, human(f.owner), "h1", usecase.ListAuditInput{Cursor: "garbage"})
	wantErr(t, err, domain.ErrInvalid)
}

// TestChangesRollBackWithTheirAuditEntry is FR-040: the entry is written in
// the transaction of the change, so a failed entry undoes the change.
func TestChangesRollBackWithTheirAuditEntry(t *testing.T) {
	f := newFamily(t)
	f.Audit.AppendErr = errAudit
	_, err := f.createBot(membership.RoleMember, "Bot", "wallets:read")
	wantErr(t, err, errAudit)
	if bots, _ := f.Bots.ListByHousehold(ctx, "h1"); len(bots) != 0 {
		t.Error("bot created without its audit entry")
	}
	_, err = usecase.NewCreateHousehold(f.deps).Do(ctx, human(f.member), usecase.CreateHouseholdInput{Name: "X", BaseCurrency: "PLN", Timezone: "UTC", MonthStartDay: 1})
	wantErr(t, err, errAudit)
	if ms, _ := f.Memberships.ListByUser(ctx, "member"); len(ms) != 1 {
		t.Error("household created without its audit entry")
	}
}
