package usecase_test

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

func TestListAndGetImports(t *testing.T) {
	f := budgetFamily(t)
	f.addWallet("wp", "h1", f.owner, wallet.Private)
	first, err := f.importRows(human(f.member), "w1", row(1, "2026-10-01", "-1.00", "A"))
	noErr(t, err)
	second, err := f.importRows(human(f.member), "w2", row(1, "2026-10-01", "-1.00", "A"))
	noErr(t, err)
	private, err := f.importRows(human(f.owner), "wp", row(1, "2026-10-01", "-1.00", "A"))
	noErr(t, err)

	list := usecase.NewListImports(f.deps)
	page, err := list.Do(ctx, human(f.viewer), "h1", usecase.ListImportsInput{})
	noErr(t, err)
	if got := ids(page.Items, func(b imports.Batch) domain.ID { return b.ID }); len(got) != 2 || got[0] != second.Batch.ID || got[1] != first.Batch.ID {
		t.Errorf("viewer sees %v", got)
	}
	page, err = list.Do(ctx, human(f.owner), "h1", usecase.ListImportsInput{Limit: domain.Some(1)})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != private.Batch.ID || page.NextCursor == "" {
		t.Errorf("owner's first page = %+v", page)
	}
	page, err = list.Do(ctx, human(f.owner), "h1", usecase.ListImportsInput{WalletID: "w1"})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != first.Batch.ID {
		t.Errorf("w1 = %+v", page)
	}
	_, err = list.Do(ctx, human(f.member), "h1", usecase.ListImportsInput{WalletID: "wp"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = list.Do(ctx, human(f.member), "h1", usecase.ListImportsInput{Cursor: "garbage"})
	wantField(t, err, "cursor")
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite)
	_, err = list.Do(ctx, asBot(b), "h1", usecase.ListImportsInput{})
	wantForbidden(t, err, domain.ForbiddenScope)

	get := usecase.NewGetImport(f.deps)
	got, err := get.Do(ctx, human(f.viewer), "h1", first.Batch.ID)
	noErr(t, err)
	if got.ID != first.Batch.ID || got.Created != 1 {
		t.Errorf("get = %+v", got)
	}
	_, err = get.Do(ctx, human(f.member), "h1", private.Batch.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = get.Do(ctx, human(f.outsider), "h1", first.Batch.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = get.Do(ctx, human(f.member), "h1", "nope")
	wantErr(t, err, domain.ErrNotFound)
}

func TestRevertImport(t *testing.T) {
	f := subscriptionFamily(t)
	out, err := f.importRows(human(f.member), "w1", row(1, "2026-10-31", "-80.00", "OPENAI *CHATGPT SUBSCR"),
		row(2, "2026-10-30", "-7.00", "Piekarnia"), row(3, "2026-10-29", "-9.00", "Kiosk"))
	noErr(t, err)
	if out.Batch.Matched != 1 {
		t.Fatalf("batch = %+v", out.Batch)
	}
	// One transaction goes by hand before the revert.
	txs := f.walletTxs("w1")
	noErr(t, usecase.NewDeleteTransaction(f.deps).Do(ctx, human(f.member), "h1", txs[0].ID))

	revert := usecase.NewRevertImport(f.deps)
	_, err = revert.Do(ctx, human(f.viewer), "h1", out.Batch.ID)
	wantForbidden(t, err, domain.ForbiddenRole)
	got, err := revert.Do(ctx, human(f.member), "h1", out.Batch.ID)
	noErr(t, err)
	if got.Status != imports.Reverted || got.Reverted != 2 || got.RevertedBy != human(f.member).Actor() || got.RevertedAt.IsZero() {
		t.Errorf("reverted = %+v", got)
	}
	if left := f.walletTxs("w1"); len(left) != 0 {
		t.Errorf("left = %+v", left)
	}
	if c := f.charge(f.vars["gpt"], "2026-10-31"); c.Status != subscription.Expected || c.TransactionID != "" {
		t.Errorf("charge = %+v", c)
	}
	acts := f.actions()
	if acts[len(acts)-1] != "import.reverted" {
		t.Errorf("audit = %v", acts)
	}
	_, err = revert.Do(ctx, human(f.member), "h1", out.Batch.ID)
	wantConflict(t, err, domain.ConflictImportReverted)

	// After the revert the same rows come in again.
	again, err := f.importRows(human(f.member), "w1", row(1, "2026-10-31", "-80.00", "OPENAI *CHATGPT SUBSCR"))
	noErr(t, err)
	if again.Batch.Created != 1 || again.Batch.Matched != 1 {
		t.Errorf("again = %+v", again.Batch)
	}
}

func TestRevertImportRefusals(t *testing.T) {
	f := budgetFamily(t)
	f.addWallet("wp", "h1", f.owner, wallet.Private)
	private, err := f.importRows(human(f.owner), "wp", row(1, "2026-10-01", "-1.00", "A"))
	noErr(t, err)
	shared, err := f.importRows(human(f.owner), "w1", row(1, "2026-10-01", "-1.00", "A"))
	noErr(t, err)
	revert := usecase.NewRevertImport(f.deps)
	_, err = revert.Do(ctx, human(f.member), "h1", private.Batch.ID)
	wantErr(t, err, domain.ErrNotFound)
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsRead)
	_, err = revert.Do(ctx, asBot(b), "h1", shared.Batch.ID)
	wantForbidden(t, err, domain.ForbiddenScope)
	w, _ := f.Wallets.ByID(ctx, "h1", "w1")
	w.ArchivedAt = f.Clock.Now()
	f.must(f.Wallets.Update(ctx, w))
	_, err = revert.Do(ctx, human(f.member), "h1", shared.Batch.ID)
	wantConflict(t, err, domain.ConflictWalletArchived)
	if len(f.walletTxs("w1")) != 1 {
		t.Error("a refused revert deleted transactions")
	}
}
