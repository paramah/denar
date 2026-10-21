package usecase

import (
	"context"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/subscription"
)

// subscriptionScope opens the household for an operation needing every
// scope given (FR-320). Scopes are checked before any subscription is
// looked at, so a missing scope is 403 even for a hidden one.
func (d Deps) subscriptionScope(ctx context.Context, p Principal, householdID domain.ID, scopes ...bot.Scope) (HouseholdScope, error) {
	s, err := d.access().household(ctx, p, householdID)
	if err != nil {
		return HouseholdScope{}, err
	}
	for _, sc := range scopes {
		if err := requireScope(s, sc); err != nil {
			return HouseholdScope{}, err
		}
	}
	return s, nil
}

// subscriptionVisible: a subscription without a wallet is the household's
// (AD-403); one with a wallet exists for those who reach the wallet.
func (d Deps) subscriptionVisible(ctx context.Context, s HouseholdScope, p Principal, sub subscription.Subscription) (bool, error) {
	if sub.WalletID == "" {
		return true, nil
	}
	_, err := d.visibleWallet(ctx, s, p, sub.WalletID)
	return ignoreNotFound(err)
}

// visibleSubscription returns one subscription the principal sees; a
// hidden one is domain.ErrNotFound, as a missing one.
func (d Deps) visibleSubscription(ctx context.Context, s HouseholdScope, p Principal, id domain.ID) (subscription.Subscription, error) {
	sub, err := d.Subscriptions.ByID(ctx, s.Household.ID, id)
	if err != nil {
		return subscription.Subscription{}, plainNotFound(err)
	}
	ok, err := d.subscriptionVisible(ctx, s, p, sub)
	if err != nil {
		return subscription.Subscription{}, err
	}
	if !ok {
		return subscription.Subscription{}, domain.ErrNotFound
	}
	return sub, nil
}

// visibleSubscriptions returns the household's subscriptions the principal
// sees, by name.
func (d Deps) visibleSubscriptions(ctx context.Context, s HouseholdScope, p Principal) ([]subscription.Subscription, error) {
	all, err := d.Subscriptions.List(ctx, s.Household.ID)
	if err != nil {
		return nil, err
	}
	ws, err := d.visibleWallets(ctx, s, p)
	if err != nil {
		return nil, err
	}
	seen := map[domain.ID]bool{}
	for _, w := range ws {
		seen[w.ID] = true
	}
	out := all[:0:0]
	for _, sub := range all {
		if sub.WalletID == "" || seen[sub.WalletID] {
			out = append(out, sub)
		}
	}
	return out, nil
}

// subscriptionIDs are the ids of the subscriptions.
func subscriptionIDs(ss []subscription.Subscription) []domain.ID {
	out := make([]domain.ID, len(ss))
	for i, s := range ss {
		out[i] = s.ID
	}
	return out
}
