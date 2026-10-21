package usecase_test

import (
	"fmt"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// subscriptionAuthzFamily is budgetFamily with the owner's private wallet
// "mine" and three subscriptions of the owner: "shared" paid from w1,
// "private" paid from mine and "nowallet" without a wallet. Each has its
// November charge matched with a transaction t-<name> of w2, and the
// transaction t-free of w2 pays nothing.
func subscriptionAuthzFamily(t *testing.T) *family {
	f := budgetFamily(t)
	f.addWallet("mine", "h1", f.owner, wallet.Private)
	for name, walletID := range map[string]string{"shared": "w1", "private": "mine", "nowallet": ""} {
		v, err := usecase.NewCreateSubscription(f.deps).Do(ctx, human(f.owner), "h1", usecase.CreateSubscriptionInput{Name: name,
			Vendor: "other", Price: usd("20.00"), Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-31")}, WalletID: walletID})
		f.must(err)
		f.vars[name] = v.Subscription.ID
		tx := f.addTx(domain.ID("t-"+name), "2026-11-30")
		c := f.charge(v.Subscription.ID, "2026-11-30")
		c.Status, c.TransactionID = subscription.Matched, tx.ID
		f.must(f.Charges.Update(ctx, c))
	}
	f.addTx("t-free", "2026-10-31")
	return f
}

func (f *family) addTx(id domain.ID, date string) transaction.Transaction {
	f.t.Helper()
	now := f.Clock.Now()
	t := transaction.Transaction{ID: id, HouseholdID: "h1", WalletID: "w2", Date: day(date), Amount: -2000, Currency: "USD",
		BaseAmount: -8000, Kind: transaction.Expense, Status: transaction.Cleared, Source: transaction.SourceImport,
		Tags: []string{}, Splits: []transaction.Split{}, CreatedBy: domain.Actor{Kind: domain.ActorUser, ID: "owner"}, CreatedAt: now, UpdatedAt: now}
	f.must(f.Transactions.Create(ctx, t))
	return t
}

// TestSubscriptionAuthorizationMatrix is FR-320 over role × kind of
// principal × bot scope × bot wallet list × the subscription's wallet
// (shared, the owner's private one, none): a person reads with any role
// and writes as member or above; a subscription with a wallet exists only
// for those who reach the wallet, one without for the whole household; a
// bot needs every scope of the operation and its creator's role. Scopes
// come before visibility: a missing scope is 403 even for a hidden one.
func TestSubscriptionAuthorizationMatrix(t *testing.T) {
	read, write := bot.ScopeSubscriptionsRead, bot.ScopeSubscriptionsWrite
	ops := []struct {
		name     string
		write    bool
		specific bool // acts on the target subscription, so its visibility counts
		scopes   []bot.Scope
		call     func(f *family, p usecase.Principal, target string) error
	}{
		{"list", false, false, []bot.Scope{read}, func(f *family, p usecase.Principal, _ string) error {
			_, err := usecase.NewListSubscriptions(f.deps).Do(ctx, p, "h1", usecase.ListSubscriptionsInput{})
			return err
		}},
		{"summary", false, false, []bot.Scope{read}, func(f *family, p usecase.Principal, _ string) error {
			_, err := usecase.NewGetSubscriptionSummary(f.deps).Do(ctx, p, "h1")
			return err
		}},
		{"upcoming", false, false, []bot.Scope{read}, func(f *family, p usecase.Principal, _ string) error {
			_, err := usecase.NewListUpcomingRenewals(f.deps).Do(ctx, p, "h1", domain.Optional[int]{})
			return err
		}},
		{"list charges", false, false, []bot.Scope{read}, func(f *family, p usecase.Principal, _ string) error {
			_, err := usecase.NewListCharges(f.deps).Do(ctx, p, "h1", usecase.ListChargesInput{})
			return err
		}},
		{"charges for a transaction", false, false, []bot.Scope{read, bot.ScopeTransactionsRead}, func(f *family, p usecase.Principal, _ string) error {
			_, err := usecase.NewSuggestCharges(f.deps).Do(ctx, p, "h1", "t-free")
			return err
		}},
		{"get", false, true, []bot.Scope{read}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewGetSubscription(f.deps).Do(ctx, p, "h1", f.vars[target])
			return err
		}},
		{"prices", false, true, []bot.Scope{read}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewListSubscriptionPrices(f.deps).Do(ctx, p, "h1", f.vars[target])
			return err
		}},
		{"get charge", false, true, []bot.Scope{read}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewGetCharge(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-10-31").ID)
			return err
		}},
		{"suggestions", false, true, []bot.Scope{read, bot.ScopeTransactionsRead}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewSuggestTransactions(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-10-31").ID, domain.Optional[int]{})
			return err
		}},
		{"create", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			w, _ := f.Subscriptions.ByID(ctx, "h1", f.vars[target])
			_, err := usecase.NewCreateSubscription(f.deps).Do(ctx, p, "h1", usecase.CreateSubscriptionInput{Name: "Nowa", Vendor: "other",
				Price: pln("1"), Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-01-01")}, WalletID: string(w.WalletID)})
			return err
		}},
		{"update", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewUpdateSubscription(f.deps).Do(ctx, p, "h1", f.vars[target], usecase.UpdateSubscriptionInput{Note: domain.Some("x")})
			return err
		}},
		{"change price", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewChangeSubscriptionPrice(f.deps).Do(ctx, p, "h1", f.vars[target], usecase.ChangeSubscriptionPriceInput{Price: usd("21")})
			return err
		}},
		{"delete", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			return usecase.NewDeleteSubscription(f.deps).Do(ctx, p, "h1", f.vars[target])
		}},
		{"skip", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewSkipCharge(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-10-31").ID)
			return err
		}},
		{"unmatch", true, true, []bot.Scope{write}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewUnmatchCharge(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-11-30").ID)
			return err
		}},
		{"confirm", true, true, []bot.Scope{write, bot.ScopeTransactionsRead}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewConfirmChargeMatch(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-10-31").ID, "t-free")
			return err
		}},
		{"post", true, true, []bot.Scope{write, bot.ScopeTransactionsWrite}, func(f *family, p usecase.Principal, target string) error {
			_, err := usecase.NewPostCharge(f.deps).Do(ctx, p, "h1", f.charge(f.vars[target], "2026-12-31").ID, usecase.PostChargeInput{WalletID: "w2"})
			return err
		}},
	}
	visible := func(target string, creator membership.Role, list string) bool {
		switch target {
		case "shared":
			return list != "other"
		case "private":
			return creator == membership.RoleOwner && list != "other"
		}
		return true
	}
	expect := func(op string, write, specific bool, scopeOK bool, target string, role membership.Role, list string) outcome {
		switch {
		case !scopeOK || (write && role == membership.RoleViewer):
			return forbidden
		case specific && !visible(target, role, list):
			return notFound
		}
		return allowed
	}
	targets := []string{"shared", "private", "nowallet"}
	for _, op := range ops {
		for _, target := range targets {
			if !op.specific && target != "shared" {
				continue
			}
			for _, role := range membership.Roles {
				t.Run(fmt.Sprintf("%s %s by %s", op.name, target, role), func(t *testing.T) {
					f := subscriptionAuthzFamily(t)
					want := expect(op.name, op.write, op.specific, true, target, role, "none")
					if got := outcomeOf(op.call(f, human(f.byRole(role)), target)); got != want {
						t.Errorf("got %s, want %s", got, want)
					}
				})
				for _, granted := range []bool{true, false} {
					for _, list := range []string{"none", "other"} {
						t.Run(fmt.Sprintf("%s %s by bot of %s granted=%v list=%s", op.name, target, role, granted, list), func(t *testing.T) {
							f := subscriptionAuthzFamily(t)
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
							want := expect(op.name, op.write, op.specific, granted, target, role, list)
							if got := outcomeOf(op.call(f, asBot(b), target)); got != want {
								t.Errorf("got %s, want %s", got, want)
							}
						})
					}
				}
			}
		}
		t.Run(op.name+" by outsider", func(t *testing.T) {
			f := subscriptionAuthzFamily(t)
			if got := outcomeOf(op.call(f, human(f.outsider), "nowallet")); got != notFound {
				t.Errorf("got %s, want not found", got)
			}
		})
	}
}

// TestBotSeesOnlySubscriptionsOfItsWallets: lists leave out what a bot's
// wallet list hides, and keep what has no wallet (AD-403).
func TestBotSeesOnlySubscriptionsOfItsWallets(t *testing.T) {
	f := subscriptionAuthzFamily(t)
	b := f.addBot("b1", "h1", f.owner, bot.ScopeSubscriptionsRead)
	b.WalletIDs = []domain.ID{"w2"}
	f.must(f.Bots.Update(ctx, b))
	list, err := usecase.NewListSubscriptions(f.deps).Do(ctx, asBot(b), "h1", usecase.ListSubscriptionsInput{})
	noErr(t, err)
	if len(list) != 1 || list[0].Subscription.Name != "nowallet" {
		t.Errorf("bot list = %+v", list)
	}
	page, err := usecase.NewListCharges(f.deps).Do(ctx, asBot(b), "h1", usecase.ListChargesInput{})
	noErr(t, err)
	for _, c := range page.Items {
		if c.Charge.SubscriptionID != f.vars["nowallet"] {
			t.Errorf("bot sees charge of %s", c.SubscriptionName)
		}
	}
	all, err := usecase.NewListSubscriptions(f.deps).Do(ctx, human(f.admin), "h1", usecase.ListSubscriptionsInput{})
	noErr(t, err)
	if len(all) != 2 {
		t.Errorf("admin sees %d, want shared and nowallet", len(all))
	}
}
