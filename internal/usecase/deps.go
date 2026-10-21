package usecase

import (
	"context"
	"log/slog"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
)

// Deps is what the household, member, invitation, bot and audit use cases
// are built from. The composition root fills it once.
type Deps struct {
	Users       UserRepository
	Households  HouseholdRepository
	Memberships MembershipRepository
	Invitations InvitationRepository
	Bots        BotRepository
	Audit       AuditRepository
	Sessions    SessionRepository
	// Wallets, Transactions, Payees, FxRates and FxOverrides hold the
	// finances (stage 002). With Wallets nil the wallet list of a bot is
	// checked for its form only.
	Wallets      WalletRepository
	Transactions TransactionRepository
	Payees       PayeeRepository
	FxRates      FxRateRepository
	FxOverrides  FxOverrideRepository
	// Rates gives exchange rates against PLN; nil means only the base
	// currency, overrides and cached rates are known.
	Rates RateProvider

	IDs    IDGenerator
	Tokens TokenGenerator
	Clock  Clock
	Hasher PasswordHasher
	Tx     TxManager
	// Log gets operational messages; nil drops them.
	Log *slog.Logger
}

func (d Deps) access() access {
	return access{households: d.Households, memberships: d.Memberships, bots: d.Bots, clock: d.Clock}
}

func (d Deps) log() *slog.Logger {
	if d.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return d.Log
}

// record appends an audit entry. Call it inside the transaction of the
// change it describes, so that both commit or neither does (FR-040).
func (d Deps) record(ctx context.Context, householdID domain.ID, actor domain.Actor, action, resourceType string,
	resourceID domain.ID, changes audit.Changes) error {
	if changes == nil {
		changes = audit.Changes{}
	}
	return d.Audit.Append(ctx, audit.Entry{
		ID:           d.IDs.NewID(),
		HouseholdID:  householdID,
		At:           d.Clock.Now(),
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
	})
}
