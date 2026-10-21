package portstest

import (
	"context"
	"testing"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// RunFxRateSuite checks an FxRateRepository.
func RunFxRateSuite(t *testing.T, factory Factory) {
	ctx := context.Background()
	t.Run("Put, Get, replace and Latest", func(t *testing.T) {
		env := factory(t)
		r := usecase.FxRate{Currency: "USD", Date: day("2026-10-18"), RateDate: day("2026-10-16"), Rate: "4.0100", FetchedAt: now(env)}
		noError(t, env.FxRates.Put(ctx, r))
		got, err := env.FxRates.Get(ctx, "USD", day("2026-10-18"))
		noError(t, err)
		if got.Rate != "4.0100" || got.RateDate != r.RateDate || got.Date != r.Date || !got.FetchedAt.Equal(r.FetchedAt) {
			t.Errorf("rate = %+v", got)
		}
		r.Rate = "4.0200"
		noError(t, env.FxRates.Put(ctx, r))
		got, err = env.FxRates.Get(ctx, "USD", day("2026-10-18"))
		noError(t, err)
		if got.Rate != "4.0200" {
			t.Errorf("replaced rate = %s", got.Rate)
		}
		_, err = env.FxRates.Get(ctx, "EUR", day("2026-10-18"))
		wantError(t, err, domain.ErrNotFound)

		noError(t, env.FxRates.Put(ctx, usecase.FxRate{Currency: "USD", Date: day("2026-10-20"), RateDate: day("2026-10-20"), Rate: "4.1", FetchedAt: now(env)}))
		noError(t, env.FxRates.Put(ctx, usecase.FxRate{Currency: "EUR", Date: day("2026-10-21"), RateDate: day("2026-10-21"), Rate: "4.3", FetchedAt: now(env)}))
		latest, err := env.FxRates.Latest(ctx, "USD", day("2026-10-15"), day("2026-10-21"))
		noError(t, err)
		if latest.Rate != "4.1" {
			t.Errorf("latest = %+v", latest)
		}
		latest, err = env.FxRates.Latest(ctx, "USD", day("2026-10-10"), day("2026-10-17"))
		noError(t, err)
		if latest.Rate != "4.0200" {
			t.Errorf("latest in window = %+v", latest)
		}
		_, err = env.FxRates.Latest(ctx, "USD", day("2026-10-01"), day("2026-10-15"))
		wantError(t, err, domain.ErrNotFound)
	})
}

// RunFxOverrideSuite checks an FxOverrideRepository. It needs Users and
// Households.
func RunFxOverrideSuite(t *testing.T, factory Factory) {
	ctx := context.Background()
	t.Run("Put, Get, replace and List", func(t *testing.T) {
		env := factory(t)
		seedWallets(t, env)
		actor := domain.Actor{Kind: domain.ActorUser, ID: "u1"}
		o := usecase.FxOverride{HouseholdID: "h1", Currency: "USD", Date: day("2026-10-20"), Rate: "4", CreatedBy: actor, CreatedAt: now(env)}
		noError(t, env.FxOverrides.Put(ctx, o))
		o.Rate = "4.25"
		noError(t, env.FxOverrides.Put(ctx, o))
		got, err := env.FxOverrides.Get(ctx, "h1", "USD", day("2026-10-20"))
		noError(t, err)
		if got.Rate != "4.25" || got.CreatedBy != actor || !got.CreatedAt.Equal(o.CreatedAt) || got.Date != o.Date {
			t.Errorf("override = %+v", got)
		}
		_, err = env.FxOverrides.Get(ctx, "h2", "USD", day("2026-10-20"))
		wantError(t, err, domain.ErrNotFound)
		noError(t, env.FxOverrides.Put(ctx, usecase.FxOverride{HouseholdID: "h1", Currency: "EUR", Date: day("2026-10-20"), Rate: "4.3", CreatedBy: actor, CreatedAt: now(env)}))
		noError(t, env.FxOverrides.Put(ctx, usecase.FxOverride{HouseholdID: "h1", Currency: "USD", Date: day("2026-10-21"), Rate: "4.1", CreatedBy: actor, CreatedAt: now(env)}))
		noError(t, env.FxOverrides.Put(ctx, usecase.FxOverride{HouseholdID: "h2", Currency: "USD", Date: day("2026-10-21"), Rate: "9", CreatedBy: actor, CreatedAt: now(env)}))
		list, err := env.FxOverrides.List(ctx, "h1")
		noError(t, err)
		var keys []string
		for _, o := range list {
			keys = append(keys, o.Currency+" "+o.Date.String())
		}
		if len(keys) != 3 || keys[0] != "USD 2026-10-21" || keys[1] != "EUR 2026-10-20" || keys[2] != "USD 2026-10-20" {
			t.Errorf("overrides = %v", keys)
		}
	})
}
