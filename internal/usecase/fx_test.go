package usecase_test

import (
	"slices"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// fxFamily has a PLN wallet w1 and a USD wallet w2 in h1 (base PLN). The
// clock says 2026-10-21; NBP has USD for 2026-10-16 (a Friday) and
// 2026-10-20.
func fxFamily(t *testing.T) *family {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWalletIn("USD", "w2", "h1", f.owner, wallet.Shared)
	f.Rates.Set("USD", "2026-10-16", "3.9000")
	f.Rates.Set("USD", "2026-10-20", "4.0000")
	return f
}

func TestBaseCurrencyNeedsNoRate(t *testing.T) {
	f := fxFamily(t)
	v := f.spend(human(f.member), "w1", pln("-12.34"), "2026-10-20")
	tx := v.Transaction
	if tx.BaseAmount != -1234 || tx.RatePending || tx.FxRate != "" || len(f.Rates.Calls()) != 0 {
		t.Errorf("tx = %+v, calls %v", tx, f.Rates.Calls())
	}
}

func TestRateFromNBPIsCachedForPastDays(t *testing.T) {
	f := fxFamily(t)
	tx := f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-20").Transaction
	if tx.BaseAmount != -4000 || tx.FxRate != "4" || tx.RatePending {
		t.Errorf("tx = %+v", tx)
	}
	f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-20")
	if calls := f.Rates.Calls(); !slices.Equal(calls, []string{"USD/2026-10-20"}) {
		t.Errorf("calls = %v, want one", calls)
	}
	// A weekend falls back to Friday.
	tx = f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-18").Transaction
	if tx.BaseAmount != -390 {
		t.Errorf("weekend base = %d", tx.BaseAmount)
	}
	cached, err := f.FxRates.Get(ctx, "USD", day("2026-10-18"))
	noErr(t, err)
	if cached.RateDate != day("2026-10-16") || cached.Rate != "3.9000" {
		t.Errorf("cache = %+v", cached)
	}
}

func TestTodaysRateIsNotCached(t *testing.T) {
	f := fxFamily(t)
	f.Rates.Set("USD", "2026-10-21", "4.1000")
	f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-21")
	f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-21")
	if n := len(f.Rates.Calls()); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
	_, err := f.FxRates.Get(ctx, "USD", day("2026-10-21"))
	wantErr(t, err, domain.ErrNotFound)
}

func TestOfflineUsesTheNewestCachedRateOrWaits(t *testing.T) {
	f := fxFamily(t)
	f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-17") // caches Friday's rate
	f.Rates.SetDown(true)
	tx := f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-20").Transaction
	if tx.RatePending || tx.BaseAmount != -3900 {
		t.Errorf("offline with cache = %+v", tx)
	}
	tx = f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-10").Transaction
	if !tx.RatePending || tx.BaseAmount != 0 {
		t.Errorf("offline without cache = %+v", tx)
	}
	view, err := usecase.NewGetWallet(f.deps).Do(ctx, human(f.member), "h1", "w2")
	noErr(t, err)
	if !view.BasePending || view.Balance.Amount != -2100 {
		t.Errorf("wallet = %+v", view)
	}

	// Back online, the job fills it in.
	f.Rates.SetDown(false)
	f.Rates.Set("USD", "2026-10-09", "3.8000")
	n, err := usecase.NewFillPendingRates(f.deps).Do(ctx)
	noErr(t, err)
	got, err := f.Transactions.ByID(ctx, "h1", tx.ID)
	noErr(t, err)
	if n != 1 || got.RatePending || got.BaseAmount != -3800 || got.FxRate != "3.8" {
		t.Errorf("filled %d: %+v", n, got)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "transaction.rate_filled" || last.Actor.Kind != domain.ActorSystem || last.ResourceID != tx.ID {
		t.Errorf("audit = %+v", last)
	}
	n, err = usecase.NewFillPendingRates(f.deps).Do(ctx)
	if n != 0 || err != nil {
		t.Errorf("second run = %d, %v", n, err)
	}
}

func TestNoProviderMeansPending(t *testing.T) {
	f := fxFamily(t)
	f.deps.Rates = nil
	tx := f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-20").Transaction
	if !tx.RatePending {
		t.Errorf("tx = %+v", tx)
	}
}

func TestCrossRateThroughPLN(t *testing.T) {
	f := fxFamily(t)
	// A household in EUR: USD → EUR = USD/PLN ÷ EUR/PLN.
	f.must(f.Households.Create(ctx, household.Household{ID: "he", Name: "Euro", BaseCurrency: "EUR", Timezone: "Europe/Berlin",
		MonthStartDay: 1, CreatedBy: f.member.ID, CreatedAt: f.Clock.Now()}))
	f.addMember("he", f.member, membership.RoleOwner)
	f.addWalletIn("USD", "wu", "he", f.member, wallet.Shared)
	f.Rates.Set("EUR", "2026-10-20", "4.3000")
	v, err := usecase.NewRecordTransaction(f.deps).Do(ctx, human(f.member), "he", usecase.RecordTransactionInput{
		WalletID: "wu", Amount: usd("-10.00"), Date: day("2026-10-20")})
	noErr(t, err)
	// -10 × 4 / 4.3 = -9.3023… → -9.30
	if v.Transaction.BaseAmount != -930 || v.Transaction.FxRate != "0.93023256" {
		t.Errorf("cross = %+v", v.Transaction)
	}
	q, err := usecase.NewQuoteFxRate(f.deps).Do(ctx, human(f.member), "he", usecase.QuoteFxRateInput{Currency: "PLN", Date: day("2026-10-20")})
	noErr(t, err)
	if q.Source != usecase.FxNBP || money.FormatRate(q.Rate) != "0.23255814" {
		t.Errorf("PLN in EUR = %+v %s", q, money.FormatRate(q.Rate))
	}
}

func TestQuoteFxRate(t *testing.T) {
	f := fxFamily(t)
	uc := usecase.NewQuoteFxRate(f.deps)
	q, err := uc.Do(ctx, human(f.viewer), "h1", usecase.QuoteFxRateInput{Currency: "USD", Date: day("2026-10-18")})
	noErr(t, err)
	if q.Source != usecase.FxNBP || q.RateDate != day("2026-10-16") || q.BaseCurrency != "PLN" || money.FormatRate(q.Rate) != "3.9" {
		t.Errorf("quote = %+v", q)
	}
	q, err = uc.Do(ctx, human(f.viewer), "h1", usecase.QuoteFxRateInput{Currency: "EUR"})
	noErr(t, err)
	if q.Source != usecase.FxUnavailable || q.Rate != nil || q.Date != day("2026-10-21") {
		t.Errorf("unavailable = %+v", q)
	}
	q, err = uc.Do(ctx, human(f.viewer), "h1", usecase.QuoteFxRateInput{Currency: "PLN"})
	noErr(t, err)
	if q.Source != usecase.FxIdentity {
		t.Errorf("identity = %+v", q)
	}
	_, err = uc.Do(ctx, human(f.viewer), "h1", usecase.QuoteFxRateInput{Currency: "XYZ"})
	wantField(t, err, "currency")
	_, err = uc.Do(ctx, human(f.outsider), "h1", usecase.QuoteFxRateInput{Currency: "USD"})
	wantErr(t, err, domain.ErrNotFound)
	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsRead)
	_, err = uc.Do(ctx, asBot(b), "h1", usecase.QuoteFxRateInput{Currency: "USD"})
	wantForbidden(t, err, domain.ForbiddenScope)
}

func TestSetFxOverride(t *testing.T) {
	f := fxFamily(t)
	f.Rates.SetDown(true)
	waiting := f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-19").Transaction
	other := f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-12").Transaction
	uc := usecase.NewSetFxOverride(f.deps)
	in := usecase.SetFxOverrideInput{Currency: "USD", Date: day("2026-10-19"), Rate: "4.2500"}

	_, err := uc.Do(ctx, human(f.member), "h1", in)
	wantForbidden(t, err, domain.ForbiddenRole)
	b := f.addBot("b1", "h1", f.owner, bot.Scopes...)
	_, err = uc.Do(ctx, asBot(b), "h1", in)
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	_, err = uc.Do(ctx, human(f.outsider), "h1", in)
	wantErr(t, err, domain.ErrNotFound)
	_, err = uc.Do(ctx, human(f.admin), "h1", usecase.SetFxOverrideInput{Currency: "PLN", Date: day("2026-10-19"), Rate: "1"})
	wantField(t, err, "currency")
	_, err = uc.Do(ctx, human(f.admin), "h1", usecase.SetFxOverrideInput{Currency: "USD", Rate: "-1"})
	wantField(t, err, "rate")
	wantField(t, err, "date")

	o, err := uc.Do(ctx, human(f.admin), "h1", in)
	noErr(t, err)
	if o.Rate != "4.25" || o.CreatedBy.ID != "admin" {
		t.Errorf("override = %+v", o)
	}
	got, err := f.Transactions.ByID(ctx, "h1", waiting.ID)
	noErr(t, err)
	if got.RatePending || got.BaseAmount != -4250 {
		t.Errorf("filled = %+v", got)
	}
	got, err = f.Transactions.ByID(ctx, "h1", other.ID)
	noErr(t, err)
	if !got.RatePending {
		t.Errorf("other day filled: %+v", got)
	}
	actions := f.actions()
	if !slices.Contains(actions, "fx_rate.set") || !slices.Contains(actions, "transaction.rate_filled") {
		t.Errorf("audit = %v", actions)
	}
	// New transactions of the day take it, and the quote says so.
	tx := f.spend(human(f.member), "w2", usd("-1.00"), "2026-10-19").Transaction
	if tx.BaseAmount != -425 {
		t.Errorf("new tx = %+v", tx)
	}
	q, err := usecase.NewQuoteFxRate(f.deps).Do(ctx, human(f.viewer), "h1", usecase.QuoteFxRateInput{Currency: "USD", Date: day("2026-10-19")})
	noErr(t, err)
	if q.Source != usecase.FxManual {
		t.Errorf("quote = %+v", q)
	}
	list, err := usecase.NewListFxOverrides(f.deps).Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if len(list) != 1 || list[0].Rate != "4.25" {
		t.Errorf("list = %+v", list)
	}
}
