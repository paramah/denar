package usecase_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/payee"
	"github.com/paramah/denar/internal/domain/transaction"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

func record(f *family, p usecase.Principal, in usecase.RecordTransactionInput) (usecase.TransactionView, error) {
	return usecase.NewRecordTransaction(f.deps).Do(ctx, p, "h1", in)
}

func TestRecordTransaction(t *testing.T) {
	f := fxFamily(t)
	v, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-25.99"), Date: day("2026-10-20"),
		PayeeName: "Biedronka", Tags: []string{" Food "}, Note: " zakupy ", ExternalID: "r-1", EnvelopeID: "e1"})
	noErr(t, err)
	tx := v.Transaction
	if tx.Amount != -2599 || tx.Kind != transaction.Expense || tx.Status != transaction.Cleared || tx.Source != transaction.SourceManual ||
		!slices.Equal(tx.Tags, []string{"food"}) || tx.Note != "zakupy" || v.PayeeName != "Biedronka" || tx.PayeeID == "" ||
		tx.CreatedBy.ID != "member" || tx.EnvelopeID != "e1" {
		t.Errorf("tx = %+v (%s)", tx, v.PayeeName)
	}
	if got := f.actions(); !slices.Equal(got, []string{"payee.created", "transaction.created"}) {
		t.Errorf("audit = %v", got)
	}
	// The same name finds the payee; so does a pattern.
	first := tx.PayeeID
	pe, err := f.Payees.ByID(ctx, "h1", first)
	noErr(t, err)
	pe.Patterns, pe.DefaultEnvelopeID = []string{"JMP S.A."}, "groceries"
	f.must(f.Payees.Update(ctx, pe))
	v, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeName: "biedronka"})
	noErr(t, err)
	if v.Transaction.PayeeID != first || v.Transaction.EnvelopeID != "groceries" {
		t.Errorf("by name = %+v", v.Transaction)
	}
	v, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeName: "PLATNOSC JMP S.A. 123"})
	noErr(t, err)
	if v.Transaction.PayeeID != first {
		t.Errorf("by pattern = %+v", v.Transaction)
	}
	// Income gets no default envelope.
	v, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("100"), Kind: "income", PayeeID: string(first)})
	noErr(t, err)
	if v.Transaction.EnvelopeID != "" || v.Transaction.Kind != transaction.Income {
		t.Errorf("income = %+v", v.Transaction)
	}
	payees, _ := f.Payees.ListByHousehold(ctx, "h1")
	if len(payees) != 1 {
		t.Errorf("payees = %v", payees)
	}

	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite, bot.ScopeTransactionsRead)
	v, err = record(f, asBot(b), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-3")})
	noErr(t, err)
	if v.Transaction.Source != transaction.SourceBot || v.Transaction.CreatedBy.Kind != domain.ActorBot {
		t.Errorf("bot tx = %+v", v.Transaction)
	}
}

func TestRecordTransactionRules(t *testing.T) {
	f := fxFamily(t)
	m := human(f.member)
	_, err := record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: usd("-1")})
	wantField(t, err, "amount.currency")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1.234")})
	wantField(t, err, "amount.amount")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeID: "p", PayeeName: "x"})
	wantField(t, err, "payee_name")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeID: "nope"})
	wantField(t, err, "payee_id")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), Kind: "transfer"})
	wantField(t, err, "kind")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("1"), Kind: "income", EnvelopeID: "e"})
	wantField(t, err, "envelope_id")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), Status: "lost"})
	wantField(t, err, "status")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "nope", Amount: pln("-1")})
	wantErr(t, err, domain.ErrNotFound)
	_, err = record(f, human(f.viewer), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1")})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), ExternalID: "x"})
	noErr(t, err)
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), ExternalID: "x"})
	wantConflict(t, err, domain.ConflictDuplicateExternalID)

	f.Audit.AppendErr = errors.New("disk full")
	_, err = record(f, m, usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeName: "Nowy"})
	if err == nil {
		t.Fatal("no error")
	}
	f.Audit.AppendErr = nil
	payees, _ := f.Payees.ListByHousehold(ctx, "h1")
	page, _ := usecase.NewSearchTransactions(f.deps).Do(ctx, m, "h1", usecase.SearchTransactionsInput{})
	if len(payees) != 0 || len(page.Items) != 1 {
		t.Errorf("rolled back badly: %v %v", payees, page.Items)
	}
}

func TestSplits(t *testing.T) {
	f := fxFamily(t)
	f.Rates.Set("USD", "2026-10-20", "3.3333")
	v, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w2", Amount: usd("-3.00"), Date: day("2026-10-20"),
		Splits: []usecase.SplitInput{{EnvelopeID: "e1", Amount: usd("-1.00")}, {EnvelopeID: "e2", Amount: usd("-1.00")}, {Amount: usd("-1.00"), Note: "x"}}})
	noErr(t, err)
	tx := v.Transaction
	var sum int64
	for _, s := range tx.Splits {
		sum += s.BaseAmount
	}
	if len(tx.Splits) != 3 || sum != tx.BaseAmount || tx.BaseAmount != -1000 {
		t.Errorf("splits = %+v, base %d", tx.Splits, tx.BaseAmount)
	}
	_, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w2", Amount: usd("-3.00"),
		Splits: []usecase.SplitInput{{Amount: usd("-1.00")}}})
	wantField(t, err, "splits")
	_, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w2", Amount: usd("-3.00"),
		Splits: []usecase.SplitInput{{Amount: pln("-3.00")}}})
	wantField(t, err, "splits[0].amount.currency")

	// An update replaces the lines and keeps the bases adding up.
	up, err := usecase.NewUpdateTransaction(f.deps).Do(ctx, human(f.member), "h1", tx.ID, usecase.UpdateTransactionInput{
		Amount: domain.Some(usd("-2.00")), Splits: domain.Some([]usecase.SplitInput{{EnvelopeID: "e1", Amount: usd("-0.50")}, {Amount: usd("-1.50")}})})
	noErr(t, err)
	if up.Transaction.BaseAmount != -667 || up.Transaction.Splits[0].BaseAmount+up.Transaction.Splits[1].BaseAmount != -667 {
		t.Errorf("updated = %+v", up.Transaction)
	}
	up, err = usecase.NewUpdateTransaction(f.deps).Do(ctx, human(f.member), "h1", tx.ID, usecase.UpdateTransactionInput{
		Splits: domain.Some([]usecase.SplitInput{}), EnvelopeID: domain.Some("e9")})
	noErr(t, err)
	if len(up.Transaction.Splits) != 0 || up.Transaction.EnvelopeID != "e9" {
		t.Errorf("unsplit = %+v", up.Transaction)
	}
}

func TestUpdateAndDeleteTransaction(t *testing.T) {
	f := fxFamily(t)
	tx := f.spend(human(f.member), "w2", usd("-10.00"), "2026-10-20").Transaction
	uc := usecase.NewUpdateTransaction(f.deps)
	v, err := uc.Do(ctx, human(f.admin), "h1", tx.ID, usecase.UpdateTransactionInput{Date: domain.Some(day("2026-10-16")),
		Status: domain.Some("reconciled"), Note: domain.Some("kino"), PayeeName: domain.Some("Cinema City"), Tags: domain.Some([]string{"fun"})})
	noErr(t, err)
	got := v.Transaction
	if got.BaseAmount != -3900 || got.Status != transaction.Reconciled || got.Note != "kino" || v.PayeeName != "Cinema City" ||
		got.Source != transaction.SourceManual || got.CreatedBy.ID != "member" {
		t.Errorf("updated = %+v", got)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "transaction.updated" || last.Changes["date"].From != "2026-10-20" || last.Changes["base_amount"].To != "-39.00 PLN" {
		t.Errorf("audit = %+v", last)
	}
	v, err = uc.Do(ctx, human(f.admin), "h1", tx.ID, usecase.UpdateTransactionInput{PayeeID: domain.Some("")})
	noErr(t, err)
	if v.Transaction.PayeeID != "" {
		t.Errorf("payee kept: %+v", v.Transaction)
	}
	_, err = uc.Do(ctx, human(f.admin), "h1", tx.ID, usecase.UpdateTransactionInput{Kind: domain.Some("income"), EnvelopeID: domain.Some("e")})
	wantField(t, err, "envelope_id")
	_, err = uc.Do(ctx, human(f.admin), "h1", tx.ID, usecase.UpdateTransactionInput{Amount: domain.Some(pln("1"))})
	wantField(t, err, "amount.currency")
	_, err = uc.Do(ctx, human(f.viewer), "h1", tx.ID, usecase.UpdateTransactionInput{Note: domain.Some("x")})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.admin), "h1", "nope", usecase.UpdateTransactionInput{Note: domain.Some("x")})
	wantErr(t, err, domain.ErrNotFound)

	del := usecase.NewDeleteTransaction(f.deps)
	wantForbidden(t, del.Do(ctx, human(f.viewer), "h1", tx.ID), domain.ForbiddenRole)
	noErr(t, del.Do(ctx, human(f.member), "h1", tx.ID))
	_, err = usecase.NewGetTransaction(f.deps).Do(ctx, human(f.member), "h1", tx.ID)
	wantErr(t, err, domain.ErrNotFound)
	last = f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "transaction.deleted" || last.Changes["amount"].From != "-10.00 USD" {
		t.Errorf("audit = %+v", last)
	}
	wantErr(t, del.Do(ctx, human(f.member), "h1", tx.ID), domain.ErrNotFound)
}

func TestSearchTransactions(t *testing.T) {
	f := fxFamily(t)
	f.addWallet("wp", "h1", f.owner, wallet.Private)
	f.spend(human(f.owner), "wp", pln("-99"), "2026-10-19")
	for i, d := range []string{"2026-10-01", "2026-10-02", "2026-10-03"} {
		_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), Date: day(d),
			Note: []string{"chleb", "mleko", "Chleb razowy"}[i], Tags: []string{"food"}})
		noErr(t, err)
	}
	uc := usecase.NewSearchTransactions(f.deps)
	page, err := uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{})
	noErr(t, err)
	if len(page.Items) != 3 {
		t.Errorf("member sees %d (not the owner's private wallet)", len(page.Items))
	}
	page, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{Text: "chleb", Limit: domain.Some(1)})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Transaction.Note != "Chleb razowy" || page.NextCursor == "" {
		t.Errorf("page 1 = %+v", page)
	}
	page, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{Text: "chleb", Limit: domain.Some(1), Cursor: page.NextCursor})
	noErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Transaction.Note != "chleb" || page.NextCursor != "" {
		t.Errorf("page 2 = %+v", page)
	}
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{Cursor: "garbage"})
	wantField(t, err, "cursor")
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{WalletID: "wp"})
	wantErr(t, err, domain.ErrNotFound)
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{Kind: "gift", Status: "x", From: day("2026-10-05"), To: day("2026-10-01")})
	wantField(t, err, "kind")
	wantField(t, err, "status")
	wantField(t, err, "to")
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.SearchTransactionsInput{Limit: domain.Some(0)})
	wantField(t, err, "limit")
	page, err = uc.Do(ctx, human(f.viewer), "h1", usecase.SearchTransactionsInput{Tag: "FOOD", From: day("2026-10-02")})
	noErr(t, err)
	if len(page.Items) != 2 {
		t.Errorf("tag+from = %d", len(page.Items))
	}
	_, err = usecase.NewGetTransaction(f.deps).Do(ctx, human(f.member), "h1", page.Items[0].Transaction.ID)
	noErr(t, err)
}

func transfer(f *family, p usecase.Principal, in usecase.CreateTransferInput) (usecase.TransferView, error) {
	return usecase.NewCreateTransfer(f.deps).Do(ctx, p, "h1", in)
}

func TestTransfers(t *testing.T) {
	f := fxFamily(t)
	f.addWallet("w3", "h1", f.owner, wallet.Shared)
	m := human(f.member)
	_, err := transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w2", Amount: pln("400")})
	wantField(t, err, "to_amount")
	_, err = transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w1", Amount: pln("400")})
	wantField(t, err, "to_wallet_id")
	_, err = transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w3", Amount: pln("-4")})
	wantField(t, err, "amount.amount")
	_, err = transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w3", Amount: pln("4"), ToAmount: &usecase.MoneyInput{Amount: "5", Currency: "PLN"}})
	wantField(t, err, "to_amount.amount")
	_, err = transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w3", Amount: pln("4"), EnvelopeID: "e1"})
	wantField(t, err, "envelope_id")

	toUSD := usd("100.00")
	v, err := transfer(f, m, usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "w2", Amount: pln("400.00"),
		ToAmount: &toUSD, Date: day("2026-10-20"), Note: "wymiana"})
	noErr(t, err)
	from, to := v.From.Transaction, v.To.Transaction
	if from.Amount != -40000 || from.BaseAmount != -40000 || to.Amount != 10000 || to.BaseAmount != 40000 ||
		from.TransferPairID == "" || from.TransferPairID != to.TransferPairID || from.TransferWalletID != "w2" ||
		to.TransferWalletID != "w1" || from.Kind != transaction.Transfer || to.Note != "wymiana" {
		t.Errorf("legs = %+v / %+v", from, to)
	}
	if got := f.actions(); !slices.Equal(got[len(got)-2:], []string{"transaction.created", "transaction.created"}) {
		t.Errorf("audit = %v", got)
	}
	ws, err := usecase.NewListWallets(f.deps).Do(ctx, m, "h1", usecase.ListWalletsInput{})
	noErr(t, err)
	if ws[0].Balance.Amount != -40000 || ws[1].Balance.Amount != 10000 || ws[1].Base.Amount != 40000 {
		t.Errorf("balances = %+v", ws)
	}

	// Of a leg only the date (of both), status, note and tags change.
	_, err = usecase.NewUpdateTransaction(f.deps).Do(ctx, m, "h1", from.ID, usecase.UpdateTransactionInput{Amount: domain.Some(pln("1"))})
	wantField(t, err, "amount")
	up, err := usecase.NewUpdateTransaction(f.deps).Do(ctx, m, "h1", from.ID, usecase.UpdateTransactionInput{Date: domain.Some(day("2026-10-16")),
		Note: domain.Some("x")})
	noErr(t, err)
	other, err := f.Transactions.ByID(ctx, "h1", to.ID)
	noErr(t, err)
	if up.Transaction.Date != day("2026-10-16") || other.Date != day("2026-10-16") || other.BaseAmount != 39000 || other.Note != "wymiana" {
		t.Errorf("moved = %+v / %+v", up.Transaction, other)
	}

	// Deleting a leg deletes the transfer.
	noErr(t, usecase.NewDeleteTransaction(f.deps).Do(ctx, m, "h1", to.ID))
	legs, _ := f.Transactions.ByPair(ctx, "h1", from.TransferPairID)
	if len(legs) != 0 {
		t.Errorf("legs left: %v", legs)
	}
}

func TestTransferEnvelopeAndHiddenLegs(t *testing.T) {
	f := fxFamily(t)
	off := f.addWallet("wo", "h1", f.member, wallet.Private)
	off.OnBudget = false
	f.must(f.Wallets.Update(ctx, off))
	v, err := transfer(f, human(f.member), usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "wo", Amount: pln("50"), EnvelopeID: "savings"})
	noErr(t, err)
	if v.From.Transaction.EnvelopeID != "savings" || v.To.Transaction.EnvelopeID != "" {
		t.Errorf("envelope on = %+v / %+v", v.From.Transaction, v.To.Transaction)
	}
	// The owner sees the leg in w1 but not the private wallet of the other.
	got, err := usecase.NewGetTransaction(f.deps).Do(ctx, human(f.owner), "h1", v.From.Transaction.ID)
	noErr(t, err)
	if got.Transaction.TransferWalletID != "" {
		t.Errorf("hidden wallet shown: %+v", got.Transaction)
	}
	wantForbidden(t, usecase.NewDeleteTransaction(f.deps).Do(ctx, human(f.owner), "h1", v.From.Transaction.ID), domain.ForbiddenWallet)
	_, err = usecase.NewUpdateTransaction(f.deps).Do(ctx, human(f.owner), "h1", v.From.Transaction.ID,
		usecase.UpdateTransactionInput{Date: domain.Some(day("2026-10-01"))})
	wantForbidden(t, err, domain.ForbiddenWallet)
	_, err = usecase.NewUpdateTransaction(f.deps).Do(ctx, human(f.owner), "h1", v.From.Transaction.ID,
		usecase.UpdateTransactionInput{Status: domain.Some("reconciled")})
	noErr(t, err)
	_, err = transfer(f, human(f.owner), usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "wo", Amount: pln("5")})
	wantErr(t, err, domain.ErrNotFound)
}

func TestPayees(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addEnvelopes("h1", "groceries")
	create := usecase.NewCreatePayee(f.deps)
	p, err := create.Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: " Lidl ", Patterns: []string{"LIDL SP", " "}, DefaultEnvelopeID: "groceries"})
	noErr(t, err)
	if p.Name != "Lidl" || !slices.Equal(p.Patterns, []string{"LIDL SP"}) || p.DefaultEnvelopeID != "groceries" {
		t.Errorf("payee = %+v", p)
	}
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: "lidl"})
	wantConflict(t, err, domain.ConflictDuplicateName)
	_, err = create.Do(ctx, human(f.viewer), "h1", usecase.CreatePayeeInput{Name: "X"})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: ""})
	wantField(t, err, "name")

	up, err := usecase.NewUpdatePayee(f.deps).Do(ctx, human(f.member), "h1", p.ID, usecase.UpdatePayeeInput{
		Name: domain.Some("Lidl PL"), DefaultEnvelopeID: domain.Some("")})
	noErr(t, err)
	if up.Name != "Lidl PL" || up.DefaultEnvelopeID != "" || !slices.Equal(up.Patterns, []string{"LIDL SP"}) {
		t.Errorf("updated = %+v", up)
	}
	_, err = usecase.NewUpdatePayee(f.deps).Do(ctx, human(f.member), "h1", "nope", usecase.UpdatePayeeInput{})
	wantErr(t, err, domain.ErrNotFound)

	tx, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), PayeeID: string(p.ID)})
	noErr(t, err)
	list, err := usecase.NewListPayees(f.deps).Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if !slices.EqualFunc(list, []payee.Payee{up}, func(a, b payee.Payee) bool { return a.ID == b.ID }) {
		t.Errorf("list = %+v", list)
	}
	noErr(t, usecase.NewDeletePayee(f.deps).Do(ctx, human(f.member), "h1", p.ID))
	got, err := usecase.NewGetTransaction(f.deps).Do(ctx, human(f.member), "h1", tx.Transaction.ID)
	noErr(t, err)
	if got.Transaction.PayeeID != "" || got.PayeeName != "" {
		t.Errorf("tx keeps payee: %+v", got)
	}
	if got := f.actions(); !slices.Contains(got, "payee.deleted") || !slices.Contains(got, "payee.updated") {
		t.Errorf("audit = %v", got)
	}
	wantErr(t, usecase.NewDeletePayee(f.deps).Do(ctx, human(f.member), "h1", p.ID), domain.ErrNotFound)
}
