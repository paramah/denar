package usecase_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/transaction"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

func TestCreateWalletWithOpeningBalance(t *testing.T) {
	f := newFamily(t)
	opening := pln("1000.00")
	v, err := usecase.NewCreateWallet(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{
		Name: " Konto ", Type: "checking", Currency: "PLN", OnBudget: true, OpeningBalance: &opening})
	noErr(t, err)
	w := v.Wallet
	if w.Name != "Konto" || w.Visibility != wallet.Shared || w.CreatedBy != "member" || v.Balance.Amount != 100000 ||
		v.Base.Amount != 100000 || v.Base.Currency.Code != "PLN" || v.BasePending || v.Cleared.Amount != 100000 {
		t.Errorf("view = %+v", v)
	}
	page, err := usecase.NewSearchTransactions(f.deps).Do(ctx, human(f.viewer), "h1", usecase.SearchTransactionsInput{WalletID: string(w.ID)})
	noErr(t, err)
	if len(page.Items) != 1 {
		t.Fatalf("transactions = %+v", page.Items)
	}
	o := page.Items[0].Transaction
	if o.Kind != transaction.Income || o.Status != transaction.Reconciled || o.Note != usecase.OpeningNote || o.Date != day("2026-10-21") {
		t.Errorf("opening = %+v", o)
	}
	if got := f.actions(); !slices.Equal(got, []string{"wallet.created", "transaction.created"}) {
		t.Errorf("audit = %v", got)
	}

	zero := pln("0")
	v, err = usecase.NewCreateWallet(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{
		Name: "Pusty", Type: "cash", Currency: "PLN", OpeningBalance: &zero})
	noErr(t, err)
	page, err = usecase.NewSearchTransactions(f.deps).Do(ctx, human(f.viewer), "h1", usecase.SearchTransactionsInput{WalletID: string(v.Wallet.ID)})
	noErr(t, err)
	if len(page.Items) != 0 {
		t.Errorf("zero opening made %v", page.Items)
	}
}

func TestCreateWalletRules(t *testing.T) {
	f := newFamily(t)
	uc := usecase.NewCreateWallet(f.deps)
	wrongCurrency := usd("5")
	_, err := uc.Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{Name: "A", Type: "checking", Currency: "PLN", OpeningBalance: &wrongCurrency})
	wantField(t, err, "opening_balance.currency")
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{Name: "", Type: "piggy", Currency: "XYZ"})
	wantField(t, err, "name")
	wantField(t, err, "type")
	wantField(t, err, "currency")
	_, err = uc.Do(ctx, human(f.viewer), "h1", usecase.CreateWalletInput{Name: "A", Type: "cash", Currency: "PLN"})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.outsider), "h1", usecase.CreateWalletInput{Name: "A", Type: "cash", Currency: "PLN"})
	wantErr(t, err, domain.ErrNotFound)

	v, err := uc.Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{Name: "Moje", Type: "cash", Currency: "PLN",
		Visibility: "private", MemberIDs: []string{"viewer"}})
	noErr(t, err)
	if !slices.Equal(v.Wallet.MemberIDs, []domain.ID{"member", "viewer"}) {
		t.Errorf("members = %v", v.Wallet.MemberIDs)
	}
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{Name: "Inne", Type: "cash", Currency: "PLN",
		Visibility: "private", MemberIDs: []string{"outsider"}})
	wantField(t, err, "member_ids")
	_, err = uc.Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{Name: "MOJE", Type: "cash", Currency: "PLN"})
	wantConflict(t, err, domain.ConflictDuplicateName)

	b := f.addBot("b1", "h1", f.member, bot.ScopeWalletsWrite)
	_, err = uc.Do(ctx, asBot(b), "h1", usecase.CreateWalletInput{Name: "Bota", Type: "cash", Currency: "PLN", Visibility: "private"})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	v, err = uc.Do(ctx, asBot(b), "h1", usecase.CreateWalletInput{Name: "Bota", Type: "cash", Currency: "EUR"})
	noErr(t, err)
	if v.Wallet.CreatedBy != "member" {
		t.Errorf("bot's wallet created by %s", v.Wallet.CreatedBy)
	}
	reader := f.addBot("b2", "h1", f.member, bot.ScopeWalletsRead)
	_, err = uc.Do(ctx, asBot(reader), "h1", usecase.CreateWalletInput{Name: "X", Type: "cash", Currency: "EUR"})
	wantForbidden(t, err, domain.ForbiddenScope)
}

func TestListWallets(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWallet("w2", "h1", f.owner, wallet.Private)
	f.addWallet("w3", "h1", f.member, wallet.Private)
	arch := f.addWallet("w4", "h1", f.owner, wallet.Shared)
	arch.ArchivedAt = f.Clock.Now()
	f.must(f.Wallets.Update(ctx, arch))
	f.spend(human(f.member), "w1", pln("-10.50"), "2026-10-20")

	ids := func(vs []usecase.WalletView) []domain.ID {
		var out []domain.ID
		for _, v := range vs {
			out = append(out, v.Wallet.ID)
		}
		return out
	}
	list, err := usecase.NewListWallets(f.deps).Do(ctx, human(f.member), "h1", usecase.ListWalletsInput{})
	noErr(t, err)
	if !slices.Equal(ids(list), []domain.ID{"w1", "w3"}) || list[0].Balance.Amount != -1050 {
		t.Errorf("member sees %v", ids(list))
	}
	list, err = usecase.NewListWallets(f.deps).Do(ctx, human(f.owner), "h1", usecase.ListWalletsInput{IncludeArchived: true})
	noErr(t, err)
	if !slices.Equal(ids(list), []domain.ID{"w1", "w2", "w4"}) {
		t.Errorf("owner sees %v (not the member's private wallet)", ids(list))
	}
	_, err = usecase.NewGetWallet(f.deps).Do(ctx, human(f.owner), "h1", "w3")
	wantErr(t, err, domain.ErrNotFound)
}

func TestUpdateWallet(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.member, wallet.Shared)
	uc := usecase.NewUpdateWallet(f.deps)
	v, err := uc.Do(ctx, human(f.member), "h1", "w1", usecase.UpdateWalletInput{Name: domain.Some("Nowa"), OnBudget: domain.Some(false),
		Archived: domain.Some(true)})
	noErr(t, err)
	if v.Wallet.Name != "Nowa" || v.Wallet.OnBudget || !v.Wallet.Archived() {
		t.Errorf("wallet = %+v", v.Wallet)
	}
	last := f.Audit.All()[len(f.Audit.All())-1]
	if last.Action != "wallet.updated" || last.Changes["name"].To != "Nowa" || last.Changes["archived"].To != true {
		t.Errorf("audit = %+v", last)
	}
	n := len(f.Audit.All())
	_, err = uc.Do(ctx, human(f.member), "h1", "w1", usecase.UpdateWalletInput{Name: domain.Some("Nowa")})
	noErr(t, err)
	if len(f.Audit.All()) != n {
		t.Error("no-op update was recorded")
	}
	// The archive takes no new transactions.
	_, err = usecase.NewRecordTransaction(f.deps).Do(ctx, human(f.member), "h1", usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1")})
	wantConflict(t, err, domain.ConflictWalletArchived)

	// An admin may make a shared wallet private; then only its members see it.
	v, err = uc.Do(ctx, human(f.admin), "h1", "w1", usecase.UpdateWalletInput{Visibility: domain.Some("private"),
		MemberIDs: domain.Some([]string{"admin"})})
	noErr(t, err)
	if !slices.Equal(v.Wallet.MemberIDs, []domain.ID{"admin", "member"}) {
		t.Errorf("members = %v", v.Wallet.MemberIDs)
	}
	_, err = uc.Do(ctx, human(f.owner), "h1", "w1", usecase.UpdateWalletInput{Name: domain.Some("X")})
	wantErr(t, err, domain.ErrNotFound)
	// Of a private wallet only the creator changes who sees it.
	_, err = uc.Do(ctx, human(f.admin), "h1", "w1", usecase.UpdateWalletInput{MemberIDs: domain.Some([]string{})})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.member), "h1", "w1", usecase.UpdateWalletInput{MemberIDs: domain.Some([]string{"outsider"})})
	wantField(t, err, "member_ids")

	f.addWallet("w2", "h1", f.owner, wallet.Shared)
	_, err = uc.Do(ctx, human(f.member), "h1", "w2", usecase.UpdateWalletInput{Visibility: domain.Some("private")})
	wantForbidden(t, err, domain.ForbiddenRole)
	b := f.addBot("b1", "h1", f.owner, bot.ScopeWalletsWrite)
	_, err = uc.Do(ctx, asBot(b), "h1", "w2", usecase.UpdateWalletInput{Visibility: domain.Some("private")})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)
	_, err = uc.Do(ctx, asBot(b), "h1", "w2", usecase.UpdateWalletInput{Name: domain.Some("Przez bota")})
	noErr(t, err)
	_, err = uc.Do(ctx, human(f.viewer), "h1", "w2", usecase.UpdateWalletInput{Name: domain.Some("X")})
	wantForbidden(t, err, domain.ForbiddenRole)
	_, err = uc.Do(ctx, human(f.owner), "h1", "w2", usecase.UpdateWalletInput{Type: domain.Some("piggy")})
	wantField(t, err, "type")
}

func TestWalletWriteIsAtomic(t *testing.T) {
	f := newFamily(t)
	f.Audit.AppendErr = errors.New("disk full")
	opening := pln("10")
	_, err := usecase.NewCreateWallet(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateWalletInput{
		Name: "A", Type: "cash", Currency: "PLN", OpeningBalance: &opening})
	if err == nil {
		t.Fatal("no error")
	}
	list, _ := f.Wallets.ListByHousehold(ctx, "h1")
	if len(list) != 0 {
		t.Errorf("wallet kept: %v", list)
	}
}
