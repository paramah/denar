package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/usecase/fakes"
	"git.eiac.dev/tools/denar/internal/usecase/portstest"
)

// newEnv is the portstest.Factory for SQLite: a migrated database in a
// temporary file.
func newEnv(t *testing.T) portstest.Env {
	t.Helper()
	db := openTestDB(t)
	return portstest.Env{
		Users:       NewUserRepository(db),
		Households:  NewHouseholdRepository(db),
		Memberships: NewMembershipRepository(db),
		Invitations: NewInvitationRepository(db),
		Bots:        NewBotRepository(db),
		Audit:       NewAuditRepository(db),
		Sessions:    NewSessionRepository(db),
		OIDCStates:  NewOIDCStateRepository(db),
		Tx:          db,
		Clock:       fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 123456789, time.UTC)),
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "env.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db, Migrations()); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUserRepositoryContract(t *testing.T)       { portstest.RunUserSuite(t, newEnv) }
func TestSessionRepositoryContract(t *testing.T)    { portstest.RunSessionSuite(t, newEnv) }
func TestOIDCStateRepositoryContract(t *testing.T)  { portstest.RunOIDCStateSuite(t, newEnv) }
func TestHouseholdRepositoryContract(t *testing.T)  { portstest.RunHouseholdSuite(t, newEnv) }
func TestMembershipRepositoryContract(t *testing.T) { portstest.RunMembershipSuite(t, newEnv) }
func TestInvitationRepositoryContract(t *testing.T) { portstest.RunInvitationSuite(t, newEnv) }
func TestBotRepositoryContract(t *testing.T)        { portstest.RunBotSuite(t, newEnv) }
func TestAuditRepositoryContract(t *testing.T)      { portstest.RunAuditSuite(t, newEnv) }
