package usecase

import (
	"context"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/membership"
)

// Test hooks into the authorization helpers.

func HouseholdScopeOf(d Deps, ctx context.Context, p Principal, householdID domain.ID) (HouseholdScope, error) {
	return d.access().household(ctx, p, householdID)
}

func RequireScope(s HouseholdScope, scope bot.Scope) error { return requireScope(s, scope) }

func RequireRole(s HouseholdScope, min membership.Role) error { return requireRole(s, min) }

// FormatMinorForTest writes minor units as "12.34 PLN".
func FormatMinorForTest(amount int64, code string) string { return formatMinor(amount, code) }
