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
	}
}

func TestUserRepositoryContract(t *testing.T)       { portstest.RunUserSuite(t, newEnv) }
func TestSessionRepositoryContract(t *testing.T)    { portstest.RunSessionSuite(t, newEnv) }
func TestOIDCStateRepositoryContract(t *testing.T)  { portstest.RunOIDCStateSuite(t, newEnv) }
func TestHouseholdRepositoryContract(t *testing.T)  { portstest.RunHouseholdSuite(t, newEnv) }
func TestMembershipRepositoryContract(t *testing.T) { portstest.RunMembershipSuite(t, newEnv) }
func TestInvitationRepositoryContract(t *testing.T) { portstest.RunInvitationSuite(t, newEnv) }
func TestBotRepositoryContract(t *testing.T)        { portstest.RunBotSuite(t, newEnv) }
func TestAuditRepositoryContract(t *testing.T)      { portstest.RunAuditSuite(t, newEnv) }
