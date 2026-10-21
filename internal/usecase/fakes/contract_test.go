package fakes

import (
	"testing"

	"git.eiac.dev/tools/denar/internal/usecase/portstest"
)

func newEnv(*testing.T) portstest.Env {
	r := NewRepos()
	return portstest.Env{
		Users: r.Users, Households: r.Households, Memberships: r.Memberships, Invitations: r.Invitations,
		Bots: r.Bots, Audit: r.Audit, Sessions: r.Sessions, OIDCStates: r.OIDCStates, Tx: r.Tx, Clock: r.Clock,
		Wallets: r.Wallets, Transactions: r.Transactions, Payees: r.Payees, FxRates: r.FxRates, FxOverrides: r.FxOverrides,
		EnvelopeGroups: r.EnvelopeGroups, Envelopes: r.Envelopes, Assignments: r.Assignments,
		Subscriptions: r.Subscriptions, Charges: r.Charges, Events: r.Events,
		Notifications: r.Notifications, Channels: r.Channels, Preferences: r.Preferences, Deliveries: r.Deliveries,
	}
}

func TestUserRepositoryContract(t *testing.T)          { portstest.RunUserSuite(t, newEnv) }
func TestSessionRepositoryContract(t *testing.T)       { portstest.RunSessionSuite(t, newEnv) }
func TestOIDCStateRepositoryContract(t *testing.T)     { portstest.RunOIDCStateSuite(t, newEnv) }
func TestHouseholdRepositoryContract(t *testing.T)     { portstest.RunHouseholdSuite(t, newEnv) }
func TestMembershipRepositoryContract(t *testing.T)    { portstest.RunMembershipSuite(t, newEnv) }
func TestInvitationRepositoryContract(t *testing.T)    { portstest.RunInvitationSuite(t, newEnv) }
func TestBotRepositoryContract(t *testing.T)           { portstest.RunBotSuite(t, newEnv) }
func TestAuditRepositoryContract(t *testing.T)         { portstest.RunAuditSuite(t, newEnv) }
func TestWalletRepositoryContract(t *testing.T)        { portstest.RunWalletSuite(t, newEnv) }
func TestTransactionRepositoryContract(t *testing.T)   { portstest.RunTransactionSuite(t, newEnv) }
func TestPayeeRepositoryContract(t *testing.T)         { portstest.RunPayeeSuite(t, newEnv) }
func TestFxRateRepositoryContract(t *testing.T)        { portstest.RunFxRateSuite(t, newEnv) }
func TestFxOverrideRepositoryContract(t *testing.T)    { portstest.RunFxOverrideSuite(t, newEnv) }
func TestEnvelopeGroupRepositoryContract(t *testing.T) { portstest.RunEnvelopeGroupSuite(t, newEnv) }
func TestEnvelopeRepositoryContract(t *testing.T)      { portstest.RunEnvelopeSuite(t, newEnv) }
func TestAssignmentRepositoryContract(t *testing.T)    { portstest.RunAssignmentSuite(t, newEnv) }
func TestSubscriptionRepositoryContract(t *testing.T)  { portstest.RunSubscriptionSuite(t, newEnv) }
func TestChargeRepositoryContract(t *testing.T)        { portstest.RunChargeSuite(t, newEnv) }
func TestEventOutboxContract(t *testing.T)             { portstest.RunEventOutboxSuite(t, newEnv) }
func TestNotificationRepositoryContract(t *testing.T)  { portstest.RunNotificationSuite(t, newEnv) }
func TestChannelRepositoryContract(t *testing.T)       { portstest.RunChannelSuite(t, newEnv) }
func TestPreferenceRepositoryContract(t *testing.T)    { portstest.RunPreferenceSuite(t, newEnv) }
func TestDeliveryRepositoryContract(t *testing.T)      { portstest.RunDeliverySuite(t, newEnv) }
func TestNotificationHousekeepingContract(t *testing.T) {
	portstest.RunNotificationHousekeepingSuite(t, newEnv)
}
