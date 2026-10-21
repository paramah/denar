package usecase

import (
	"context"
	"log/slog"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/notification"
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
	// EnvelopeGroups, Envelopes and Assignments hold the budget (stage
	// 003). With Envelopes nil the envelope of a transaction or payee is
	// not checked.
	EnvelopeGroups EnvelopeGroupRepository
	Envelopes      EnvelopeRepository
	Assignments    AssignmentRepository
	// Subscriptions, Charges and Events hold the subscriptions (stage
	// 004). With Subscriptions nil envelopes and transactions are not
	// checked against them. ChargeHorizonDays is how far ahead charges
	// are expected; 0 means 90.
	Subscriptions     SubscriptionRepository
	Charges           ChargeRepository
	Events            EventOutbox
	ChargeHorizonDays int
	// Notifications, Channels, Preferences and Deliveries hold the
	// notifications (stage 005). Notifiers send over each channel type
	// the server can reach; Mailer sends e-mail (nil without SMTP);
	// Secrets encrypts channel secrets (nil without a master key).
	// BaseURL starts the links sent outside the app. GoalDueDays is how
	// many days ahead a goal is reminded of; 0 means 7.
	Notifications NotificationRepository
	Channels      ChannelRepository
	Preferences   PreferenceRepository
	Deliveries    DeliveryRepository
	Notifiers     map[notification.ChannelType]Notifier
	Mailer        Mailer
	Secrets       SecretBox
	BaseURL       string
	GoalDueDays   int
	// NotifyTargets says which addresses channels may send to (stage 009);
	// the zero value allows public addresses only.
	NotifyTargets notification.TargetPolicy
	// ImportBatches and Connections hold the imports and the links to
	// outside services (stage 008). CSV reads CSV files; nil means CSV
	// imports are unavailable. Connectors is the registry of providers by
	// key; nil means none.
	ImportBatches ImportBatchRepository
	Connections   ConnectionRepository
	CSV           CSVParser
	Connectors    map[string]ConnectorProvider
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
