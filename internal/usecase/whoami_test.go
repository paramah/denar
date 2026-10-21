package usecase_test

import (
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/wallet"
	"git.eiac.dev/tools/denar/internal/usecase"
)

func TestWhoami(t *testing.T) {
	f := newFamily(t)
	f.addWallet("w1", "h1", f.owner, wallet.Shared)
	f.addWallet("w2", "h1", f.owner, wallet.Shared)
	f.addWallet("w-priv", "h1", f.owner, wallet.Private)
	whoami := usecase.NewWhoami(f.deps)

	b := f.addBot("b1", "h1", f.member, bot.ScopeTransactionsWrite, bot.ScopeWalletsWrite, bot.ScopeSubscriptionsRead)
	got, err := whoami.Do(ctx, asBot(b))
	noErr(t, err)
	if got.Bot.Bot.ID != "b1" || got.Household.ID != "h1" || got.Household.BaseCurrency != "PLN" ||
		got.Today.String() != "2026-10-21" || got.Month.String() != "2026-10" {
		t.Fatalf("context = %+v", got)
	}
	if len(got.Bot.EffectiveScopes) != 3 || got.Bot.Bot.TokenHash != "" {
		t.Errorf("bot = %+v", got.Bot)
	}
	// The member does not see the owner's private wallet, nor does the bot.
	if len(got.Wallets) != 2 || got.Wallets[0].ID != "w1" || got.Wallets[1].ID != "w2" {
		t.Errorf("wallets = %+v", got.Wallets)
	}

	// Limited to one wallet.
	b.WalletIDs = []domain.ID{"w2"}
	f.must(f.Bots.Update(ctx, b))
	got, err = whoami.Do(ctx, asBot(b))
	noErr(t, err)
	if len(got.Wallets) != 1 || got.Wallets[0].ID != "w2" {
		t.Errorf("limited wallets = %+v", got.Wallets)
	}

	// Without a wallet or transaction scope no wallet is named.
	only := f.addBot("b2", "h1", f.owner, bot.ScopeSubscriptionsRead)
	got, err = whoami.Do(ctx, asBot(only))
	noErr(t, err)
	if len(got.Wallets) != 0 || got.Wallets == nil {
		t.Errorf("wallets without scopes = %+v", got.Wallets)
	}

	// A viewer's bot keeps only its read scopes.
	v := f.addBot("b3", "h1", f.viewer, bot.ScopeTransactionsWrite, bot.ScopeTransactionsRead)
	got, err = whoami.Do(ctx, asBot(v))
	noErr(t, err)
	if len(got.Bot.EffectiveScopes) != 1 || got.Bot.EffectiveScopes[0] != bot.ScopeTransactionsRead {
		t.Errorf("viewer's bot scopes = %v", got.Bot.EffectiveScopes)
	}

	// People use GET /me.
	_, err = whoami.Do(ctx, human(f.owner))
	wantErr(t, err, domain.ErrForbidden)
}
