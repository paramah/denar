package membership

import (
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Invitation lets whoever holds its one-time token join a household with
// the role. Only the SHA-256 of the token is stored.
type Invitation struct {
	ID          domain.ID
	HouseholdID domain.ID
	Email       string // normalised; whom the inviter meant
	Role        Role
	TokenHash   string
	CreatedBy   domain.ID
	CreatedAt   time.Time
	ExpiresAt   time.Time
	AcceptedAt  time.Time // zero while not accepted
	AcceptedBy  domain.ID
	RevokedAt   time.Time // zero while not revoked
}

// InvitationState is where an invitation stands at a moment.
type InvitationState string

const (
	InvitationPending  InvitationState = "pending"
	InvitationAccepted InvitationState = "accepted"
	InvitationRevoked  InvitationState = "revoked"
	InvitationExpired  InvitationState = "expired"
)

// State returns the state at now. Acceptance and revocation are final and
// win over expiry.
func (i Invitation) State(now time.Time) InvitationState {
	switch {
	case !i.AcceptedAt.IsZero():
		return InvitationAccepted
	case !i.RevokedAt.IsZero():
		return InvitationRevoked
	case !now.Before(i.ExpiresAt):
		return InvitationExpired
	}
	return InvitationPending
}

// CheckUsable reports why an invitation cannot be accepted at now: an
// accepted one is used, an expired one has expired, a revoked one does not
// exist any more for whoever holds the link.
func (i Invitation) CheckUsable(now time.Time) error {
	switch i.State(now) {
	case InvitationAccepted:
		return domain.ConflictError{Reason: domain.ConflictInvitationUsed}
	case InvitationRevoked:
		return domain.ErrNotFound
	case InvitationExpired:
		return domain.ConflictError{Reason: domain.ConflictInvitationExpired}
	}
	return nil
}
