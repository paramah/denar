package usecase_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/imports"
	"git.eiac.dev/tools/denar/internal/domain/subscription"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func row(n int, date, amount, payee string) usecase.ImportedTransaction {
	return usecase.ImportedTransaction{Row: n, Date: day(date), Amount: amount, PayeeRaw: payee}
}

func (f *family) importRows(p usecase.Principal, walletID string, rows ...usecase.ImportedTransaction) (usecase.ImportOutcome, error) {
	return usecase.NewImportTransactions(f.deps).Do(ctx, p, "h1", usecase.ImportInput{WalletID: walletID, Source: imports.SourceCSV,
		SourceRef: "wyciag.csv", Preset: "generic", Rows: rows})
}

// walletTxs returns the wallet's transactions, oldest id first.
func (f *family) walletTxs(walletID domain.ID) []transaction.Transaction {
	f.t.Helper()
	page, _, err := f.Transactions.List(ctx, usecase.TransactionFilter{HouseholdID: "h1", WalletIDs: []domain.ID{walletID}},
		usecase.Page{Limit: 200})
	f.must(err)
	slices.Reverse(page)
	return page
}

func TestImportTransactions(t *testing.T) {
	f := budgetFamily(t)
	_, err := usecase.NewCreatePayee(f.deps).Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: "Lidl",
		Patterns: []string{"LIDL SP"}, DefaultEnvelopeID: string(f.vars["food"])})
	noErr(t, err)
	rows := []usecase.ImportedTransaction{
		row(2, "2026-10-01", "-43.21", "LIDL SP. Z O.O. WARSZAWA"),
		{Row: 3, Date: day("2026-10-02"), Amount: "+5000,00", PayeeRaw: "Pracodawca"},
		{Row: 4, Date: day("2026-10-02"), Amount: "5000.00", PayeeRaw: "Pracodawca", Memo: "Wynagrodzenie", ExternalID: "ID-1"},
		{Row: 5, Date: day("2026-10-03"), Amount: "-1.00", Memo: "Opłata", ExternalID: "ID-1"}, // the same id again
		row(6, "2026-10-04", "-12.00", "Kawiarnia"),
		row(7, "2026-10-04", "-12.00", "kawiarnia"), // a second coffee: another occurrence
		row(8, "2026-10-05", "0", "Nic"),
		row(9, "2026-10-05", "-1.234", "Za dużo miejsc"),
		{Row: 10, Date: day("2026-10-05"), Amount: "-1.00", Currency: "EUR", PayeeRaw: "Euro"},
		{Row: 11, Amount: "-1.00", PayeeRaw: "Bez daty"},
		row(12, "2026-10-05", "", "Bez kwoty"),
	}
	out, err := f.importRows(human(f.member), "w1", rows...)
	noErr(t, err)
	b := out.Batch
	if b.ID == "" || b.Rows != 11 || b.Created != 4 || b.Duplicates != 1 || b.Failed != 6 || b.Status != imports.Completed ||
		b.WalletID != "w1" || b.SourceRef != "wyciag.csv" || b.CreatedBy != human(f.member).Actor() {
		t.Fatalf("batch = %+v", b)
	}
	var fields []string
	for _, e := range b.Errors {
		fields = append(fields, fmt.Sprintf("%d:%s", e.Row, e.Field))
	}
	if strings.Join(fields, " ") != "3:amount 8:amount 9:amount 10:currency 11:date 12:amount" {
		t.Errorf("errors = %+v", b.Errors)
	}
	txs := f.walletTxs("w1")
	if len(txs) != 4 {
		t.Fatalf("transactions = %+v", txs)
	}
	lidl := txs[0]
	if lidl.PayeeID == "" || lidl.EnvelopeID != f.vars["food"] || lidl.Kind != transaction.Expense || lidl.Source != transaction.SourceImport ||
		lidl.Status != transaction.Cleared || lidl.Note != "LIDL SP. Z O.O. WARSZAWA" || !strings.HasPrefix(lidl.ExternalID, "h:") ||
		lidl.BaseAmount != -4321 {
		t.Errorf("lidl = %+v", lidl)
	}
	// "+5000,00" is not a decimal amount: the comma is the parser's job.
	salary := txs[1]
	if salary.Kind != transaction.Income || salary.ExternalID != "ID-1" || salary.Note != "Pracodawca · Wynagrodzenie" || salary.EnvelopeID != "" {
		t.Errorf("salary = %+v", salary)
	}
	if txs[2].ExternalID == txs[3].ExternalID || txs[2].PayeeID != "" {
		t.Errorf("coffees = %+v %+v", txs[2], txs[3])
	}
	ids, err := f.ImportBatches.TransactionIDs(ctx, "h1", b.ID)
	noErr(t, err)
	if len(ids) != 4 {
		t.Errorf("batch transactions = %v", ids)
	}
	acts := f.actions()
	if acts[len(acts)-1] != "import.created" || strings.Count(strings.Join(acts, " "), "transaction.created") != 4 {
		t.Errorf("audit = %v", acts)
	}

	// The same rows again: nothing new.
	again, err := f.importRows(human(f.member), "w1", rows...)
	noErr(t, err)
	if again.Batch.Created != 0 || again.Batch.Duplicates != 5 || again.Batch.Failed != 6 {
		t.Errorf("again = %+v", again.Batch)
	}
	if len(f.walletTxs("w1")) != 4 {
		t.Error("the second import added transactions")
	}
}

func TestImportUsesRatesOfTheWallet(t *testing.T) {
	f := budgetFamily(t)
	out, err := f.importRows(human(f.member), "w2", row(1, "2026-10-20", "-10.00", "Amazon"), row(2, "2026-10-21", "-1.00", "Amazon"))
	noErr(t, err)
	if out.Batch.Created != 2 {
		t.Fatalf("batch = %+v", out.Batch)
	}
	txs := f.walletTxs("w2")
	if txs[0].BaseAmount != -4000 || txs[0].RatePending || txs[0].Currency != "USD" {
		t.Errorf("first = %+v", txs[0])
	}
	// No rate for the day, NBP down: it waits, as in stage 002.
	f.Rates.SetDown(true)
	out, err = f.importRows(human(f.member), "w2", row(1, "2026-10-30", "-2.00", "Amazon"))
	noErr(t, err)
	txs = f.walletTxs("w2")
	if last := txs[len(txs)-1]; !last.RatePending {
		t.Errorf("without a rate = %+v", last)
	}
}

func TestImportMatchesSubscriptionCharges(t *testing.T) {
	f := subscriptionFamily(t)
	c := f.charge(f.vars["gpt"], "2026-10-31")
	out, err := f.importRows(human(f.member), "w1", row(1, "2026-10-31", "-80.00", "OPENAI *CHATGPT SUBSCR"),
		row(2, "2026-10-31", "-7.00", "Piekarnia"))
	noErr(t, err)
	if out.Batch.Matched != 1 || len(out.Matches) != 1 || out.Matches[0].ChargeID != c.ID {
		t.Fatalf("outcome = %+v", out)
	}
	if got := f.charge(f.vars["gpt"], "2026-10-31"); got.Status != subscription.Matched || got.TransactionID != out.Matches[0].TransactionID {
		t.Errorf("charge = %+v", got)
	}
	stored, err := f.ImportBatches.ByID(ctx, "h1", out.Batch.ID)
	noErr(t, err)
	if stored.Matched != 1 {
		t.Errorf("stored batch = %+v", stored)
	}

	// A bot that may write transactions only does not touch subscriptions.
	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite)
	out, err = f.importRows(asBot(b), "w1", row(1, "2026-11-30", "-82.00", "OPENAI *CHATGPT SUBSCR"))
	noErr(t, err)
	if out.Batch.Created != 1 || out.Batch.Matched != 0 || f.charge(f.vars["gpt"], "2026-11-30").Status != subscription.Expected {
		t.Errorf("bot import = %+v", out.Batch)
	}
	if txs := f.walletTxs("w1"); txs[len(txs)-1].CreatedBy.Kind != domain.ActorBot || txs[len(txs)-1].Source != transaction.SourceImport {
		t.Errorf("bot transaction = %+v", txs[len(txs)-1])
	}
}

func TestImportDryRun(t *testing.T) {
	f := budgetFamily(t)
	_, err := f.importRows(human(f.member), "w1", usecase.ImportedTransaction{Row: 1, Date: day("2026-10-01"), Amount: "-5.00", ExternalID: "A"})
	noErr(t, err)
	before := len(f.Audit.All())
	rows := []usecase.ImportedTransaction{
		{Row: 1, Date: day("2026-10-01"), Amount: "-5.00", ExternalID: "A"},
		{Row: 2, Date: day("2026-10-02"), Amount: "-6.00", ExternalID: "B"},
		{Row: 3, Date: day("2026-10-03"), Amount: "-7.00", ExternalID: "C"},
		{Row: 4, Date: day("2026-10-03"), Amount: "x"},
	}
	out, err := usecase.NewImportTransactions(f.deps).Do(ctx, human(f.member), "h1", usecase.ImportInput{WalletID: "w1",
		Source: imports.SourceCSV, Rows: rows, DryRun: true, PreviewLimit: 2})
	noErr(t, err)
	if out.Batch.ID != "" || out.Batch.Rows != 4 || out.Batch.Created != 2 || out.Batch.Duplicates != 1 || out.Batch.Failed != 1 {
		t.Errorf("batch = %+v", out.Batch)
	}
	if len(out.Preview) != 2 || out.Preview[0].Status != usecase.ImportRowDuplicate || out.Preview[1].Status != usecase.ImportRowNew ||
		out.Preview[1].Amount != -600 || out.Preview[1].Currency != "PLN" || out.Preview[1].Kind != transaction.Expense {
		t.Errorf("preview = %+v", out.Preview)
	}
	if len(f.walletTxs("w1")) != 1 || len(f.Audit.All()) != before {
		t.Error("a dry run stored something")
	}
}

func TestImportRefusesEmptyAndHugeBatches(t *testing.T) {
	f := budgetFamily(t)
	_, err := f.importRows(human(f.member), "w1")
	wantField(t, err, "file")
	huge := make([]usecase.ImportedTransaction, imports.MaxRows+1)
	_, err = f.importRows(human(f.member), "w1", huge...)
	wantField(t, err, "file")
	// Only row errors still make a batch.
	out, err := usecase.NewImportTransactions(f.deps).Do(ctx, human(f.member), "h1", usecase.ImportInput{WalletID: "w1",
		Source: imports.SourceCSV, RowErrors: []imports.RowError{{Row: 2, Field: "date", Message: "is not a date"}}})
	noErr(t, err)
	if out.Batch.Failed != 1 || out.Batch.Created != 0 {
		t.Errorf("batch = %+v", out.Batch)
	}
}

// The authorization matrix of imports: role × bot scope × wallet list ×
// private wallet × archived wallet.
func TestImportAuthorization(t *testing.T) {
	type tc struct {
		name   string
		setup  func(f *family) usecase.Principal
		wallet string
		want   func(t *testing.T, err error)
	}
	ok := func(t *testing.T, err error) { noErr(t, err) }
	notFound := func(t *testing.T, err error) { wantErr(t, err, domain.ErrNotFound) }
	forbidden := func(r domain.ForbiddenReason) func(t *testing.T, err error) {
		return func(t *testing.T, err error) { wantForbidden(t, err, r) }
	}
	cases := []tc{
		{"owner", func(f *family) usecase.Principal { return human(f.owner) }, "w1", ok},
		{"admin", func(f *family) usecase.Principal { return human(f.admin) }, "w1", ok},
		{"member", func(f *family) usecase.Principal { return human(f.member) }, "w1", ok},
		{"viewer", func(f *family) usecase.Principal { return human(f.viewer) }, "w1", forbidden(domain.ForbiddenRole)},
		{"outsider", func(f *family) usecase.Principal { return human(f.outsider) }, "w1", notFound},
		{"bot with the scope", func(f *family) usecase.Principal {
			return asBot(f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite))
		}, "w1", ok},
		{"bot without the scope", func(f *family) usecase.Principal {
			return asBot(f.addBot("b1", "h1", f.member, bot.ScopeTransactionsRead))
		}, "w1", forbidden(domain.ForbiddenScope)},
		{"bot of a viewer", func(f *family) usecase.Principal {
			return asBot(f.addBot("b1", "h1", f.viewer, bot.ScopeTransactionsWrite))
		}, "w1", forbidden(domain.ForbiddenScope)},
		{"bot limited to another wallet", func(f *family) usecase.Principal {
			b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite)
			b.WalletIDs = []domain.ID{"w2"}
			f.must(f.Bots.Update(ctx, b))
			return asBot(b)
		}, "w1", notFound},
		{"someone else's private wallet", func(f *family) usecase.Principal { return human(f.member) }, "wp", notFound},
		{"the owner's own private wallet", func(f *family) usecase.Principal { return human(f.owner) }, "wp", ok},
		{"archived wallet", func(f *family) usecase.Principal {
			w, _ := f.Wallets.ByID(ctx, "h1", "w1")
			w.ArchivedAt = f.Clock.Now()
			f.must(f.Wallets.Update(ctx, w))
			return human(f.member)
		}, "w1", func(t *testing.T, err error) { wantConflict(t, err, domain.ConflictWalletArchived) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := fxFamily(t)
			f.addWallet("wp", "h1", f.owner, wallet.Private)
			p := c.setup(f)
			_, err := f.importRows(p, c.wallet, row(1, "2026-10-01", "-1.00", "X"))
			c.want(t, err)
			_, err = usecase.NewImportTransactions(f.deps).Do(ctx, p, "h1", usecase.ImportInput{WalletID: c.wallet, Source: imports.SourceCSV,
				Rows: []usecase.ImportedTransaction{row(1, "2026-10-01", "-1.00", "X")}, DryRun: true})
			c.want(t, err)
		})
	}
}
