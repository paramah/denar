package sqldb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paramah/denar/internal/usecase/fakes"
	"github.com/paramah/denar/internal/usecase/portstest"
)

// postgresEnv names the PostgreSQL server the tests run against; without it
// they run on SQLite only.
const postgresEnv = "DENAR_TEST_POSTGRES"

// newEnv is the portstest.Factory for SQLite: a migrated database in a
// temporary file.
func newEnv(t *testing.T) portstest.Env {
	t.Helper()
	return envOn(t, openTestDB(t))
}

// newPostgresEnv is the portstest.Factory for PostgreSQL: a migrated, fresh
// schema of its own.
func newPostgresEnv(t *testing.T) portstest.Env {
	t.Helper()
	return envOn(t, openPostgresTestDB(t))
}

// eachDialect runs a contract suite on SQLite and, when DENAR_TEST_POSTGRES
// is set, on PostgreSQL.
func eachDialect(t *testing.T, suite func(*testing.T, portstest.Factory)) {
	t.Run("sqlite", func(t *testing.T) { suite(t, newEnv) })
	t.Run("postgres", func(t *testing.T) {
		if os.Getenv(postgresEnv) == "" {
			t.Skip(postgresEnv + " not set")
		}
		suite(t, newPostgresEnv)
	})
}

func envOn(t *testing.T, db *DB) portstest.Env {
	t.Helper()
	return portstest.Env{
		Users:        NewUserRepository(db),
		Households:   NewHouseholdRepository(db),
		Memberships:  NewMembershipRepository(db),
		Invitations:  NewInvitationRepository(db),
		Bots:         NewBotRepository(db),
		Audit:        NewAuditRepository(db),
		Sessions:     NewSessionRepository(db),
		OIDCStates:   NewOIDCStateRepository(db),
		Wallets:      NewWalletRepository(db),
		Transactions: NewTransactionRepository(db),
		Payees:       NewPayeeRepository(db),
		FxRates:      NewFxRateRepository(db),
		FxOverrides:  NewFxOverrideRepository(db),

		EnvelopeGroups: NewEnvelopeGroupRepository(db),
		Envelopes:      NewEnvelopeRepository(db),
		Assignments:    NewAssignmentRepository(db),

		Subscriptions: NewSubscriptionRepository(db),
		Charges:       NewChargeRepository(db),
		Events:        NewEventOutbox(db),

		Notifications: NewNotificationRepository(db),
		Channels:      NewChannelRepository(db),
		Preferences:   NewPreferenceRepository(db),
		Deliveries:    NewDeliveryRepository(db),
		ImportBatches: NewImportBatchRepository(db),
		Connections:   NewConnectionRepository(db),
		Tx:            db,
		Clock:         fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 123456789, time.UTC)),
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "env.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db, db.Migrations()); err != nil {
		t.Fatal(err)
	}
	return db
}

// openPostgresTestDB is openPostgresSchema, migrated.
func openPostgresTestDB(t *testing.T) *DB {
	t.Helper()
	db := openPostgresSchema(t)
	if err := Migrate(context.Background(), db, db.Migrations()); err != nil {
		t.Fatal(err)
	}
	return db
}

// openPostgresSchema creates a schema with a random name on the server in
// DENAR_TEST_POSTGRES (skipping the test without it), opens a DB whose
// search_path is that schema and drops the schema when the test ends.
func openPostgresSchema(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv(postgresEnv)
	if dsn == "" {
		t.Skip(postgresEnv + " not set")
	}
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	schema := "t_" + hex.EncodeToString(raw)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestUserRepositoryContract(t *testing.T)        { eachDialect(t, portstest.RunUserSuite) }
func TestSessionRepositoryContract(t *testing.T)     { eachDialect(t, portstest.RunSessionSuite) }
func TestOIDCStateRepositoryContract(t *testing.T)   { eachDialect(t, portstest.RunOIDCStateSuite) }
func TestHouseholdRepositoryContract(t *testing.T)   { eachDialect(t, portstest.RunHouseholdSuite) }
func TestMembershipRepositoryContract(t *testing.T)  { eachDialect(t, portstest.RunMembershipSuite) }
func TestInvitationRepositoryContract(t *testing.T)  { eachDialect(t, portstest.RunInvitationSuite) }
func TestBotRepositoryContract(t *testing.T)         { eachDialect(t, portstest.RunBotSuite) }
func TestAuditRepositoryContract(t *testing.T)       { eachDialect(t, portstest.RunAuditSuite) }
func TestWalletRepositoryContract(t *testing.T)      { eachDialect(t, portstest.RunWalletSuite) }
func TestTransactionRepositoryContract(t *testing.T) { eachDialect(t, portstest.RunTransactionSuite) }
func TestPayeeRepositoryContract(t *testing.T)       { eachDialect(t, portstest.RunPayeeSuite) }
func TestFxRateRepositoryContract(t *testing.T)      { eachDialect(t, portstest.RunFxRateSuite) }
func TestFxOverrideRepositoryContract(t *testing.T)  { eachDialect(t, portstest.RunFxOverrideSuite) }
func TestEnvelopeGroupRepositoryContract(t *testing.T) {
	eachDialect(t, portstest.RunEnvelopeGroupSuite)
}
func TestEnvelopeRepositoryContract(t *testing.T)     { eachDialect(t, portstest.RunEnvelopeSuite) }
func TestAssignmentRepositoryContract(t *testing.T)   { eachDialect(t, portstest.RunAssignmentSuite) }
func TestSubscriptionRepositoryContract(t *testing.T) { eachDialect(t, portstest.RunSubscriptionSuite) }
func TestChargeRepositoryContract(t *testing.T)       { eachDialect(t, portstest.RunChargeSuite) }
func TestEventOutboxContract(t *testing.T)            { eachDialect(t, portstest.RunEventOutboxSuite) }
func TestNotificationRepositoryContract(t *testing.T) { eachDialect(t, portstest.RunNotificationSuite) }
func TestChannelRepositoryContract(t *testing.T)      { eachDialect(t, portstest.RunChannelSuite) }
func TestPreferenceRepositoryContract(t *testing.T)   { eachDialect(t, portstest.RunPreferenceSuite) }
func TestDeliveryRepositoryContract(t *testing.T)     { eachDialect(t, portstest.RunDeliverySuite) }
func TestNotificationHousekeepingContract(t *testing.T) {
	eachDialect(t, portstest.RunNotificationHousekeepingSuite)
}
func TestImportBatchRepositoryContract(t *testing.T) { eachDialect(t, portstest.RunImportBatchSuite) }
func TestConnectionRepositoryContract(t *testing.T)  { eachDialect(t, portstest.RunConnectionSuite) }
