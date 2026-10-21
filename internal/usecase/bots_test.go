package usecase_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func (f *family) createBot(by membership.Role, name string, scopes ...string) (usecase.BotWithToken, error) {
	return usecase.NewCreateBot(f.deps).Do(ctx, human(f.byRole(by)), "h1", usecase.CreateBotInput{Name: name, Scopes: scopes})
}

func TestCreateBot(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	got, err := usecase.NewCreateBot(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateBotInput{
		Name: " Claude ", Scopes: []string{"transactions:write", "wallets:read"}, WalletIDs: []string{"w1", "w1"},
		ExpiresAt: f.Clock.Now().Add(90 * 24 * time.Hour)})
	noErr(t, err)
	if !strings.HasPrefix(got.Token, "dnr_") || got.Bot.TokenHash != "" || got.Bot.TokenPrefix != got.Token[:8] ||
		got.Bot.Name != "Claude" || got.Bot.CreatedBy != "member" || !slices.Equal(got.Bot.WalletIDs, []domain.ID{"w1"}) ||
		!slices.Equal(got.EffectiveScopes, []bot.Scope{bot.ScopeWalletsRead, bot.ScopeTransactionsWrite}) {
		t.Errorf("bot = %+v", got)
	}
	stored, err := f.Bots.ByTokenHash(ctx, usecase.HashToken(got.Token))
	noErr(t, err)
	if stored.ID != got.Bot.ID {
		t.Error("token hash not stored")
	}
	if a := f.actions(); len(a) != 1 || a[0] != "bot.created" {
		t.Errorf("audit = %v", a)
	}
	for _, e := range f.Audit.All() {
		for _, c := range e.Changes {
			if s, ok := c.To.(string); ok && strings.Contains(s, got.Token[4:]) {
				t.Error("the token is in the audit log")
			}
		}
	}

	_, err = f.createBot(membership.RoleViewer, "V", "wallets:read")
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = f.createBot(membership.RoleMember, "claude", "wallets:read")
	wantConflict(t, err, domain.ConflictDuplicateName)
	_, err = f.createBot(membership.RoleMember, "", "nope:read")
	wantErr(t, err, domain.ErrInvalid)
	_, err = usecase.NewCreateBot(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateBotInput{Name: "Old", Scopes: []string{"wallets:read"}, ExpiresAt: f.Clock.Now()})
	wantErr(t, err, domain.ErrInvalid)
	_, err = usecase.NewCreateBot(f.deps).Do(ctx, human(f.outsider), "h1", usecase.CreateBotInput{Name: "X", Scopes: []string{"wallets:read"}})
	wantErr(t, err, domain.ErrNotFound)
	_, err = usecase.NewCreateBot(f.deps).Do(ctx, asBot(stored), "h1", usecase.CreateBotInput{Name: "Child", Scopes: []string{"wallets:read"}})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
}

// TestCreateBotChecksWallets: the wallets of a bot must be wallets of the
// household its creator sees (FR-150).
func TestCreateBotChecksWallets(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWallet("hidden", "h1", f.owner, wallet.Private)
	f.addWallet("mine", "h1", f.member, wallet.Private)
	f.addWallet("foreign", "h2", f.outsider, wallet.Shared)
	created, err := usecase.NewCreateBot(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateBotInput{Name: "A", Scopes: []string{"wallets:read"}, WalletIDs: []string{"w1", "mine"}})
	noErr(t, err)
	if !slices.Equal(created.Bot.WalletIDs, []domain.ID{"w1", "mine"}) {
		t.Errorf("wallets = %v", created.Bot.WalletIDs)
	}
	for _, id := range []string{"hidden", "foreign", "nope"} {
		_, err = usecase.NewCreateBot(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateBotInput{Name: "B " + id, Scopes: []string{"wallets:read"}, WalletIDs: []string{id}})
		wantErr(t, err, domain.ErrInvalid)
	}
}

// TestBotManagementByRole is decision D-010: owners and admins manage every
// bot of the household, others only their own; nobody sees bots of another
// household.
func TestBotManagementByRole(t *testing.T) {
	roles := membership.Roles
	for _, creator := range []membership.Role{membership.RoleOwner, membership.RoleAdmin, membership.RoleMember} {
		for _, actor := range roles {
			t.Run(string(creator)+" bot, "+string(actor)+" acts", func(t *testing.T) {
				f := newFamily(t)
				created, err := f.createBot(creator, "Bot", "wallets:read")
				noErr(t, err)
				id := created.Bot.ID
				p := human(f.byRole(actor))
				allowed := actor.CanManage() || actor == creator

				_, err = usecase.NewGetBot(f.deps).Do(ctx, p, "h1", id)
				check(t, "get", err, allowed)
				_, err = usecase.NewUpdateBot(f.deps).Do(ctx, p, "h1", id, usecase.UpdateBotInput{Name: domain.Some("Renamed")})
				check(t, "update", err, allowed)
				_, err = usecase.NewRotateBotToken(f.deps).Do(ctx, p, "h1", id)
				check(t, "rotate", err, allowed)
				list, err := usecase.NewListBots(f.deps).Do(ctx, p, "h1")
				noErr(t, err)
				if (len(list) == 1) != allowed {
					t.Errorf("list shows %d bots, allowed %v", len(list), allowed)
				}
				err = usecase.NewDeleteBot(f.deps).Do(ctx, p, "h1", id)
				check(t, "delete", err, allowed)

				_, err = usecase.NewGetBot(f.deps).Do(ctx, human(f.outsider), "h2", id)
				wantErr(t, err, domain.ErrNotFound)
			})
		}
	}
}

func check(t *testing.T, what string, err error, allowed bool) {
	t.Helper()
	if allowed && err != nil {
		t.Errorf("%s: %v, want allowed", what, err)
	}
	if !allowed {
		var fe domain.ForbiddenError
		if err == nil || !strings.Contains(err.Error(), "forbidden") || !asForbidden(err, &fe) {
			t.Errorf("%s: %v, want forbidden", what, err)
		}
	}
}

func asForbidden(err error, fe *domain.ForbiddenError) bool {
	for err != nil {
		if e, ok := err.(domain.ForbiddenError); ok {
			*fe = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestUpdateBot(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w2", "h1", f.owner, wallet.Shared)
	created, err := f.createBot(membership.RoleMember, "Bot", "wallets:read")
	noErr(t, err)
	uc := usecase.NewUpdateBot(f.deps)
	exp := f.Clock.Now().Add(time.Hour)
	got, err := uc.Do(ctx, human(f.member), "h1", created.Bot.ID, usecase.UpdateBotInput{
		Scopes: domain.Some([]string{"reports:read", "budget:write"}), WalletIDs: domain.Some([]string{"w2"}),
		ExpiresAt: domain.Some(exp), Disabled: domain.Some(true)})
	noErr(t, err)
	if !slices.Equal(got.Bot.Scopes, []bot.Scope{bot.ScopeBudgetWrite, bot.ScopeReportsRead}) || !got.Bot.Disabled ||
		!got.Bot.ExpiresAt.Equal(exp) || !slices.Equal(got.Bot.WalletIDs, []domain.ID{"w2"}) {
		t.Errorf("updated = %+v", got.Bot)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "bot.updated" || last.Changes["scopes"].To != "budget:write reports:read" || last.Changes["disabled"].To != true {
		t.Errorf("audit = %+v", last)
	}
	got, err = uc.Do(ctx, human(f.member), "h1", created.Bot.ID, usecase.UpdateBotInput{ExpiresAt: domain.Some(time.Time{}), WalletIDs: domain.Some([]string{})})
	noErr(t, err)
	if !got.Bot.ExpiresAt.IsZero() || len(got.Bot.WalletIDs) != 0 {
		t.Errorf("cleared = %+v", got.Bot)
	}
	_, err = uc.Do(ctx, human(f.member), "h1", created.Bot.ID, usecase.UpdateBotInput{Scopes: domain.Some([]string{})})
	wantErr(t, err, domain.ErrInvalid)

	// A bot that has expired can still be renamed.
	f.Clock.Advance(2 * time.Hour)
	expired, err := uc.Do(ctx, human(f.member), "h1", created.Bot.ID, usecase.UpdateBotInput{ExpiresAt: domain.Some(f.Clock.Now().Add(time.Minute))})
	noErr(t, err)
	f.Clock.Advance(time.Hour)
	_, err = uc.Do(ctx, human(f.member), "h1", expired.Bot.ID, usecase.UpdateBotInput{Name: domain.Some("Again")})
	noErr(t, err)
}

func TestRotateAndDeleteBot(t *testing.T) {
	f := newFamily(t)
	created, err := f.createBot(membership.RoleMember, "Bot", "wallets:read")
	noErr(t, err)
	auth := usecase.NewAuthenticateBot(f.Bots, f.Clock, nil)
	_, err = auth.Do(ctx, created.Token)
	noErr(t, err)

	rotated, err := usecase.NewRotateBotToken(f.deps).Do(ctx, human(f.member), "h1", created.Bot.ID)
	noErr(t, err)
	if rotated.Token == created.Token {
		t.Error("token not replaced")
	}
	_, err = auth.Do(ctx, created.Token)
	wantErr(t, err, domain.ErrUnauthenticated)
	_, err = auth.Do(ctx, rotated.Token)
	noErr(t, err)

	noErr(t, usecase.NewDeleteBot(f.deps).Do(ctx, human(f.admin), "h1", created.Bot.ID))
	_, err = auth.Do(ctx, rotated.Token)
	wantErr(t, err, domain.ErrUnauthenticated)
	wantErr(t, usecase.NewDeleteBot(f.deps).Do(ctx, human(f.admin), "h1", created.Bot.ID), domain.ErrNotFound)
	a := f.actions()
	if a[len(a)-2] != "bot.token_rotated" || a[len(a)-1] != "bot.deleted" {
		t.Errorf("audit = %v", a)
	}
}

func TestCreateBotCLI(t *testing.T) {
	f := newFamily(t)
	uc := usecase.NewCreateBotCLI(f.deps)
	got, err := uc.Do(ctx, usecase.CreateBotCLIInput{HouseholdID: "h1", CreateBotInput: usecase.CreateBotInput{Name: "cron", Scopes: []string{"reports:read"}}})
	noErr(t, err)
	if got.Bot.CreatedBy != "owner" || got.Token == "" {
		t.Errorf("bot = %+v", got.Bot)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Actor != usecase.SystemActor {
		t.Errorf("actor = %+v", last.Actor)
	}
	got, err = uc.Do(ctx, usecase.CreateBotCLIInput{HouseholdID: "h1", CreatorEmail: "member@example.com", CreateBotInput: usecase.CreateBotInput{Name: "m", Scopes: []string{"reports:read"}}})
	noErr(t, err)
	if got.Bot.CreatedBy != "member" {
		t.Errorf("creator = %s", got.Bot.CreatedBy)
	}
	for _, in := range []usecase.CreateBotCLIInput{
		{HouseholdID: "h1", CreatorEmail: "viewer@example.com", CreateBotInput: usecase.CreateBotInput{Name: "v", Scopes: []string{"reports:read"}}},
		{HouseholdID: "h1", CreatorEmail: "outsider@example.com", CreateBotInput: usecase.CreateBotInput{Name: "o", Scopes: []string{"reports:read"}}},
	} {
		_, err := uc.Do(ctx, in)
		wantErr(t, err, domain.ErrInvalid)
	}
	_, err = uc.Do(ctx, usecase.CreateBotCLIInput{HouseholdID: "nope", CreateBotInput: usecase.CreateBotInput{Name: "x", Scopes: []string{"reports:read"}}})
	wantErr(t, err, domain.ErrNotFound)
}
