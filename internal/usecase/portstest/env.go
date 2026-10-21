// Package portstest holds the contract tests of the repository ports. An
// adapter passes them by handing its implementations over in an Env; the
// suites know the ports only, so the same tests run against the fakes,
// SQLite and any later engine.
package portstest

import (
	"errors"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/usecase"
)

// Env is one empty, isolated set of repositories sharing a store.
type Env struct {
	Users       usecase.UserRepository
	Households  usecase.HouseholdRepository
	Memberships usecase.MembershipRepository
	Invitations usecase.InvitationRepository
	Bots        usecase.BotRepository
	Audit       usecase.AuditRepository
	Sessions    usecase.SessionRepository
	OIDCStates  usecase.OIDCStateRepository
	// Stage 002.
	Wallets      usecase.WalletRepository
	Transactions usecase.TransactionRepository
	Payees       usecase.PayeeRepository
	FxRates      usecase.FxRateRepository
	FxOverrides  usecase.FxOverrideRepository
	// Stage 003.
	EnvelopeGroups usecase.EnvelopeGroupRepository
	Envelopes      usecase.EnvelopeRepository
	Assignments    usecase.AssignmentRepository
	// Stage 004.
	Subscriptions usecase.SubscriptionRepository
	Charges       usecase.ChargeRepository
	Events        usecase.EventOutbox
	// Stage 005.
	Notifications usecase.NotificationRepository
	Channels      usecase.ChannelRepository
	Preferences   usecase.PreferenceRepository
	Deliveries    usecase.DeliveryRepository
	Tx            usecase.TxManager
	Clock         usecase.Clock
}

// Factory builds a fresh Env for one test and cleans up through t.
type Factory func(t *testing.T) Env

// now returns the environment's time in the precision repositories keep:
// microseconds, UTC.
func now(env Env) time.Time {
	return env.Clock.Now().UTC().Truncate(time.Microsecond)
}

func noError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantError(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("error = %v, want %v", err, target)
	}
}

func wantConflict(t *testing.T, err error, reason domain.ConflictReason) {
	t.Helper()
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("error = %v, want ErrConflict", err)
		return
	}
	var ce domain.ConflictError
	if !errors.As(err, &ce) || ce.Reason != reason {
		t.Errorf("error = %#v, want a ConflictError with reason %q", err, reason)
	}
}

// sameSet reports whether got and want hold the same ids, in any order.
func sameSet(t *testing.T, what string, got, want []domain.ID) {
	t.Helper()
	seen := map[domain.ID]int{}
	for _, id := range got {
		seen[id]++
	}
	ok := len(got) == len(want)
	for _, id := range want {
		ok = ok && seen[id] == 1
	}
	if !ok {
		t.Errorf("%s = %v, want %v in any order", what, got, want)
	}
}

// sameOrder reports whether got holds exactly want, in order.
func sameOrder(t *testing.T, what string, got, want []domain.ID) {
	t.Helper()
	ok := len(got) == len(want)
	for i := 0; ok && i < len(got); i++ {
		ok = got[i] == want[i]
	}
	if !ok {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}
