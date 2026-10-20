// Package usecase holds the application's interactors and the ports they
// depend on. Adapters implement the ports; this package never imports them.
package usecase

import (
	"context"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/audit"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/household"
	"git.eiac.dev/tools/denar/internal/domain/membership"
	"git.eiac.dev/tools/denar/internal/domain/user"
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
	// bots and audit trail.
	Delete(ctx context.Context, id domain.ID) error
	ByID(ctx context.Context, id domain.ID) (household.Household, error)
	// ByIDs returns the households that exist; unknown ids are skipped.
	ByIDs(ctx context.Context, ids []domain.ID) ([]household.Household, error)
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

// WalletDirectory is the extension point of stage 002: it checks that the
// wallets a bot is limited to exist in the household and are visible to
// its creator. Stage 001 has no wallets; a nil WalletDirectory accepts any
// syntactically valid list.
type WalletDirectory interface {
	// ValidateWalletIDs returns a domain.ValidationError on the field
	// "wallet_ids" when one of the wallets is not one the creator sees in
	// the household.
	ValidateWalletIDs(ctx context.Context, householdID, creatorID domain.ID, ids []domain.ID) error
}
