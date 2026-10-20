package fakes

import (
	"time"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// Repos is every fake port on one store, wired as the composition root
// wires the real ones.
type Repos struct {
	Store       *Store
	Users       *UserRepository
	Households  *HouseholdRepository
	Memberships *MembershipRepository
	Invitations *InvitationRepository
	Bots        *BotRepository
	Audit       *AuditRepository
	Sessions    *SessionRepository
	OIDCStates  *OIDCStateRepository
	Tx          *TxManager
	Clock       *Clock
	IDs         *IDGenerator
	Tokens      *TokenGenerator
	Hasher      *PasswordHasher
}

// NewRepos returns empty repositories and a clock at a fixed time.
func NewRepos() *Repos {
	s := NewStore()
	r := &Repos{
		Store:       s,
		Users:       NewUserRepository(s),
		Households:  NewHouseholdRepository(s),
		Memberships: NewMembershipRepository(s),
		Invitations: NewInvitationRepository(s),
		Bots:        NewBotRepository(s),
		Audit:       NewAuditRepository(s),
		Sessions:    NewSessionRepository(s),
		OIDCStates:  NewOIDCStateRepository(s),
		Tx:          NewTxManager(s),
		Clock:       NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)),
		IDs:         NewIDGenerator(),
		Tokens:      NewTokenGenerator(),
		Hasher:      NewPasswordHasher(),
	}
	r.Households.Cascade(r.Memberships, r.Invitations, r.Bots, r.Audit)
	return r
}

// Deps returns the use case dependencies over the fakes.
func (r *Repos) Deps() usecase.Deps {
	return usecase.Deps{
		Users: r.Users, Households: r.Households, Memberships: r.Memberships, Invitations: r.Invitations,
		Bots: r.Bots, Audit: r.Audit, Sessions: r.Sessions,
		IDs: r.IDs, Tokens: r.Tokens, Clock: r.Clock, Hasher: r.Hasher, Tx: r.Tx,
	}
}
