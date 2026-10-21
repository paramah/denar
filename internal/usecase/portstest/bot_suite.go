package portstest

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
)

func newBot(env Env, id, householdID, createdBy domain.ID, name string) bot.Bot {
	return bot.Bot{
		ID: id, HouseholdID: householdID, Name: name, Scopes: []bot.Scope{bot.ScopeWalletsRead, bot.ScopeTransactionsWrite},
		WalletIDs: []domain.ID{}, CreatedBy: createdBy, CreatedAt: now(env),
		TokenHash: "hash-" + string(id), TokenPrefix: "dnr_" + string(id),
	}
}

func wantBot(t *testing.T, got, want bot.Bot) {
	t.Helper()
	for _, pair := range [][2]time.Time{{got.CreatedAt, want.CreatedAt}, {got.ExpiresAt, want.ExpiresAt}, {got.LastUsedAt, want.LastUsedAt}} {
		if !pair[0].Equal(pair[1]) {
			t.Errorf("time = %v, want %v", pair[0], pair[1])
		}
	}
	if !slices.Equal(got.Scopes, want.Scopes) {
		t.Errorf("Scopes = %v, want %v", got.Scopes, want.Scopes)
	}
	if !slices.Equal(got.WalletIDs, want.WalletIDs) || got.WalletIDs == nil {
		t.Errorf("WalletIDs = %#v, want %v", got.WalletIDs, want.WalletIDs)
	}
	got.CreatedAt, got.ExpiresAt, got.LastUsedAt = want.CreatedAt, want.ExpiresAt, want.LastUsedAt
	got.Scopes, got.WalletIDs = nil, nil
	want.Scopes, want.WalletIDs = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bot = %+v, want %+v", got, want)
	}
}

func botIDs(bots []bot.Bot) []domain.ID {
	out := make([]domain.ID, len(bots))
	for i, b := range bots {
		out[i] = b.ID
	}
	return out
}

// RunBotSuite checks a BotRepository. It needs Users and Households.
func RunBotSuite(t *testing.T, factory Factory) {
	ctx := context.Background()

	t.Run("Create then read back by id and token hash", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		b := newBot(env, "b1", "h1", "u1", "Claude")
		b.WalletIDs = []domain.ID{"w2", "w1"}
		b.ExpiresAt = now(env).Add(90 * 24 * time.Hour)
		plain := newBot(env, "b2", "h1", "u1", "Plain")
		noError(t, env.Bots.Create(ctx, b))
		noError(t, env.Bots.Create(ctx, plain))

		got, err := env.Bots.ByID(ctx, "b1")
		noError(t, err)
		wantBot(t, got, b)
		got, err = env.Bots.ByTokenHash(ctx, "hash-b2")
		noError(t, err)
		wantBot(t, got, plain)
		_, err = env.Bots.ByTokenHash(ctx, "")
		wantError(t, err, domain.ErrNotFound)
		_, err = env.Bots.ByID(ctx, "nope")
		wantError(t, err, domain.ErrNotFound)
	})

	t.Run("names are unique per household, ignoring case", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b1", "h1", "u1", "Claude")))
		wantConflict(t, env.Bots.Create(ctx, newBot(env, "b2", "h1", "u1", "claude")), domain.ConflictDuplicateName)
		noError(t, env.Bots.Create(ctx, newBot(env, "b3", "h2", "u1", "Claude")))
		renamed := newBot(env, "b4", "h1", "u1", "Other")
		noError(t, env.Bots.Create(ctx, renamed))
		renamed.Name = "CLAUDE"
		wantConflict(t, env.Bots.Update(ctx, renamed), domain.ConflictDuplicateName)
	})

	t.Run("Update stores the mutable fields", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		b := newBot(env, "b1", "h1", "u1", "Claude")
		noError(t, env.Bots.Create(ctx, b))
		b.Name, b.Scopes, b.WalletIDs = "Renamed", []bot.Scope{bot.ScopeReportsRead}, []domain.ID{"w9"}
		b.ExpiresAt, b.Disabled, b.TokenHash, b.TokenPrefix = now(env).Add(time.Hour), true, "hash-new", "dnr_new"
		noError(t, env.Bots.Update(ctx, b))
		got, err := env.Bots.ByID(ctx, "b1")
		noError(t, err)
		wantBot(t, got, b)
		_, err = env.Bots.ByTokenHash(ctx, "hash-b1")
		wantError(t, err, domain.ErrNotFound)

		b.WalletIDs, b.ExpiresAt = nil, time.Time{}
		noError(t, env.Bots.Update(ctx, b))
		got, err = env.Bots.ByID(ctx, "b1")
		noError(t, err)
		b.WalletIDs = []domain.ID{}
		wantBot(t, got, b)
		wantError(t, env.Bots.Update(ctx, newBot(env, "nope", "h1", "u1", "X")), domain.ErrNotFound)
	})

	t.Run("ListByHousehold is by name; Delete removes", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b1", "h1", "u1", "zeta")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b2", "h1", "u1", "Alpha")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b3", "h2", "u1", "beta")))
		list, err := env.Bots.ListByHousehold(ctx, "h1")
		noError(t, err)
		sameOrder(t, "ListByHousehold", botIDs(list), []domain.ID{"b2", "b1"})

		noError(t, env.Bots.Delete(ctx, "b2"))
		_, err = env.Bots.ByID(ctx, "b2")
		wantError(t, err, domain.ErrNotFound)
		wantError(t, env.Bots.Delete(ctx, "b2"), domain.ErrNotFound)
	})

	t.Run("DisableByCreator disables that creator's bots in that household", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env, "u2")
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b1", "h1", "u2", "a")))
		already := newBot(env, "b2", "h1", "u2", "b")
		already.Disabled = true
		noError(t, env.Bots.Create(ctx, already))
		noError(t, env.Bots.Create(ctx, newBot(env, "b3", "h1", "u1", "c")))
		noError(t, env.Bots.Create(ctx, newBot(env, "b4", "h2", "u2", "d")))

		changed, err := env.Bots.DisableByCreator(ctx, "h1", "u2")
		noError(t, err)
		sameSet(t, "DisableByCreator", botIDs(changed), []domain.ID{"b1"})
		for id, want := range map[domain.ID]bool{"b1": true, "b2": true, "b3": false, "b4": false} {
			got, err := env.Bots.ByID(ctx, id)
			noError(t, err)
			if got.Disabled != want {
				t.Errorf("%s disabled = %v, want %v", id, got.Disabled, want)
			}
		}
	})

	t.Run("TouchUsed stores the last use", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Bots.Create(ctx, newBot(env, "b1", "h1", "u1", "a")))
		at := now(env).Add(time.Minute)
		noError(t, env.Bots.TouchUsed(ctx, "b1", at))
		got, err := env.Bots.ByID(ctx, "b1")
		noError(t, err)
		if !got.LastUsedAt.Equal(at) {
			t.Errorf("LastUsedAt = %v, want %v", got.LastUsedAt, at)
		}
		wantError(t, env.Bots.TouchUsed(ctx, "nope", at), domain.ErrNotFound)
	})
}
