package usecase

import (
	"context"
	"slices"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/wallet"
)

// visibleWallets returns the household's wallets the principal reaches,
// archived ones included: those its person sees (for a bot, its creator),
// narrowed for a bot to its wallet list (FR-032c).
func (d Deps) visibleWallets(ctx context.Context, s HouseholdScope, p Principal) ([]wallet.Wallet, error) {
	all, err := d.Wallets.ListByHousehold(ctx, s.Household.ID)
	if err != nil {
		return nil, err
	}
	who := s.ActorUserID(p)
	return slices.DeleteFunc(all, func(w wallet.Wallet) bool {
		return !w.VisibleTo(who) || !s.WalletAllowed(w.ID)
	}), nil
}

// visibleWallet returns one wallet the principal reaches; a hidden wallet
// is domain.ErrNotFound, as a missing one.
func (d Deps) visibleWallet(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (wallet.Wallet, error) {
	w, err := d.Wallets.ByID(ctx, s.Household.ID, id)
	if err != nil {
		return wallet.Wallet{}, plainNotFound(err)
	}
	if !w.VisibleTo(s.ActorUserID(p)) || !s.WalletAllowed(w.ID) {
		return wallet.Wallet{}, domain.ErrNotFound
	}
	return w, nil
}

func walletIDs(ws []wallet.Wallet) []domain.ID {
	ids := make([]domain.ID, len(ws))
	for i, w := range ws {
		ids[i] = w.ID
	}
	return ids
}

// validateBotWallets checks that every wallet a bot is to be limited to is
// a wallet of the household its creator sees (FR-150).
func validateBotWallets(ctx context.Context, d Deps, householdID, creatorID domain.ID, ids []domain.ID) error {
	all, err := d.Wallets.ListByHousehold(ctx, householdID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		i := slices.IndexFunc(all, func(w wallet.Wallet) bool { return w.ID == id })
		if i < 0 || !all[i].VisibleTo(creatorID) {
			return domain.ValidationError{Field: "wallet_ids", Message: "contains a wallet that is not one of yours in this household: " + string(id)}
		}
	}
	return nil
}
