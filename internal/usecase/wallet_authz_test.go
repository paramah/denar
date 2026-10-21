package usecase_test

import (
	"errors"
	"fmt"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// outcome of an operation in the matrix.
type outcome string

const (
	allowed   outcome = "allowed"
	forbidden outcome = "forbidden"
	notFound  outcome = "not found"
)

func outcomeOf(err error) outcome {
	switch {
	case err == nil:
		return allowed
	case errors.Is(err, domain.ErrForbidden):
		return forbidden
	case errors.Is(err, domain.ErrNotFound):
		return notFound
	}
	return outcome("error: " + err.Error())
}

// TestWalletAuthorizationMatrix is FR-150 over role × kind of principal ×
// bot scope × bot wallet list × wallet visibility, for reading and for
// writing: a person reads with any role and writes as member or above; a
// private wallet exists only for its members, whatever their role; a bot
// needs the scope, its creator's role, a wallet its creator sees and, with
// a list, a wallet on it. Scopes are checked before wallets, so a missing
// scope is 403 even for a hidden wallet; a hidden wallet is 404.
func TestWalletAuthorizationMatrix(t *testing.T) {
	type walletCase struct {
		id      domain.ID
		visible func(f *family, who membership.Role) bool
	}
	setup := func(t *testing.T) *family {
		f := newFamily(t)
		f.addWallet("shared", "h1", f.owner, wallet.Shared)
		f.addWallet("owners", "h1", f.owner, wallet.Private)
		f.addWallet("members", "h1", f.member, wallet.Private, "viewer")
		for _, id := range []string{"shared", "owners", "members"} {
			w, _ := f.Wallets.ByID(ctx, "h1", domain.ID(id))
			tx := f.spend(human(f.byRole(roleSeeing(w))), id, pln("-1"), "2026-10-20")
			f.vars[id] = tx.Transaction.ID
		}
		return f
	}
	wallets := []domain.ID{"shared", "owners", "members"}
	seen := func(id domain.ID, who membership.Role) bool {
		switch id {
		case "owners":
			return who == membership.RoleOwner
		case "members":
			return who == membership.RoleMember || who == membership.RoleViewer
		}
		return true
	}
	ops := []struct {
		name  string
		write bool
		scope bot.Scope
		call  func(f *family, p usecase.Principal, w domain.ID) error
	}{
		{"get wallet", false, bot.ScopeWalletsRead, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := usecase.NewGetWallet(f.deps).Do(ctx, p, "h1", w)
			return err
		}},
		{"update wallet", true, bot.ScopeWalletsWrite, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := usecase.NewUpdateWallet(f.deps).Do(ctx, p, "h1", w, usecase.UpdateWalletInput{Name: domain.Some("Renamed " + string(w))})
			return err
		}},
		{"get transaction", false, bot.ScopeTransactionsRead, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := usecase.NewGetTransaction(f.deps).Do(ctx, p, "h1", f.vars[string(w)])
			return err
		}},
		{"search wallet", false, bot.ScopeTransactionsRead, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := usecase.NewSearchTransactions(f.deps).Do(ctx, p, "h1", usecase.SearchTransactionsInput{WalletID: string(w)})
			return err
		}},
		{"record", true, bot.ScopeTransactionsWrite, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := record(f, p, usecase.RecordTransactionInput{WalletID: string(w), Amount: pln("-1")})
			return err
		}},
		{"update transaction", true, bot.ScopeTransactionsWrite, func(f *family, p usecase.Principal, w domain.ID) error {
			_, err := usecase.NewUpdateTransaction(f.deps).Do(ctx, p, "h1", f.vars[string(w)], usecase.UpdateTransactionInput{Note: domain.Some("x")})
			return err
		}},
		{"delete transaction", true, bot.ScopeTransactionsWrite, func(f *family, p usecase.Principal, w domain.ID) error {
			return usecase.NewDeleteTransaction(f.deps).Do(ctx, p, "h1", f.vars[string(w)])
		}},
	}
	for _, op := range ops {
		for _, w := range wallets {
			for _, role := range membership.Roles {
				t.Run(fmt.Sprintf("%s %s by %s", op.name, w, role), func(t *testing.T) {
					f := setup(t)
					want := allowed
					switch {
					case op.write && role == membership.RoleViewer:
						want = forbidden
					case !seen(w, role):
						want = notFound
					}
					if got := outcomeOf(op.call(f, human(f.byRole(role)), w)); got != want {
						t.Errorf("got %s, want %s", got, want)
					}
				})
				for _, granted := range []bool{true, false} {
					for _, list := range []string{"none", "this", "other"} {
						t.Run(fmt.Sprintf("%s %s by bot of %s granted=%v list=%s", op.name, w, role, granted, list), func(t *testing.T) {
							f := setup(t)
							scopes := []bot.Scope{bot.ScopeReportsRead}
							if granted {
								scopes = append(scopes, op.scope)
							}
							b := f.addBot("b1", "h1", f.byRole(role), scopes...)
							switch list {
							case "this":
								b.WalletIDs = []domain.ID{w}
							case "other":
								b.WalletIDs = []domain.ID{"elsewhere"}
							}
							f.must(f.Bots.Update(ctx, b))
							want := allowed
							switch {
							case !granted || (op.write && role == membership.RoleViewer):
								want = forbidden
							case !seen(w, role) || list == "other":
								want = notFound
							}
							if got := outcomeOf(op.call(f, asBot(b), w)); got != want {
								t.Errorf("got %s, want %s", got, want)
							}
						})
					}
				}
			}
			t.Run(fmt.Sprintf("%s %s by outsider", op.name, w), func(t *testing.T) {
				f := setup(t)
				if got := outcomeOf(op.call(f, human(f.outsider), w)); got != notFound {
					t.Errorf("got %s, want not found", got)
				}
			})
		}
	}
}

// roleSeeing is a role whose holder sees the wallet and may write to it.
func roleSeeing(w wallet.Wallet) membership.Role {
	if w.CreatedBy == "member" {
		return membership.RoleMember
	}
	return membership.RoleOwner
}

// TestBotIsReadAgain is FR-151: a bot disabled, deleted or given a new
// token after its request was authenticated is refused.
func TestBotIsReadAgain(t *testing.T) {
	for name, change := range map[string]func(f *family, b *bot.Bot){
		"disabled": func(f *family, b *bot.Bot) { b.Disabled = true; f.must(f.Bots.Update(ctx, *b)) },
		"expired":  func(f *family, b *bot.Bot) { b.ExpiresAt = f.Clock.Now(); f.must(f.Bots.Update(ctx, *b)) },
		"rotated": func(f *family, b *bot.Bot) {
			c := *b
			c.TokenHash = "other"
			f.must(f.Bots.Update(ctx, c))
		},
		"deleted": func(f *family, b *bot.Bot) { f.must(f.Bots.Delete(ctx, b.ID)) },
	} {
		t.Run(name, func(t *testing.T) {
			f := fxFamily(t)
			b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite)
			authenticated := b
			change(f, &b)
			_, err := record(f, asBot(authenticated), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1")})
			wantErr(t, err, domain.ErrUnauthenticated)
		})
	}
}
