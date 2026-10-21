package usecase_test

import (
	"fmt"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// spendingWorld: a shared card w1, a shared cash wallet w2, a private
// wallet of the owner and a USD wallet without a rate; October's expenses
// on them and some that do not count.
func spendingWorld(t *testing.T) *family {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWallet("w2", "h1", f.owner, wallet.Shared)
	f.addWallet("w-priv", "h1", f.owner, wallet.Private)
	f.addWalletIn("USD", "w-usd", "h1", f.owner, wallet.Shared)
	f.addEnvelopes("h1", "food", "fun")
	rec := usecase.NewRecordTransaction(f.deps)
	add := func(p usecase.Principal, in usecase.RecordTransactionInput) domain.ID {
		t.Helper()
		v, err := rec.Do(ctx, p, "h1", in)
		noErr(t, err)
		return v.Transaction.ID
	}
	m := human(f.member)
	add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-10-02"), Amount: pln("-50.00"), PayeeName: "Biedronka", EnvelopeID: "food"})
	add(m, usecase.RecordTransactionInput{WalletID: "w2", Date: day("2026-10-05"), Amount: pln("-30.00"), PayeeName: "Lidl", EnvelopeID: "food"})
	// A refund lowers the spending.
	add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-10-06"), Amount: pln("10.00"), PayeeName: "Biedronka", EnvelopeID: "food"})
	// A split counts by its lines.
	add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-10-07"), Amount: pln("-100.00"), PayeeName: "Biedronka",
		Splits: []usecase.SplitInput{{EnvelopeID: "food", Amount: pln("-60.00")}, {EnvelopeID: "fun", Amount: pln("-40.00")}}})
	// Without an envelope.
	f.vars["netflix-tx"] = add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-10-20"), Amount: pln("-43.00"), PayeeName: "Netflix"})
	// Income, a transfer, September and the private wallet do not count
	// for the member; the USD expense waits for a rate.
	add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-10-01"), Amount: pln("5000.00"), Kind: "income"})
	_, err := usecase.NewCreateTransfer(f.deps).Do(ctx, m, "h1", usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w2",
		Date: day("2026-10-03"), Amount: pln("200.00")})
	noErr(t, err)
	add(m, usecase.RecordTransactionInput{WalletID: "w1", Date: day("2026-09-30"), Amount: pln("-77.00"), EnvelopeID: "food"})
	add(human(f.owner), usecase.RecordTransactionInput{WalletID: "w-priv", Date: day("2026-10-04"), Amount: pln("-999.00"), EnvelopeID: "fun"})
	add(m, usecase.RecordTransactionInput{WalletID: "w-usd", Date: day("2026-10-08"), Amount: usd("-5.00"), EnvelopeID: "fun"})

	// The Netflix payment pays a subscription's charge.
	sub, err := usecase.NewCreateSubscription(f.deps).Do(ctx, m, "h1", usecase.CreateSubscriptionInput{Name: "Netflix", Vendor: "netflix",
		Price: pln("43.00"), Cycle: usecase.CycleInput{Unit: "month", Anchor: day("2026-10-20")}, WalletID: "w1"})
	noErr(t, err)
	page, err := usecase.NewListCharges(f.deps).Do(ctx, m, "h1", usecase.ListChargesInput{SubscriptionID: string(sub.Subscription.ID)})
	noErr(t, err)
	_, err = usecase.NewConfirmChargeMatch(f.deps).Do(ctx, m, "h1", page.Items[0].Charge.ID, f.vars["netflix-tx"])
	noErr(t, err)
	return f
}

// groups writes the groups as "key=name:total/count".
func groups(r usecase.SpendingReport) string {
	parts := make([]string, len(r.Groups))
	for i, g := range r.Groups {
		parts[i] = fmt.Sprintf("%s=%s:%d/%d", g.Key, g.Name, g.Total, g.Count)
	}
	return strings.Join(parts, " ")
}

func TestSpendingReport(t *testing.T) {
	f := spendingWorld(t)
	report := usecase.NewSpendingReport(f.deps)
	m := human(f.member)
	october := usecase.SpendingReportInput{From: day("2026-10-01"), To: day("2026-10-31")}

	for _, tc := range []struct {
		groupBy, want string
	}{
		{"envelope", "food=food:13000/4 =:4300/1 fun=fun:4000/1"},
		{"payee", "=Biedronka:14000/3 =Netflix:4300/1 =Lidl:3000/1"},
		{"wallet", "w1=Wallet w1:18300/4 w2=Wallet w2:3000/1"},
		{"vendor", "=:17000/4 netflix=Netflix:4300/1"},
	} {
		in := october
		in.GroupBy = tc.groupBy
		r, err := report.Do(ctx, m, "h1", in)
		noErr(t, err)
		got := groups(r)
		if tc.groupBy == "payee" {
			// Payee ids are generated; compare names and sums.
			got = stripKeys(r)
		}
		if got != tc.want {
			t.Errorf("%s: groups = %s, want %s", tc.groupBy, got, tc.want)
		}
		if r.Total != 21300 || r.Count != 5 || r.Currency != "PLN" || r.GroupBy != tc.groupBy {
			t.Errorf("%s: total %d count %d currency %s", tc.groupBy, r.Total, r.Count, r.Currency)
		}
		if !r.Incomplete || r.PendingTransactions != 1 || r.Truncated {
			t.Errorf("%s: pending %d incomplete %v truncated %v", tc.groupBy, r.PendingTransactions, r.Incomplete, r.Truncated)
		}
	}

	// The owner sees the private wallet too.
	r, err := report.Do(ctx, human(f.owner), "h1", usecase.SpendingReportInput{GroupBy: "envelope", From: day("2026-10-01"), To: day("2026-10-31")})
	noErr(t, err)
	if r.Total != 21300+99900 {
		t.Errorf("owner's total = %d", r.Total)
	}

	// One wallet.
	r, err = report.Do(ctx, m, "h1", usecase.SpendingReportInput{GroupBy: "envelope", From: day("2026-10-01"), To: day("2026-10-31"), WalletID: "w2"})
	noErr(t, err)
	if groups(r) != "food=food:3000/1" {
		t.Errorf("w2 = %s", groups(r))
	}
	_, err = report.Do(ctx, m, "h1", usecase.SpendingReportInput{GroupBy: "envelope", WalletID: "w-priv"})
	wantErr(t, err, domain.ErrNotFound)

	// By default: the current budget month up to today (21 October).
	r, err = report.Do(ctx, m, "h1", usecase.SpendingReportInput{GroupBy: "wallet"})
	noErr(t, err)
	if r.From.String() != "2026-10-01" || r.To.String() != "2026-10-21" || r.Total != 21300 {
		t.Errorf("default range %s..%s total %d", r.From, r.To, r.Total)
	}
	// Only from given: up to today; September alone.
	r, err = report.Do(ctx, m, "h1", usecase.SpendingReportInput{GroupBy: "wallet", From: day("2026-09-01"), To: day("2026-09-30")})
	noErr(t, err)
	if r.Total != 7700 || r.Count != 1 {
		t.Errorf("september = %d/%d", r.Total, r.Count)
	}

	// Invalid input.
	for _, in := range []usecase.SpendingReportInput{
		{GroupBy: "category"},
		{GroupBy: "envelope", From: day("2026-10-10"), To: day("2026-10-01")},
		{GroupBy: "envelope", From: day("2025-01-01"), To: day("2026-10-01")},
	} {
		_, err := report.Do(ctx, m, "h1", in)
		wantErr(t, err, domain.ErrInvalid)
	}
}

func stripKeys(r usecase.SpendingReport) string {
	parts := make([]string, len(r.Groups))
	for i, g := range r.Groups {
		parts[i] = fmt.Sprintf("=%s:%d/%d", g.Name, g.Total, g.Count)
	}
	return strings.Join(parts, " ")
}

func TestSpendingReportTruncates(t *testing.T) {
	f := spendingWorld(t)
	restore := usecase.SetSpendingScanLimit(3)
	defer restore()
	r, err := usecase.NewSpendingReport(f.deps).Do(ctx, human(f.member), "h1", usecase.SpendingReportInput{GroupBy: "wallet",
		From: day("2026-10-01"), To: day("2026-10-31")})
	noErr(t, err)
	if !r.Truncated || r.Count > 3 {
		t.Errorf("report = %+v", r)
	}
}

// The matrix of who may read the report and over which wallets.
func TestSpendingReportAuthorization(t *testing.T) {
	f := spendingWorld(t)
	report := usecase.NewSpendingReport(f.deps)
	in := usecase.SpendingReportInput{GroupBy: "wallet", From: day("2026-10-01"), To: day("2026-10-31")}
	for _, role := range []membership.Role{membership.RoleOwner, membership.RoleAdmin, membership.RoleMember, membership.RoleViewer} {
		creator := f.byRole(role)
		_, err := report.Do(ctx, human(creator), "h1", in)
		noErr(t, err)
		for i, scopes := range [][]bot.Scope{{bot.ScopeReportsRead}, {bot.ScopeTransactionsRead, bot.ScopeWalletsRead}} {
			for _, limited := range []bool{false, true} {
				b := f.addBot(domain.ID(fmt.Sprintf("b-%s-%d-%v", role, i, limited)), "h1", creator, scopes...)
				if limited {
					b.WalletIDs = []domain.ID{"w2", "w-priv"}
					f.must(f.Bots.Update(ctx, b))
				}
				r, err := report.Do(ctx, asBot(b), "h1", in)
				if i == 1 {
					wantForbidden(t, err, domain.ForbiddenScope)
					continue
				}
				noErr(t, err)
				want := int64(21300)
				switch {
				case limited && role == membership.RoleOwner:
					want = 3000 + 99900 // w2 and the owner's private wallet
				case limited:
					want = 3000
				case role == membership.RoleOwner:
					want += 99900
				}
				if r.Total != want {
					t.Errorf("%s bot limited=%v: total %d, want %d", role, limited, r.Total, want)
				}
				// A wallet off its list is not found.
				_, err = report.Do(ctx, asBot(b), "h1", usecase.SpendingReportInput{GroupBy: "wallet", WalletID: "w1"})
				if limited {
					wantErr(t, err, domain.ErrNotFound)
				} else {
					noErr(t, err)
				}
			}
		}
	}
	_, err := report.Do(ctx, human(f.outsider), "h1", in)
	wantErr(t, err, domain.ErrNotFound)
	other := f.addBot("b-out", "h2", f.outsider, bot.ScopeReportsRead)
	_, err = report.Do(ctx, asBot(other), "h1", in)
	wantErr(t, err, domain.ErrNotFound)
}
