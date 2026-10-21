package fakes

import (
	"time"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// Repos is every fake port on one store, wired as the composition root
// wires the real ones.
type Repos struct {
	Store        *Store
	Users        *UserRepository
	Households   *HouseholdRepository
	Memberships  *MembershipRepository
	Invitations  *InvitationRepository
	Bots         *BotRepository
	Audit        *AuditRepository
	Sessions     *SessionRepository
	OIDCStates   *OIDCStateRepository
	Wallets      *WalletRepository
	Transactions *TransactionRepository
	Payees       *PayeeRepository
	FxRates      *FxRateRepository
	FxOverrides  *FxOverrideRepository
	Rates        *RateProvider
	Tx           *TxManager
	Clock        *Clock
	IDs          *IDGenerator
	Tokens       *TokenGenerator
	Hasher       *PasswordHasher
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
		Wallets:     NewWalletRepository(s),
		Payees:      NewPayeeRepository(s),
		FxRates:     NewFxRateRepository(s),
		FxOverrides: NewFxOverrideRepository(s),
		Rates:       NewRateProvider(),
		Tx:          NewTxManager(s),
		Clock:       NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)),
		IDs:         NewIDGenerator(),
		Tokens:      NewTokenGenerator(),
		Hasher:      NewPasswordHasher(),
	}
	r.Transactions = NewTransactionRepository(s, r.Payees)
	r.Households.Cascade(r.Memberships, r.Invitations, r.Bots, r.Audit, r.Wallets, r.Transactions, r.Payees, r.FxOverrides)
	return r
}

// Deps returns the use case dependencies over the fakes.
func (r *Repos) Deps() usecase.Deps {
	return usecase.Deps{
		Users: r.Users, Households: r.Households, Memberships: r.Memberships, Invitations: r.Invitations,
		Bots: r.Bots, Audit: r.Audit, Sessions: r.Sessions,
		Wallets: r.Wallets, Transactions: r.Transactions, Payees: r.Payees, FxRates: r.FxRates, FxOverrides: r.FxOverrides,
		Rates: r.Rates,
		IDs:   r.IDs, Tokens: r.Tokens, Clock: r.Clock, Hasher: r.Hasher, Tx: r.Tx,
	}
}
