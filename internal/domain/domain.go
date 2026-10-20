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
