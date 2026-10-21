package fakes

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// WalletRepository keeps wallets in memory.
type WalletRepository struct {
	store   *Store
	wallets map[domain.ID]wallet.Wallet
}

func NewWalletRepository(s *Store) *WalletRepository {
	r := &WalletRepository{store: s, wallets: map[domain.ID]wallet.Wallet{}}
	s.Register(r)
	return r
}

func (r *WalletRepository) Snapshot() (restore func()) {
	saved := maps.Clone(r.wallets)
	return func() { r.wallets = saved }
}

func (r *WalletRepository) dropHousehold(id domain.ID) {
	maps.DeleteFunc(r.wallets, func(_ domain.ID, w wallet.Wallet) bool { return w.HouseholdID == id })
}

func cloneWallet(w wallet.Wallet) wallet.Wallet {
	w.MemberIDs = slices.Clone(w.MemberIDs)
	if w.MemberIDs == nil {
		w.MemberIDs = []domain.ID{}
	}
	slices.Sort(w.MemberIDs)
	return w
}

func (r *WalletRepository) checkName(w wallet.Wallet) error {
	for _, other := range r.wallets {
		if other.ID != w.ID && other.HouseholdID == w.HouseholdID && strings.EqualFold(other.Name, w.Name) {
			return domain.ConflictError{Reason: domain.ConflictDuplicateName}
		}
	}
	return nil
}

func (r *WalletRepository) Create(ctx context.Context, w wallet.Wallet) (err error) {
	r.store.Do(ctx, func() {
		if _, ok := r.wallets[w.ID]; ok {
			err = domain.ErrConflict
			return
		}
		if err = r.checkName(w); err == nil {
			r.wallets[w.ID] = cloneWallet(w)
		}
	})
	return err
}

func (r *WalletRepository) Update(ctx context.Context, w wallet.Wallet) (err error) {
	r.store.Do(ctx, func() {
		old, ok := r.wallets[w.ID]
		if !ok || old.HouseholdID != w.HouseholdID {
			err = domain.ErrNotFound
			return
		}
		if err = r.checkName(w); err != nil {
			return
		}
		old.Name, old.Type, old.OnBudget, old.Visibility, old.MemberIDs, old.ArchivedAt =
			w.Name, w.Type, w.OnBudget, w.Visibility, w.MemberIDs, w.ArchivedAt
		r.wallets[w.ID] = cloneWallet(old)
	})
	return err
}

func (r *WalletRepository) ByID(ctx context.Context, householdID, id domain.ID) (w wallet.Wallet, err error) {
	r.store.Do(ctx, func() {
		var ok bool
		if w, ok = r.wallets[id]; !ok || w.HouseholdID != householdID {
			w, err = wallet.Wallet{}, domain.ErrNotFound
		}
	})
	return cloneWallet(w), err
}

func (r *WalletRepository) ListByHousehold(ctx context.Context, householdID domain.ID) ([]wallet.Wallet, error) {
	out := []wallet.Wallet{}
	r.store.Do(ctx, func() {
		for _, w := range r.wallets {
			if w.HouseholdID == householdID {
				out = append(out, cloneWallet(w))
			}
		}
	})
	slices.SortFunc(out, func(a, b wallet.Wallet) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}
