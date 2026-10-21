package usecase_test

import (
	"fmt"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/usecase"
)

// TestAuthorizationMatrix checks FR-032 over role × scope × creator status
// × kind of principal: a person may use a scope their role permits; a bot
// only a scope it holds and its creator's current role permits, and
// nothing once the creator has left the household.
func TestAuthorizationMatrix(t *testing.T) {
	type creatorStatus string
	const (
		active  creatorStatus = "member"
		removed creatorStatus = "removed"
	)
	for _, role := range membership.Roles {
		for _, scope := range bot.Scopes {
			t.Run(fmt.Sprintf("user %s %s", role, scope), func(t *testing.T) {
				f := newFamily(t)
				s, err := usecase.HouseholdScopeOf(f.deps, ctx, human(f.byRole(role)), "h1")
				noErr(t, err)
				want := scope.IsRead() || role != membership.RoleViewer
				gotAllowed(t, usecase.RequireScope(s, scope), want)
			})
			for _, status := range []creatorStatus{active, removed} {
				for _, granted := range []bool{true, false} {
					name := fmt.Sprintf("bot of %s (%s) granted=%v %s", role, status, granted, scope)
					t.Run(name, func(t *testing.T) {
						f := newFamily(t)
						creator := f.byRole(role)
						var scopes []bot.Scope
						if granted {
							scopes = []bot.Scope{scope}
						} else {
							for _, other := range bot.Scopes {
								if other != scope {
									scopes = append(scopes, other)
								}
							}
						}
						b := f.addBot("b1", "h1", creator, scopes...)
						if status == removed {
							// The owner leaves through a second owner.
							if role == membership.RoleOwner {
								f.addMember("h1", f.outsider, membership.RoleOwner)
							}
							f.must(f.Memberships.Delete(ctx, "h1", creator.ID))
						}
						s, err := usecase.HouseholdScopeOf(f.deps, ctx, asBot(b), "h1")
						noErr(t, err)
						want := granted && status == active && (scope.IsRead() || role != membership.RoleViewer)
						err = usecase.RequireScope(s, scope)
						gotAllowed(t, err, want)
						if err != nil {
							wantForbidden(t, err, domain.ForbiddenScope)
						}
						// Administration stays closed to every bot.
						wantForbidden(t, usecase.RequireRole(s, membership.RoleViewer), domain.ForbiddenHumanOnly)
					})
				}
			}
		}
	}
}

func gotAllowed(t *testing.T, err error, want bool) {
	t.Helper()
	if (err == nil) != want {
		t.Errorf("err = %v, want allowed %v", err, want)
	}
}

// TestBotIsNarrowedAtOnce: a creator who drops to viewer loses the bot's
// write scopes with the next request, without touching the bot.
func TestBotIsNarrowedAtOnce(t *testing.T) {
	f := newFamily(t)
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite, bot.ScopeTransactionsRead)
	s, err := usecase.HouseholdScopeOf(f.deps, ctx, asBot(b), "h1")
	noErr(t, err)
	noErr(t, usecase.RequireScope(s, bot.ScopeTransactionsWrite))

	_, err = usecase.NewUpdateMemberRole(f.deps).Do(ctx, human(f.owner), "h1", "member", membership.RoleViewer)
	noErr(t, err)
	s, err = usecase.HouseholdScopeOf(f.deps, ctx, asBot(b), "h1")
	noErr(t, err)
	wantForbidden(t, usecase.RequireScope(s, bot.ScopeTransactionsWrite), domain.ForbiddenScope)
	noErr(t, usecase.RequireScope(s, bot.ScopeTransactionsRead))
}

func TestWalletAllowed(t *testing.T) {
	f := newFamily(t)
	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	b.WalletIDs = []domain.ID{"w1"}
	f.must(f.Bots.Update(ctx, b))
	s, err := usecase.HouseholdScopeOf(f.deps, ctx, asBot(b), "h1")
	noErr(t, err)
	if !s.WalletAllowed("w1") || s.WalletAllowed("w2") {
		t.Error("wallet list not applied")
	}
	unlimited := f.addBot("b2", "h1", f.member, bot.ScopeWalletsRead)
	s, err = usecase.HouseholdScopeOf(f.deps, ctx, asBot(unlimited), "h1")
	noErr(t, err)
	if !s.WalletAllowed("w2") {
		t.Error("empty list limits")
	}
	s, err = usecase.HouseholdScopeOf(f.deps, ctx, human(f.viewer), "h1")
	noErr(t, err)
	if !s.WalletAllowed("anything") || s.ActorUserID(human(f.viewer)) != "viewer" {
		t.Error("person limited")
	}
}

// TestBotsMayNotAdminister is FR-033: every administrative use case
// refuses a bot, whatever its scopes, before it looks at anything.
func TestBotsMayNotAdminister(t *testing.T) {
	f := newFamily(t)
	b := asBot(f.addBot("b1", "h1", f.owner, bot.Scopes...))
	d := f.deps
	inv := usecase.InvitationConfig{TTL: ttl}
	calls := map[string]func() error{
		"create household": func() error {
			_, err := usecase.NewCreateHousehold(d).Do(ctx, b, usecase.CreateHouseholdInput{})
			return err
		},
		"update household": func() error {
			_, err := usecase.NewUpdateHousehold(d).Do(ctx, b, "h1", usecase.UpdateHouseholdInput{})
			return err
		},
		"delete household": func() error { return usecase.NewDeleteHousehold(d).Do(ctx, b, "h1") },
		"list members": func() error {
			_, err := usecase.NewListMembers(d).Do(ctx, b, "h1")
			return err
		},
		"change role": func() error {
			_, err := usecase.NewUpdateMemberRole(d).Do(ctx, b, "h1", "member", membership.RoleViewer)
			return err
		},
		"remove member": func() error { return usecase.NewRemoveMember(d).Do(ctx, b, "h1", "member") },
		"invite": func() error {
			_, err := usecase.NewCreateInvitation(d, inv).Do(ctx, b, "h1", usecase.CreateInvitationInput{})
			return err
		},
		"list invitations": func() error {
			_, err := usecase.NewListInvitations(d).Do(ctx, b, "h1")
			return err
		},
		"revoke invitation": func() error { return usecase.NewRevokeInvitation(d).Do(ctx, b, "h1", "i1") },
		"accept invitation": func() error {
			_, err := usecase.NewAcceptInvitation(d).Do(ctx, b, "t")
			return err
		},
		"create bot": func() error {
			_, err := usecase.NewCreateBot(d).Do(ctx, b, "h1", usecase.CreateBotInput{})
			return err
		},
		"list bots": func() error {
			_, err := usecase.NewListBots(d).Do(ctx, b, "h1")
			return err
		},
		"get bot": func() error {
			_, err := usecase.NewGetBot(d).Do(ctx, b, "h1", "b1")
			return err
		},
		"update bot": func() error {
			_, err := usecase.NewUpdateBot(d).Do(ctx, b, "h1", "b1", usecase.UpdateBotInput{})
			return err
		},
		"rotate bot": func() error {
			_, err := usecase.NewRotateBotToken(d).Do(ctx, b, "h1", "b1")
			return err
		},
		"delete bot": func() error { return usecase.NewDeleteBot(d).Do(ctx, b, "h1", "b1") },
		"audit": func() error {
			_, err := usecase.NewListAudit(d).Do(ctx, b, "h1", usecase.ListAuditInput{})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) { wantForbidden(t, call(), domain.ForbiddenHumanOnly) })
	}
}
