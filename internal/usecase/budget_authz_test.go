package usecase_test

import (
	"fmt"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// TestBudgetAuthorizationMatrix is FR-230 over role × kind of principal ×
// bot scope × bot wallet list: a person reads the budget with any role and
// changes it as member or above; a bot needs budget:read or budget:write
// and its creator's role; its wallet list does not narrow the budget's
// sums (AD-302). An outsider does not find the household.
func TestBudgetAuthorizationMatrix(t *testing.T) {
	ops := []struct {
		name   string
		write  bool
		scopes []bot.Scope
		call   func(f *family, p usecase.Principal) error
	}{
		{"month", false, []bot.Scope{bot.ScopeBudgetRead}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewGetBudgetMonth(f.deps).Do(ctx, p, "h1", "2026-10")
			return err
		}},
		{"list groups", false, []bot.Scope{bot.ScopeBudgetRead}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewListEnvelopeGroups(f.deps).Do(ctx, p, "h1")
			return err
		}},
		{"list envelopes", false, []bot.Scope{bot.ScopeBudgetRead}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewListEnvelopes(f.deps).Do(ctx, p, "h1", usecase.ListEnvelopesInput{})
			return err
		}},
		{"get envelope", false, []bot.Scope{bot.ScopeBudgetRead}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewGetEnvelope(f.deps).Do(ctx, p, "h1", f.vars["food"])
			return err
		}},
		{"activity", false, []bot.Scope{bot.ScopeBudgetRead, bot.ScopeTransactionsRead}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewListEnvelopeActivity(f.deps).Do(ctx, p, "h1", f.vars["food"], usecase.ListEnvelopeActivityInput{Month: "2026-10"})
			return err
		}},
		{"create group", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewCreateEnvelopeGroup(f.deps).Do(ctx, p, "h1", usecase.CreateEnvelopeGroupInput{Name: "Nowa"})
			return err
		}},
		{"update group", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewUpdateEnvelopeGroup(f.deps).Do(ctx, p, "h1", f.vars["group"], usecase.UpdateEnvelopeGroupInput{Position: domain.Some(4)})
			return err
		}},
		{"delete group", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			return usecase.NewDeleteEnvelopeGroup(f.deps).Do(ctx, p, "h1", f.vars["empty"])
		}},
		{"create envelope", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewCreateEnvelope(f.deps).Do(ctx, p, "h1", usecase.CreateEnvelopeInput{GroupID: string(f.vars["group"]), Name: "Nowa"})
			return err
		}},
		{"update envelope", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewUpdateEnvelope(f.deps).Do(ctx, p, "h1", f.vars["food"], usecase.UpdateEnvelopeInput{Hidden: domain.Some(true)})
			return err
		}},
		{"delete envelope", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			return usecase.NewDeleteEnvelope(f.deps).Do(ctx, p, "h1", f.vars["fun"])
		}},
		{"assign", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewSetAssignment(f.deps).Do(ctx, p, "h1", f.vars["food"], usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("10")})
			return err
		}},
		{"move", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewMoveBetweenEnvelopes(f.deps).Do(ctx, p, "h1", usecase.MoveBetweenEnvelopesInput{Month: "2026-10",
				FromEnvelopeID: string(f.vars["food"]), ToEnvelopeID: string(f.vars["fun"]), Amount: pln("1")})
			return err
		}},
		{"auto-assign", true, []bot.Scope{bot.ScopeBudgetWrite}, func(f *family, p usecase.Principal) error {
			_, err := usecase.NewAutoAssign(f.deps).Do(ctx, p, "h1", "2026-10")
			return err
		}},
	}
	setup := func(t *testing.T) *family {
		f := budgetFamily(t)
		g, err := usecase.NewCreateEnvelopeGroup(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "Pusta"})
		f.must(err)
		f.vars["empty"] = g.ID
		return f
	}
	for _, op := range ops {
		for _, role := range membership.Roles {
			t.Run(fmt.Sprintf("%s by %s", op.name, role), func(t *testing.T) {
				f := setup(t)
				want := allowed
				if op.write && role == membership.RoleViewer {
					want = forbidden
				}
				if got := outcomeOf(op.call(f, human(f.byRole(role)))); got != want {
					t.Errorf("got %s, want %s", got, want)
				}
			})
			for _, granted := range []bool{true, false} {
				for _, list := range []string{"none", "other"} {
					t.Run(fmt.Sprintf("%s by bot of %s granted=%v list=%s", op.name, role, granted, list), func(t *testing.T) {
						f := setup(t)
						scopes := []bot.Scope{bot.ScopeReportsRead}
						if granted {
							scopes = append(scopes, op.scopes...)
						} else if len(op.scopes) > 1 {
							scopes = append(scopes, op.scopes[0]) // one of two is not enough
						}
						b := f.addBot("b1", "h1", f.byRole(role), scopes...)
						if list == "other" {
							b.WalletIDs = []domain.ID{"w2"}
							f.must(f.Bots.Update(ctx, b))
						}
						want := allowed
						if !granted || (op.write && role == membership.RoleViewer) {
							want = forbidden
						}
						if got := outcomeOf(op.call(f, asBot(b))); got != want {
							t.Errorf("got %s, want %s", got, want)
						}
					})
				}
			}
		}
		t.Run(op.name+" by outsider", func(t *testing.T) {
			f := setup(t)
			if got := outcomeOf(op.call(f, human(f.outsider))); got != notFound {
				t.Errorf("got %s, want not found", got)
			}
		})
	}
}
