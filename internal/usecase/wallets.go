package usecase

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/money"
	"git.eiac.dev/tools/denar/internal/domain/transaction"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// MoneyInput is an amount as clients write it: a decimal string and the
// currency it is in.
type MoneyInput struct {
	Amount   string
	Currency string
}

// parseMoney reads an amount that must be in the currency want (FR-101).
func parseMoney(field string, in MoneyInput, want string) (money.Money, error) {
	if in.Currency != want {
		return money.Money{}, domain.ValidationError{Field: field + ".currency", Message: "must be " + want + ", the currency of the wallet"}
	}
	return money.Parse(field+".amount", in.Amount, want)
}

// OpeningNote is the note of the transaction a wallet opens with.
const OpeningNote = "Saldo otwarcia"

// WalletView is a wallet with its balances: in its own currency, the part
// the bank has confirmed, and in the household's base currency.
type WalletView struct {
	Wallet  wallet.Wallet
	Balance money.Money
	Cleared money.Money
	Base    money.Money
	// BasePending: some transactions wait for a rate, so Base is partial.
	BasePending bool
}

func walletView(w wallet.Wallet, b WalletBalance, baseCode string) WalletView {
	cur, _ := money.LookupCurrency(w.Currency)
	base, _ := money.LookupCurrency(baseCode)
	return WalletView{Wallet: w, Balance: money.Money{Amount: b.Total, Currency: cur},
		Cleared: money.Money{Amount: b.Cleared, Currency: cur}, Base: money.Money{Amount: b.Base, Currency: base},
		BasePending: b.Pending > 0}
}

func (d Deps) walletViews(ctx context.Context, s HouseholdScope, ws []wallet.Wallet) ([]WalletView, error) {
	balances, err := d.Transactions.Balances(ctx, s.Household.ID, walletIDs(ws))
	if err != nil {
		return nil, err
	}
	out := make([]WalletView, len(ws))
	for i, w := range ws {
		out[i] = walletView(w, balances[w.ID], s.Household.BaseCurrency)
	}
	return out, nil
}

// checkMembers checks that the people on a private wallet's list are
// members of the household.
func (d Deps) checkMembers(ctx context.Context, householdID domain.ID, ids []domain.ID) error {
	for _, id := range ids {
		_, err := d.Memberships.Get(ctx, householdID, id)
		if found, err := ignoreNotFound(err); err != nil {
			return err
		} else if !found {
			return domain.ValidationError{Field: "member_ids", Message: "contains someone who is not a member of the household: " + string(id)}
		}
	}
	return nil
}

func toIDs(in []string) []domain.ID {
	out := make([]domain.ID, len(in))
	for i, s := range in {
		out[i] = domain.ID(strings.TrimSpace(s))
	}
	return out
}

func walletChanges(w wallet.Wallet) audit.Changes {
	return audit.Changes{}.Set("name", w.Name).Set("type", string(w.Type)).Set("currency", w.Currency).
		Set("on_budget", w.OnBudget).Set("visibility", string(w.Visibility)).Set("member_ids", joinIDs(w.MemberIDs))
}

type CreateWalletInput struct {
	Name       string
	Type       string
	Currency   string
	OnBudget   bool
	Visibility string // "" means shared
	MemberIDs  []string
	// OpeningBalance, when set and not zero, becomes the wallet's first
	// transaction, dated OpenedOn (today when zero).
	OpeningBalance *MoneyInput
	OpenedOn       domain.Date
}

// CreateWallet adds a wallet (FR-120). A bot with wallets:write may add
// shared wallets only.
type CreateWallet struct{ d Deps }

func NewCreateWallet(d Deps) *CreateWallet { return &CreateWallet{d: d} }

func (uc *CreateWallet) Do(ctx context.Context, p Principal, householdID domain.ID, in CreateWalletInput) (WalletView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return WalletView{}, err
	}
	if err := requireScope(s, bot.ScopeWalletsWrite); err != nil {
		return WalletView{}, err
	}
	vis := wallet.Visibility(in.Visibility)
	if vis == "" {
		vis = wallet.Shared
	}
	if p.IsBot() && vis != wallet.Shared {
		return WalletView{}, errHumanOnly
	}
	w := wallet.Wallet{
		ID: uc.d.IDs.NewID(), HouseholdID: householdID, Name: in.Name, Type: wallet.Type(in.Type), Currency: in.Currency,
		OnBudget: in.OnBudget, Visibility: vis, MemberIDs: toIDs(in.MemberIDs), CreatedBy: s.ActorUserID(p), CreatedAt: uc.d.Clock.Now(),
	}.Normalize()
	errs := []error{w.Validate()}
	var opening *transaction.Transaction
	if in.OpeningBalance != nil && w.Validate() == nil {
		m, err := parseMoney("opening_balance", *in.OpeningBalance, w.Currency)
		errs = append(errs, err)
		if err == nil && !m.IsZero() {
			date := in.OpenedOn
			if date.IsZero() {
				date = today(s.Household, uc.d.Clock.Now())
			}
			t := transaction.Transaction{ID: uc.d.IDs.NewID(), HouseholdID: householdID, WalletID: w.ID, Date: date,
				Amount: m.Amount, Currency: w.Currency, Kind: transaction.Income, Status: transaction.Reconciled,
				Source: sourceOf(p), Note: OpeningNote, Tags: []string{}, CreatedBy: p.Actor(), CreatedAt: w.CreatedAt, UpdatedAt: w.CreatedAt}
			if t, err = uc.d.fx().withBase(ctx, s.Household, t); err != nil {
				return WalletView{}, err
			}
			opening = &t
		}
	}
	if err := errors.Join(errs...); err != nil {
		return WalletView{}, err
	}
	var view WalletView
	err = uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeWalletsWrite); err != nil {
			return err
		}
		if err := uc.d.checkMembers(ctx, householdID, w.MemberIDs); err != nil {
			return err
		}
		if err := uc.d.Wallets.Create(ctx, w); err != nil {
			return err
		}
		if err := uc.d.record(ctx, householdID, p.Actor(), "wallet.created", audit.ResourceWallet, w.ID, walletChanges(w)); err != nil {
			return err
		}
		if opening != nil {
			if err := insertTransaction(ctx, uc.d, s.Household.BaseCurrency, *opening, p.Actor()); err != nil {
				return err
			}
		}
		views, err := uc.d.walletViews(ctx, s, []wallet.Wallet{w})
		if err != nil {
			return err
		}
		view = views[0]
		return nil
	})
	return view, err
}

type ListWalletsInput struct {
	IncludeArchived bool
}

// ListWallets returns the wallets the principal sees, by name, with their
// balances (FR-121).
type ListWallets struct{ d Deps }

func NewListWallets(d Deps) *ListWallets { return &ListWallets{d: d} }

func (uc *ListWallets) Do(ctx context.Context, p Principal, householdID domain.ID, in ListWalletsInput) ([]WalletView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return nil, err
	}
	if err := requireScope(s, bot.ScopeWalletsRead); err != nil {
		return nil, err
	}
	ws, err := uc.d.visibleWallets(ctx, s, p)
	if err != nil {
		return nil, err
	}
	if !in.IncludeArchived {
		ws = slices.DeleteFunc(ws, wallet.Wallet.Archived)
	}
	return uc.d.walletViews(ctx, s, ws)
}

// GetWallet returns one wallet the principal sees.
type GetWallet struct{ d Deps }

func NewGetWallet(d Deps) *GetWallet { return &GetWallet{d: d} }

func (uc *GetWallet) Do(ctx context.Context, p Principal, householdID, walletID domain.ID) (WalletView, error) {
	s, err := uc.d.access().household(ctx, p, householdID)
	if err != nil {
		return WalletView{}, err
	}
	if err := requireScope(s, bot.ScopeWalletsRead); err != nil {
		return WalletView{}, err
	}
	w, err := uc.d.visibleWallet(ctx, s, p, walletID)
	if err != nil {
		return WalletView{}, err
	}
	views, err := uc.d.walletViews(ctx, s, []wallet.Wallet{w})
	if err != nil {
		return WalletView{}, err
	}
	return views[0], nil
}

type UpdateWalletInput struct {
	Name       domain.Optional[string]
	Type       domain.Optional[string]
	OnBudget   domain.Optional[bool]
	Visibility domain.Optional[string]
	MemberIDs  domain.Optional[[]string]
	Archived   domain.Optional[bool]
}

// UpdateWallet changes a wallet the principal sees. Who sees it — the
// visibility and the member list — only a person changes: the creator, or
// an owner or admin while the wallet is shared (D: AD-110).
type UpdateWallet struct{ d Deps }

func NewUpdateWallet(d Deps) *UpdateWallet { return &UpdateWallet{d: d} }

func (uc *UpdateWallet) Do(ctx context.Context, p Principal, householdID, walletID domain.ID, in UpdateWalletInput) (WalletView, error) {
	var view WalletView
	err := uc.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := uc.d.access().household(ctx, p, householdID)
		if err != nil {
			return err
		}
		if err := requireScope(s, bot.ScopeWalletsWrite); err != nil {
			return err
		}
		old, err := uc.d.visibleWallet(ctx, s, p, walletID)
		if err != nil {
			return err
		}
		w := old
		if in.Visibility.Set || in.MemberIDs.Set {
			if p.IsBot() {
				return errHumanOnly
			}
			if old.CreatedBy != p.User.ID && !(old.Visibility == wallet.Shared && s.Role.AtLeast(membership.RoleAdmin)) {
				return domain.ForbiddenError{Reason: domain.ForbiddenRole}
			}
			if in.Visibility.Set {
				w.Visibility = wallet.Visibility(in.Visibility.Value)
			}
			if in.MemberIDs.Set {
				w.MemberIDs = toIDs(in.MemberIDs.Value)
			}
		}
		if in.Name.Set {
			w.Name = in.Name.Value
		}
		if in.Type.Set {
			w.Type = wallet.Type(in.Type.Value)
		}
		if in.OnBudget.Set {
			w.OnBudget = in.OnBudget.Value
		}
		if in.Archived.Set && in.Archived.Value != old.Archived() {
			w.ArchivedAt = zeroOrNow(in.Archived.Value, uc.d.Clock)
		}
		w = w.Normalize()
		if err := w.Validate(); err != nil {
			return err
		}
		if err := uc.d.checkMembers(ctx, householdID, w.MemberIDs); err != nil {
			return err
		}
		changes := audit.Changes{}.Diff("name", old.Name, w.Name).Diff("type", string(old.Type), string(w.Type)).
			Diff("on_budget", old.OnBudget, w.OnBudget).Diff("visibility", string(old.Visibility), string(w.Visibility)).
			Diff("member_ids", joinIDs(old.MemberIDs), joinIDs(w.MemberIDs)).Diff("archived", old.Archived(), w.Archived())
		if len(changes) > 0 {
			if err := uc.d.Wallets.Update(ctx, w); err != nil {
				return err
			}
			if err := uc.d.record(ctx, householdID, p.Actor(), "wallet.updated", audit.ResourceWallet, w.ID, changes); err != nil {
				return err
			}
		}
		views, err := uc.d.walletViews(ctx, s, []wallet.Wallet{w})
		if err != nil {
			return err
		}
		view = views[0]
		return nil
	})
	return view, err
}

func zeroOrNow(on bool, c Clock) time.Time {
	if on {
		return c.Now()
	}
	return time.Time{}
}
