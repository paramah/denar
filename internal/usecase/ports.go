// Package usecase holds the application's interactors and the ports they
// depend on. Adapters implement the ports; this package never imports them.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/connection"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/household"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/domain/membership"
	"github.com/paramah/denar/internal/domain/notification"
	"github.com/paramah/denar/internal/domain/payee"
	"github.com/paramah/denar/internal/domain/subscription"
	"github.com/paramah/denar/internal/domain/transaction"
	"github.com/paramah/denar/internal/domain/user"
	"github.com/paramah/denar/internal/domain/wallet"
)

// Repository methods that look up a single entity return domain.ErrNotFound
// when it does not exist, and domain.ErrConflict when a write violates a
// uniqueness rule. Times are kept to the microsecond.

// Clock supplies the current time.
type Clock interface {
	Now() time.Time
}

// IDGenerator creates new entity identifiers.
type IDGenerator interface {
	NewID() domain.ID
}

// TokenGenerator creates secrets such as session tokens.
type TokenGenerator interface {
	// NewToken returns a URL-safe random secret of at least 256 bits.
	NewToken() (string, error)
}

// PasswordHasher hashes and checks passwords of local accounts.
type PasswordHasher interface {
	// Hash returns an encoded, salted hash of the password.
	Hash(password string) (string, error)
	// Verify reports whether password matches the encoded hash. It returns
	// false, not an error, for a mismatch or a malformed hash.
	Verify(password, encodedHash string) bool
}

// TxManager runs fn in one transaction. Repositories called with the context
// passed to fn take part in it; an error from fn rolls it back.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Page selects one page of a list. An empty Cursor starts from the
// beginning; list methods return the cursor of the next page, or "" when
// there is none. Limit is already normalised by the caller to 1..200. A
// cursor that is malformed or belongs to a different query gives
// domain.ErrInvalid.
type Page struct {
	Cursor string
	Limit  int
}

// Session is a signed-in browser session. TokenHash is the SHA-256 of the
// secret kept in the cookie; the secret itself is never stored.
type Session struct {
	TokenHash  string
	UserID     domain.ID
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type SessionRepository interface {
	Create(ctx context.Context, s Session) error
	// ByTokenHash does not check expiry; the caller does.
	ByTokenHash(ctx context.Context, hash string) (Session, error)
	Touch(ctx context.Context, hash string, lastSeen, expiresAt time.Time) error
	// Delete is idempotent.
	Delete(ctx context.Context, hash string) error
	DeleteByUser(ctx context.Context, userID domain.ID) error
	DeleteExpired(ctx context.Context, now time.Time) error
}

// OIDCLoginState is what the server remembers between sending a browser to
// the identity provider and getting it back.
type OIDCLoginState struct {
	StateHash    string // SHA-256 of the state value
	Nonce        string
	CodeVerifier string
	ReturnTo     string
	ExpiresAt    time.Time
}

type OIDCStateRepository interface {
	Create(ctx context.Context, s OIDCLoginState) error
	// Consume returns and deletes the state in one step, so it can be used
	// once. It returns domain.ErrNotFound when the state is unknown or
	// already consumed. It does not check expiry.
	Consume(ctx context.Context, stateHash string) (OIDCLoginState, error)
	DeleteExpired(ctx context.Context, now time.Time) error
}

// OIDCIdentity is what the identity provider vouches for after a login.
type OIDCIdentity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified *bool // nil when the provider does not say
	Name          string
}

// OIDCProvider talks to the identity provider.
type OIDCProvider interface {
	// Enabled reports whether OIDC is configured.
	Enabled() bool
	// AuthURL returns the provider's authorization URL for the given values.
	AuthURL(ctx context.Context, state, nonce, codeVerifier string) (string, error)
	// Exchange trades the code for tokens and verifies the ID token,
	// including the nonce.
	Exchange(ctx context.Context, code, nonce, codeVerifier string) (OIDCIdentity, error)
}

type UserRepository interface {
	// Create returns a domain.ConflictError with ConflictDuplicateEmail when
	// the email is taken.
	Create(ctx context.Context, u user.User) error
	Update(ctx context.Context, u user.User) error
	ByID(ctx context.Context, id domain.ID) (user.User, error)
	// ByIDs returns the users that exist; unknown ids are skipped.
	ByIDs(ctx context.Context, ids []domain.ID) ([]user.User, error)
	ByOIDCSubject(ctx context.Context, issuer, subject string) (user.User, error)
	ByEmail(ctx context.Context, email string) (user.User, error)
}

type HouseholdRepository interface {
	Create(ctx context.Context, h household.Household) error
	// Update stores the name, the time zone and the month start day; the
	// base currency, the creator and the creation time never change.
	Update(ctx context.Context, h household.Household) error
	// Delete removes the household with its memberships, invitations,
	// bots, audit trail, wallets, transactions, payees and rate overrides.
	Delete(ctx context.Context, id domain.ID) error
	ByID(ctx context.Context, id domain.ID) (household.Household, error)
	// ByIDs returns the households that exist; unknown ids are skipped.
	ByIDs(ctx context.Context, ids []domain.ID) ([]household.Household, error)
	// ListIDs returns the ids of every household, for the jobs.
	ListIDs(ctx context.Context) ([]domain.ID, error)
}

type MembershipRepository interface {
	// Create returns a domain.ConflictError with ConflictAlreadyMember when
	// the user is a member already.
	Create(ctx context.Context, m membership.Membership) error
	// UpdateRole returns domain.ErrNotFound for a missing membership.
	UpdateRole(ctx context.Context, householdID, userID domain.ID, role membership.Role) error
	// Delete returns domain.ErrNotFound for a missing membership.
	Delete(ctx context.Context, householdID, userID domain.ID) error
	Get(ctx context.Context, householdID, userID domain.ID) (membership.Membership, error)
	// ListByHousehold returns the members, oldest first.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Membership, error)
	// ListByUser returns the user's memberships, oldest first.
	ListByUser(ctx context.Context, userID domain.ID) ([]membership.Membership, error)
}

type InvitationRepository interface {
	Create(ctx context.Context, i membership.Invitation) error
	// Update stores AcceptedAt, AcceptedBy and RevokedAt.
	Update(ctx context.Context, i membership.Invitation) error
	ByID(ctx context.Context, id domain.ID) (membership.Invitation, error)
	ByTokenHash(ctx context.Context, hash string) (membership.Invitation, error)
	// ListByHousehold returns the invitations, newest first.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]membership.Invitation, error)
	// PendingByEmail returns the invitations of the email that are neither
	// accepted nor revoked and expire after now.
	PendingByEmail(ctx context.Context, email string, now time.Time) ([]membership.Invitation, error)
}

type BotRepository interface {
	// Create returns a domain.ConflictError with ConflictDuplicateName when
	// the household has a bot of that name, case-insensitively.
	Create(ctx context.Context, b bot.Bot) error
	// Update stores the name, scopes, wallets, expiry, disabled flag and
	// the token (hash and prefix).
	Update(ctx context.Context, b bot.Bot) error
	Delete(ctx context.Context, id domain.ID) error
	ByID(ctx context.Context, id domain.ID) (bot.Bot, error)
	ByTokenHash(ctx context.Context, hash string) (bot.Bot, error)
	// ListByHousehold returns the bots ordered by name, then id.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]bot.Bot, error)
	// DisableByCreator disables the bots the user created in the household
	// and returns those it changed.
	DisableByCreator(ctx context.Context, householdID, creatorID domain.ID) ([]bot.Bot, error)
	// TouchUsed stores the time the token was last used.
	TouchUsed(ctx context.Context, id domain.ID, at time.Time) error
}

// AuditFilter narrows AuditRepository.List. Zero fields do not filter.
type AuditFilter struct {
	HouseholdID  domain.ID // required
	ActorKind    domain.ActorKind
	ActorID      domain.ID
	ResourceType string
}

type AuditRepository interface {
	Append(ctx context.Context, e audit.Entry) error
	// List returns the entries matching f, newest first (by id, which
	// sorts by time).
	List(ctx context.Context, f AuditFilter, p Page) (items []audit.Entry, next string, err error)
}

// WalletRepository stores wallets. Lookups by id take the household, so
// that the id of another household's wallet is not found.
type WalletRepository interface {
	// Create returns a domain.ConflictError with ConflictDuplicateName when
	// the household has a wallet of that name, case-insensitively.
	Create(ctx context.Context, w wallet.Wallet) error
	// Update stores the name, type, on-budget flag, visibility, members
	// and archive time; the currency, creator and creation time never
	// change.
	Update(ctx context.Context, w wallet.Wallet) error
	ByID(ctx context.Context, householdID, id domain.ID) (wallet.Wallet, error)
	// ListByHousehold returns every wallet, archived ones too, ordered by
	// name, then id.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]wallet.Wallet, error)
}

// TransactionFilter narrows TransactionRepository.List. Zero fields do not
// filter, except WalletIDs: only transactions of those wallets are listed,
// and none when it is empty.
type TransactionFilter struct {
	HouseholdID domain.ID
	WalletIDs   []domain.ID
	PayeeID     domain.ID
	// EnvelopeID matches the transaction's envelope or one of its split
	// lines.
	EnvelopeID domain.ID
	From, To   domain.Date // inclusive
	// Text occurs, ignoring case, in the note or the payee's name.
	Text   string
	Kind   transaction.Kind
	Status transaction.Status
	Tag    string
}

// WalletBalance sums the transactions of one wallet.
type WalletBalance struct {
	Total   int64 // minor units of the wallet's currency
	Cleared int64 // cleared and reconciled transactions only
	Base    int64 // base amounts known so far
	Pending int   // transactions whose base amount waits for a rate
}

// TransactionRepository stores transactions with their split lines.
type TransactionRepository interface {
	// Create returns a domain.ConflictError with
	// ConflictDuplicateExternalID when the wallet has a transaction with
	// that external id.
	Create(ctx context.Context, t transaction.Transaction) error
	// Update stores every field except the id, household, wallet, source,
	// creator and creation time; the splits are replaced.
	Update(ctx context.Context, t transaction.Transaction) error
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (transaction.Transaction, error)
	// ByPair returns the legs of a transfer, ordered by id.
	ByPair(ctx context.Context, householdID, pairID domain.ID) ([]transaction.Transaction, error)
	// List returns the matching transactions, newest date first, then by
	// id descending.
	List(ctx context.Context, f TransactionFilter, p Page) (items []transaction.Transaction, next string, err error)
	// Balances sums the transactions of the wallets; a wallet without
	// transactions has a zero balance in the map.
	Balances(ctx context.Context, householdID domain.ID, walletIDs []domain.ID) (map[domain.ID]WalletBalance, error)
	// ListRatePending returns up to limit transactions waiting for a rate,
	// oldest first; an empty householdID means every household.
	ListRatePending(ctx context.Context, householdID domain.ID, limit int) ([]transaction.Transaction, error)
	// BudgetFlows sums the base amounts of the wallets' transactions dated
	// up to to (a zero to: all), for the budget: a transaction without
	// split lines counts with its own envelope, one with split lines by its
	// lines, each with its line's envelope and the transaction's kind.
	// Transactions waiting for a rate are left out of the sums and counted
	// per day in Pending.
	BudgetFlows(ctx context.Context, householdID domain.ID, walletIDs []domain.ID, to domain.Date) (BudgetFlows, error)
	// CountByEnvelope counts the household's transactions that have the
	// envelope, on themselves or on a split line.
	CountByEnvelope(ctx context.Context, householdID, envelopeID domain.ID) (int, error)
	// ExternalIDs returns which of the external ids the wallet's
	// transactions have, with the id of the transaction of each (stage
	// 008, import deduplication).
	ExternalIDs(ctx context.Context, householdID, walletID domain.ID, externalIDs []string) (map[string]domain.ID, error)
}

// BudgetFlow is the sum of base amounts of one day, envelope and kind.
type BudgetFlow struct {
	Date       domain.Date
	EnvelopeID domain.ID // "" without an envelope
	Kind       transaction.Kind
	Base       int64
}

// BudgetFlows are the sums TransactionRepository.BudgetFlows returns: one
// flow per day, envelope and kind, ordered by day, envelope, kind; and the
// number of transactions waiting for a rate per day.
type BudgetFlows struct {
	Flows   []BudgetFlow
	Pending map[domain.Date]int
}

// EnvelopeGroupRepository stores envelope groups.
type EnvelopeGroupRepository interface {
	// Create and Update return a domain.ConflictError with
	// ConflictDuplicateName when the household has a group of that name,
	// case-insensitively.
	Create(ctx context.Context, g budget.Group) error
	// Update stores the name, position and update time.
	Update(ctx context.Context, g budget.Group) error
	// Delete returns a domain.ConflictError with ConflictGroupNotEmpty
	// while the group has envelopes.
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (budget.Group, error)
	// ListByHousehold returns the groups ordered as budget.SortGroups does.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Group, error)
}

// EnvelopeRepository stores envelopes.
type EnvelopeRepository interface {
	// Create and Update return a domain.ConflictError with
	// ConflictDuplicateName when the household has an envelope of that
	// name, case-insensitively, and domain.ErrNotFound when the group is
	// not one of the household.
	Create(ctx context.Context, e budget.Envelope) error
	// Update stores the group, name, position, hidden flag, goal and
	// update time.
	Update(ctx context.Context, e budget.Envelope) error
	// Delete removes the envelope with its assignments.
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (budget.Envelope, error)
	// ListByHousehold returns every envelope, hidden ones too, ordered as
	// budget.SortEnvelopes does.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Envelope, error)
}

// AssignmentRepository stores the monthly assignments of envelopes.
type AssignmentRepository interface {
	// Set stores the assignment, replacing the envelope's one of the same
	// month; an amount of zero removes it. The envelope must be one of the
	// household (domain.ErrNotFound otherwise).
	Set(ctx context.Context, a budget.Assignment) error
	// Get returns the assignment, with a zero amount when there is none.
	Get(ctx context.Context, householdID, envelopeID domain.ID, m budget.Month) (budget.Assignment, error)
	// ListByHousehold returns the household's assignments ordered by
	// month, then envelope id.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]budget.Assignment, error)
}

// PayeeRepository stores payees.
type PayeeRepository interface {
	// Create and Update return a domain.ConflictError with
	// ConflictDuplicateName when the household has a payee of that name,
	// case-insensitively.
	Create(ctx context.Context, p payee.Payee) error
	// Update stores the name, patterns, default envelope and update time.
	Update(ctx context.Context, p payee.Payee) error
	// Delete removes the payee; its transactions keep no payee.
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (payee.Payee, error)
	// ListByHousehold returns the payees ordered by name, then id.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]payee.Payee, error)
}

// FxRate is a cached rate of NBP table A: how many PLN one unit of the
// currency cost on RateDate, the business day the rate of Date falls back
// to.
type FxRate struct {
	Currency  string
	Date      domain.Date
	RateDate  domain.Date
	Rate      string // decimal
	FetchedAt time.Time
}

// FxRateRepository caches rates of the rate provider for every household.
type FxRateRepository interface {
	Get(ctx context.Context, currency string, date domain.Date) (FxRate, error)
	// Put stores the rate, replacing one of the same currency and date.
	Put(ctx context.Context, r FxRate) error
	// Latest returns the cached rate of the currency with the newest
	// RateDate within [from, to].
	Latest(ctx context.Context, currency string, from, to domain.Date) (FxRate, error)
}

// FxOverride is a rate a household set by hand: one unit of Currency is
// Rate units of the household's base currency on Date.
type FxOverride struct {
	HouseholdID domain.ID
	Currency    string
	Date        domain.Date
	Rate        string // decimal
	CreatedBy   domain.Actor
	CreatedAt   time.Time
}

type FxOverrideRepository interface {
	// Put stores the override, replacing one of the same currency and day.
	Put(ctx context.Context, o FxOverride) error
	Get(ctx context.Context, householdID domain.ID, currency string, date domain.Date) (FxOverride, error)
	// List returns the household's overrides, newest date first, then by
	// currency.
	List(ctx context.Context, householdID domain.ID) ([]FxOverride, error)
}

// ErrRateUnavailable: the rate provider has no rate for the day, or cannot
// be reached.
var ErrRateUnavailable = errors.New("exchange rate unavailable")

// RateProvider gives exchange rates against PLN (NBP table A).
type RateProvider interface {
	// PLNRate returns how many PLN one unit of the currency costs on the
	// newest business day within [date-6, date], and that day. It returns
	// an error matching ErrRateUnavailable when there is none or the
	// provider cannot be reached.
	PLNRate(ctx context.Context, currency string, date domain.Date) (rate string, rateDate domain.Date, err error)
}

// SubscriptionRepository stores subscriptions with their price points.
// Lookups by id take the household.
type SubscriptionRepository interface {
	// Create stores the subscription and its prices. It returns a
	// domain.ConflictError with ConflictDuplicateName when the household
	// has a subscription of that name, case-insensitively.
	Create(ctx context.Context, s subscription.Subscription) error
	// Update stores every field except the id, household, prices, creator
	// and creation time.
	Update(ctx context.Context, s subscription.Subscription) error
	// PutPrice stores the price point, replacing the subscription's point
	// of the same EffectiveFrom.
	PutPrice(ctx context.Context, householdID, subscriptionID domain.ID, p subscription.PricePoint) error
	// Delete removes the subscription with its prices and charges.
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (subscription.Subscription, error)
	// List returns the household's subscriptions, or with an empty
	// householdID those of every household, ordered as
	// subscription.SortByName does (by household first for all).
	List(ctx context.Context, householdID domain.ID) ([]subscription.Subscription, error)
	// CountByEnvelope counts the household's subscriptions with the
	// envelope.
	CountByEnvelope(ctx context.Context, householdID, envelopeID domain.ID) (int, error)
}

// ChargeFilter narrows ChargeRepository.List. Zero fields do not filter,
// except SubscriptionIDs: only charges of those subscriptions are listed,
// and none when it is empty.
type ChargeFilter struct {
	HouseholdID     domain.ID
	SubscriptionIDs []domain.ID
	Statuses        []subscription.ChargeStatus
	From, To        domain.Date // inclusive
}

// ChargeRepository stores the charges expected from subscriptions.
type ChargeRepository interface {
	// Create returns a domain.ConflictError with ConflictDuplicateCharge
	// when the subscription has a charge on the due date.
	Create(ctx context.Context, c subscription.Charge) error
	// Update stores the amount, currency, status, transaction and update
	// time. A transaction that is another charge's gives a
	// domain.ConflictError with ConflictTransactionLinked.
	Update(ctx context.Context, c subscription.Charge) error
	Delete(ctx context.Context, householdID, id domain.ID) error
	ByID(ctx context.Context, householdID, id domain.ID) (subscription.Charge, error)
	// ByTransaction returns the charge linked to the transaction.
	ByTransaction(ctx context.Context, householdID, transactionID domain.ID) (subscription.Charge, error)
	// List returns the matching charges by due date, then id.
	List(ctx context.Context, f ChargeFilter, p Page) (items []subscription.Charge, next string, err error)
	// ListBySubscription returns the subscription's charges due on or
	// after from (a zero from: all), by due date.
	ListBySubscription(ctx context.Context, householdID, subscriptionID domain.ID, from domain.Date) ([]subscription.Charge, error)
	// ListOpen returns up to limit expected and missed charges due within
	// [from, to] (zero bounds do not limit), by due date, then id; an
	// empty householdID means every household.
	ListOpen(ctx context.Context, householdID domain.ID, from, to domain.Date, limit int) ([]subscription.Charge, error)
}

// EventOutbox keeps domain events until they are dispatched (stage 005).
// Append them in the transaction of the change they describe.
type EventOutbox interface {
	// Append stores the event unless the household has one with the same
	// DedupeKey, and reports whether it stored it.
	Append(ctx context.Context, e event.Event) (bool, error)
	// ListPending returns up to limit events not dispatched yet, oldest
	// first (by id).
	ListPending(ctx context.Context, limit int) ([]event.Event, error)
	// MarkDispatched stores the time the event was dispatched.
	MarkDispatched(ctx context.Context, id domain.ID, at time.Time) error
	// DeleteDispatchedBefore removes the events dispatched before the time
	// and returns how many it removed (stage 005, retention).
	DeleteDispatchedBefore(ctx context.Context, before time.Time) (int, error)
}

// NotificationFilter narrows the notifications of one person.
type NotificationFilter struct {
	UserID      domain.ID // required
	HouseholdID domain.ID // "" means every household
	UnreadOnly  bool
	// RestrictWallets keeps only notifications without a wallet or with
	// one of WalletIDs (a bot limited to wallets).
	RestrictWallets bool
	WalletIDs       []domain.ID
}

// NotificationRepository stores the notifications of the app. Lookups by
// id take the person, so that another person's notification is not found.
type NotificationRepository interface {
	// Create stores the notification unless the person has one of the same
	// event, and reports whether it stored it.
	Create(ctx context.Context, n notification.Notification) (bool, error)
	ByID(ctx context.Context, userID, id domain.ID) (notification.Notification, error)
	// List returns the matching notifications, newest first (by id
	// descending).
	List(ctx context.Context, f NotificationFilter, p Page) (items []notification.Notification, next string, err error)
	CountUnread(ctx context.Context, f NotificationFilter) (int, error)
	// MarkRead sets ReadAt unless it is set already.
	MarkRead(ctx context.Context, userID, id domain.ID, at time.Time) error
	// MarkAllRead marks the matching unread notifications read and returns
	// how many it marked.
	MarkAllRead(ctx context.Context, f NotificationFilter, at time.Time) (int, error)
	// DeleteReadBefore removes notifications read before the time.
	DeleteReadBefore(ctx context.Context, before time.Time) (int, error)
}

// ChannelRepository stores the channels of people.
type ChannelRepository interface {
	// Create and Update return a domain.ConflictError with
	// ConflictDuplicateName when the person has a channel of that name,
	// case-insensitively.
	Create(ctx context.Context, c notification.Channel) error
	// Update stores every field except the id, person, type and creation
	// time.
	Update(ctx context.Context, c notification.Channel) error
	// Delete removes the channel with its deliveries.
	Delete(ctx context.Context, userID, id domain.ID) error
	ByID(ctx context.Context, userID, id domain.ID) (notification.Channel, error)
	// ListByUser returns the person's channels by name, then id.
	ListByUser(ctx context.Context, userID domain.ID) ([]notification.Channel, error)
}

// PreferenceRepository stores what people want to be told per household
// and event type; a type without a stored preference has the default.
type PreferenceRepository interface {
	// List returns the stored preferences in the order of event.Types.
	List(ctx context.Context, userID, householdID domain.ID) ([]notification.Preference, error)
	// Replace stores exactly these preferences of the person in the
	// household. The household must exist (domain.ErrNotFound).
	Replace(ctx context.Context, userID, householdID domain.ID, prefs []notification.Preference) error
}

// DeliveryRepository is the outbox of messages to channels.
type DeliveryRepository interface {
	// Create stores the delivery unless there is one of the same event,
	// person and channel, and reports whether it stored it.
	Create(ctx context.Context, d notification.Delivery) (bool, error)
	// ListDue returns up to limit pending deliveries whose next attempt is
	// due at now, the most overdue first.
	ListDue(ctx context.Context, now time.Time, limit int) ([]notification.Delivery, error)
	ByID(ctx context.Context, id domain.ID) (notification.Delivery, error)
	// Update stores the status, attempts, next attempt, last error, update
	// and send times.
	Update(ctx context.Context, d notification.Delivery) error
	// DeleteFinishedBefore removes sent and failed deliveries last updated
	// before the time.
	DeleteFinishedBefore(ctx context.Context, before time.Time) (int, error)
}

// ChannelTarget is a channel as a Notifier needs it: its settings and its
// secrets in the clear.
type ChannelTarget struct {
	Type    notification.ChannelType
	Config  map[string]string
	Secrets map[string]string
}

// OutgoingMessage is what a Notifier sends.
type OutgoingMessage struct {
	// ID identifies the message; sending the same ID twice is the same
	// message (a Matrix transaction id, the webhook delivery header).
	ID            string
	Type          string
	Title         string
	Body          string
	URL           string // absolute; "" for none
	HouseholdID   domain.ID
	HouseholdName string
	OccurredAt    time.Time
	Data          map[string]string
}

// Notifier sends messages over one type of channel. Errors are short and
// never contain secrets.
type Notifier interface {
	Send(ctx context.Context, target ChannelTarget, msg OutgoingMessage) error
}

// Mailer sends plain-text e-mail through the server's SMTP settings.
type Mailer interface {
	SendMail(ctx context.Context, to, subject, body string) error
}

// SecretBox encrypts secrets kept in the database. aad binds a sealed
// value to its record.
type SecretBox interface {
	Seal(plaintext, aad []byte) ([]byte, error)
	Open(sealed, aad []byte) ([]byte, error)
}

// ImportedTransaction is one transaction brought in from outside: a row
// of a CSV file or what a connector fetched. The amount is in the wallet's
// currency.
type ImportedTransaction struct {
	// Row is the line of the file (1-based), or the position in what a
	// connector returned; it names the row in errors.
	Row int
	// ExternalID identifies the transaction at its source; "" gets a hash
	// (imports.HashExternalID).
	ExternalID string
	Date       domain.Date
	// Amount is a decimal ("-12.34"): negative pays out, positive comes in.
	Amount string
	// Currency is the amount's; "" means the wallet's. Another currency
	// than the wallet's is a row error.
	Currency string
	// PayeeRaw is the counterparty as the source writes it; Memo the rest
	// of the description.
	PayeeRaw string
	Memo     string
}

// ImportBatchFilter narrows ImportBatchRepository.List: only batches of
// WalletIDs are listed, and none when it is empty.
type ImportBatchFilter struct {
	HouseholdID domain.ID
	WalletIDs   []domain.ID
}

// ImportBatchRepository stores import batches and which transactions each
// created. Lookups by id take the household.
type ImportBatchRepository interface {
	Create(ctx context.Context, b imports.Batch) error
	// Update stores the status, the matched and reverted counters and who
	// reverted the batch when.
	Update(ctx context.Context, b imports.Batch) error
	ByID(ctx context.Context, householdID, id domain.ID) (imports.Batch, error)
	// List returns the matching batches, newest first (by id descending).
	List(ctx context.Context, f ImportBatchFilter, p Page) (items []imports.Batch, next string, err error)
	// AddTransactions links transactions to the batch.
	AddTransactions(ctx context.Context, batchID domain.ID, transactionIDs []domain.ID) error
	// TransactionIDs returns the batch's transactions that still exist,
	// in ascending order; a deleted transaction is no longer linked.
	TransactionIDs(ctx context.Context, householdID, batchID domain.ID) ([]domain.ID, error)
}

// CSVColumns maps fields to the columns of a CSV file. Each holds
// alternatives: a header name (ignoring case and a leading '#') or a
// 1-based column number. Date, Amount, Debit, Credit, ExternalID,
// Currency, Fee and State take the first column the file has; Payee the
// first non-empty value of a row among them; Memo every one, joined.
type CSVColumns struct {
	Date, Amount, Debit, Credit, ExternalID, Currency, Fee, State []string
	Payee, Memo                                                   []string
}

// CSVOptions says how to read a CSV file. Empty fields take the preset's
// value; the preset's "" means detect it.
type CSVOptions struct {
	Preset string
	// Delimiter is ";", "," or "tab".
	Delimiter string
	// Encoding is "utf-8" or "windows-1250".
	Encoding string
	// DateFormat is made of YYYY, MM, DD, M, D (and HH, mm, ss).
	DateFormat string
	// Decimal is "," or ".".
	Decimal      string
	SkipRows     int
	HasHeader    *bool
	InvertAmount *bool
	// AcceptStates, with a State column, are the states imported.
	AcceptStates []string
	// Columns given replace the preset's, field by field.
	Columns CSVColumns
}

// CSVParseResult is what a CSV file holds and how it was read.
type CSVParseResult struct {
	Rows      []ImportedTransaction
	Errors    []imports.RowError
	Delimiter string
	Encoding  string
	// HeaderLine is the line of the header, 0 without one.
	HeaderLine int
	Headers    []string
}

// CSVPreset is a known export format: its defaults.
type CSVPreset struct {
	Key         string
	Name        string
	Description string
	Options     CSVOptions
}

// CSVParser reads bank statements in CSV.
type CSVParser interface {
	// Presets returns the presets, "generic" first.
	Presets() []CSVPreset
	// Parse reads the file. A file it cannot read at all (an unknown
	// preset or option, no header, no date or amount column) gives a
	// domain.ValidationError; bad rows are in the result's Errors.
	Parse(data []byte, opts CSVOptions) (CSVParseResult, error)
}

// ConnectorAccount is an account a bank connector sees.
type ConnectorAccount struct {
	ExternalID string
	Name       string
	Currency   string
}

// BillingCharge is a cost a billing connector reports: a positive amount.
type BillingCharge struct {
	ExternalID  string
	Date        domain.Date
	Amount      string // decimal, > 0
	Currency    string
	Description string
	// SubscriptionRef is the provider's id of the plan or subscription, to
	// meet Subscription.ExternalRef later.
	SubscriptionRef string
}

// BankConnector reads accounts and transactions of a bank (stage 009).
// Credentials are what CreateConnection sealed, in the clear. Errors are
// short and never contain secrets.
type BankConnector interface {
	ListAccounts(ctx context.Context, credentials []byte) ([]ConnectorAccount, error)
	FetchTransactions(ctx context.Context, credentials []byte, accountExternalID string, since domain.Date) ([]ImportedTransaction, error)
}

// BillingConnector reads the charges of a provider (stages 010, 011).
type BillingConnector interface {
	FetchCharges(ctx context.Context, credentials []byte, since domain.Date) ([]BillingCharge, error)
}

// ConnectorProvider is one provider of the registry: a bank one has Bank,
// a billing one Billing.
type ConnectorProvider struct {
	Key     string
	Name    string
	Kind    connection.Kind
	Bank    BankConnector
	Billing BillingConnector
}

// ConnectionRepository stores connections and their accounts. Lookups by
// id take the household.
type ConnectionRepository interface {
	Create(ctx context.Context, c connection.Connection) error
	// Update stores the status, display name, credentials, last sync, last
	// error and update time.
	Update(ctx context.Context, c connection.Connection) error
	ByID(ctx context.Context, householdID, id domain.ID) (connection.Connection, error)
	// ListByHousehold returns the connections by display name, then id.
	ListByHousehold(ctx context.Context, householdID domain.ID) ([]connection.Connection, error)
	// PutAccount stores the account, replacing the connection's account of
	// the same external id. The connection must be one of the household
	// (domain.ErrNotFound otherwise).
	PutAccount(ctx context.Context, householdID domain.ID, a connection.Account) error
	// ListAccounts returns the connection's accounts by external id.
	ListAccounts(ctx context.Context, householdID, connectionID domain.ID) ([]connection.Account, error)
}
