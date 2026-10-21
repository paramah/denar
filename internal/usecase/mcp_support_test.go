package usecase_test

import (
	"errors"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

// A refusal for a missing scope names the scope, for the bot and for a
// person whose role does not permit it (AD-605).
func TestForbiddenNamesTheScope(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsRead)
	_, err := usecase.NewRecordTransaction(f.deps).Do(ctx, asBot(b), "h1", usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1.00")})
	var fe domain.ForbiddenError
	if !errors.As(err, &fe) || fe.Reason != domain.ForbiddenScope || fe.Scope != string(bot.ScopeTransactionsWrite) {
		t.Fatalf("err = %#v", err)
	}
	// A viewer's bot holds the write scope, but its creator's role drops it.
	v := f.addBot("b2", "h1", f.viewer, bot.ScopeTransactionsWrite)
	_, err = usecase.NewRecordTransaction(f.deps).Do(ctx, asBot(v), "h1", usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1.00")})
	if !errors.As(err, &fe) || fe.Reason != domain.ForbiddenScope || fe.Scope != string(bot.ScopeTransactionsWrite) {
		t.Fatalf("viewer's bot: err = %#v", err)
	}
	_, err = usecase.NewRecordTransaction(f.deps).Do(ctx, human(f.viewer), "h1", usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1.00")})
	if !errors.As(err, &fe) || fe.Reason != domain.ForbiddenRole || fe.Scope != string(bot.ScopeTransactionsWrite) {
		t.Fatalf("viewer: err = %#v", err)
	}
}

// An empty month is the household's current budget month (AD-606).
func TestEmptyMonthIsTheCurrentOne(t *testing.T) {
	f := newFamily(t)
	f.addEnvelopes("h1", "food", "fun")
	// 21 October 2026, 09:30 UTC; months start on the 25th, so it is
	// still the budget month of September.
	h, err := f.Households.ByID(ctx, "h1")
	noErr(t, err)
	h.MonthStartDay = 25
	noErr(t, f.Households.Update(ctx, h))

	v, err := usecase.NewGetBudgetMonth(f.deps).Do(ctx, human(f.member), "h1", "")
	noErr(t, err)
	if v.Month.String() != "2026-09" {
		t.Fatalf("month = %s", v.Month)
	}
	v, err = usecase.NewSetAssignment(f.deps).Do(ctx, human(f.member), "h1", "food", usecase.SetAssignmentInput{Amount: pln("100.00")})
	noErr(t, err)
	if v.Month.String() != "2026-09" || v.Assigned != 10000 {
		t.Fatalf("assigned = %+v", v)
	}
	v, err = usecase.NewMoveBetweenEnvelopes(f.deps).Do(ctx, human(f.member), "h1",
		usecase.MoveBetweenEnvelopesInput{FromEnvelopeID: "food", ToEnvelopeID: "fun", Amount: pln("40.00")})
	noErr(t, err)
	if v.Month.String() != "2026-09" {
		t.Fatalf("moved in %s", v.Month)
	}
	// The day the month turns, in the household's time zone.
	f.Clock.Set(time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC)) // 25 October 00:30 in Warsaw
	v, err = usecase.NewGetBudgetMonth(f.deps).Do(ctx, human(f.member), "h1", "")
	noErr(t, err)
	if v.Month.String() != "2026-10" {
		t.Fatalf("month after the turn = %s", v.Month)
	}
}
