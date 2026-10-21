// Package domain holds the types shared by every domain subpackage.
// It and its subpackages import only the standard library.
package domain

import "errors"

// ID identifies an entity. Values are opaque and generated outside the domain.
type ID string

// ActorKind tells who performed an action.
type ActorKind string

const (
	ActorUser ActorKind = "user"
	ActorBot  ActorKind = "bot"
	// ActorSystem is the command line of the instance administrator.
	ActorSystem ActorKind = "system"
)

// Valid reports whether k is a known actor kind.
func (k ActorKind) Valid() bool {
	return k == ActorUser || k == ActorBot || k == ActorSystem
}

// Actor is whoever performs an action.
type Actor struct {
	Kind ActorKind
	ID   ID
}

// IsZero reports whether the actor is unset.
func (a Actor) IsZero() bool { return a.Kind == "" && a.ID == "" }

// Sentinel errors. Adapters map them to transport-level codes.
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalid         = errors.New("invalid")
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrBadRequest: the request is malformed beyond its fields, for
	// example it carries two kinds of credentials.
	ErrBadRequest = errors.New("bad request")
)

// ValidationError says which input field was rejected and why. It matches
// ErrInvalid. Report several at once with errors.Join.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Message }

func (e ValidationError) Is(target error) bool { return target == ErrInvalid }

// ConflictReason says why a write conflicted with the stored state.
type ConflictReason string

const (
	ConflictDuplicateEmail    ConflictReason = "duplicate_email"
	ConflictDuplicateName     ConflictReason = "duplicate_name"
	ConflictLastOwner         ConflictReason = "last_owner"
	ConflictAlreadyMember     ConflictReason = "already_member"
	ConflictInvitationUsed    ConflictReason = "invitation_used"
	ConflictInvitationExpired ConflictReason = "invitation_expired"
)

// ConflictError is a conflict with a known reason. It matches ErrConflict.
type ConflictError struct {
	Reason ConflictReason
}

func (e ConflictError) Error() string { return "conflict: " + string(e.Reason) }

func (e ConflictError) Is(target error) bool { return target == ErrConflict }

// ForbiddenReason says why an action is refused.
type ForbiddenReason string

const (
	// ForbiddenHumanOnly: bots may not do this at all (administration).
	ForbiddenHumanOnly ForbiddenReason = "human_only"
	// ForbiddenRole: the caller's role in the household is too low.
	ForbiddenRole ForbiddenReason = "role"
	// ForbiddenScope: the bot lacks the scope, or its creator the role.
	ForbiddenScope ForbiddenReason = "scope"
)

// ForbiddenError is a refusal with a known reason. It matches ErrForbidden.
type ForbiddenError struct {
	Reason ForbiddenReason
	// Scope names the bot scope the operation needs, when that is what is
	// missing (ForbiddenScope, or ForbiddenRole for a scope the role does
	// not permit); "" otherwise.
	Scope string
}

func (e ForbiddenError) Error() string { return "forbidden: " + string(e.Reason) }

func (e ForbiddenError) Is(target error) bool { return target == ErrForbidden }

// Optional is a value that may be absent. Partial updates use it to tell
// "leave unchanged" (unset) from "set to the zero value".
type Optional[T any] struct {
	Set   bool
	Value T
}

// Some returns a set Optional holding v.
func Some[T any](v T) Optional[T] { return Optional[T]{Set: true, Value: v} }

// Reasons of stage 002.
const (
	// ConflictDuplicateExternalID: the wallet has a transaction with that
	// external id already.
	ConflictDuplicateExternalID ConflictReason = "duplicate_external_id"
	// ConflictWalletArchived: the wallet is archived and takes no writes.
	ConflictWalletArchived ConflictReason = "wallet_archived"
	// ForbiddenWallet: the operation reaches a wallet the caller does not
	// see, such as the other leg of a transfer.
	ForbiddenWallet ForbiddenReason = "wallet"
)

// Reasons of stage 003.
const (
	// ConflictGroupNotEmpty: an envelope group still has envelopes.
	ConflictGroupNotEmpty ConflictReason = "group_not_empty"
	// ConflictEnvelopeInUse: transactions use the envelope; hide it
	// instead.
	ConflictEnvelopeInUse ConflictReason = "envelope_in_use"
)

// Reasons of stage 004.
const (
	// ConflictDuplicateCharge: the subscription has a charge on that day.
	ConflictDuplicateCharge ConflictReason = "duplicate_charge"
	// ConflictChargeState: the charge's status does not allow the
	// operation (for example, matching one already matched).
	ConflictChargeState ConflictReason = "charge_state"
	// ConflictTransactionLinked: the transaction pays another charge.
	ConflictTransactionLinked ConflictReason = "transaction_linked"
)

// Reasons of stage 005.
const (
	// ConflictSecretsUnavailable: the channel needs secrets and the server
	// has no master key (DENAR_MASTER_KEY) to encrypt them.
	ConflictSecretsUnavailable ConflictReason = "secrets_unavailable"
	// ConflictSMTPNotConfigured: sending e-mail needs SMTP, which the
	// server does not have.
	ConflictSMTPNotConfigured ConflictReason = "smtp_not_configured"
	// ConflictTooManyChannels: the person has the most channels allowed.
	ConflictTooManyChannels ConflictReason = "too_many_channels"
)
