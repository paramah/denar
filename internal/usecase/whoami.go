package usecase

import (
	"context"
	"strings"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/budget"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
)

// BotContext is what a bot needs to know to act: itself with its
// effective scopes, its household, the day and budget month there, and
// the wallets it reaches (AD-602).
type BotContext struct {
	Bot       BotView
	Household household.Household
	Today     domain.Date
	Month     budget.Month
	// Wallets the bot reaches, archived ones included, without balances;
	// empty unless it has a wallets or transactions scope.
	Wallets []wallet.Wallet
}

// Whoami describes the bot of the request (MCP whoami). People are
// refused: GET /me describes them.
type Whoami struct{ d Deps }

func NewWhoami(d Deps) *Whoami { return &Whoami{d: d} }

func (uc *Whoami) Do(ctx context.Context, p Principal) (BotContext, error) {
	if !p.IsBot() {
		return BotContext{}, domain.ErrForbidden
	}
	s, err := uc.d.access().household(ctx, p, p.Bot.HouseholdID)
	if err != nil {
		return BotContext{}, err
	}
	today := today(s.Household, uc.d.Clock.Now())
	out := BotContext{Bot: botView(*s.Bot, s.Role), Household: s.Household, Today: today,
		Month: budget.MonthOf(today, s.Household.MonthStartDay), Wallets: []wallet.Wallet{}}
	names := false
	for _, sc := range s.Scopes {
		names = names || strings.HasPrefix(string(sc), "wallets:") || strings.HasPrefix(string(sc), "transactions:")
	}
	if names {
		if out.Wallets, err = uc.d.visibleWallets(ctx, s, p); err != nil {
			return BotContext{}, err
		}
	}
	return out, nil
}
