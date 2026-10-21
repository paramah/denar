package sqldb

import (
	"context"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMigrateAndCheckSchema(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	checkMigrate(t, db)
}

func TestMigrateAndCheckSchemaPostgres(t *testing.T) {
	checkMigrate(t, openPostgresSchema(t))
}

func checkMigrate(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	if err := CheckSchemaCurrent(ctx, db, db.Migrations()); err == nil || !strings.Contains(err.Error(), "denar migrate") {
		t.Errorf("fresh database: err = %v, want a hint to run denar migrate", err)
	}
	if err := Migrate(ctx, db, db.Migrations()); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, db.Migrations()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if err := CheckSchemaCurrent(ctx, db, db.Migrations()); err != nil {
		t.Error(err)
	}
	cur, newest, err := SchemaVersion(ctx, db, db.Migrations())
	if err != nil || cur != newest || cur < 1 {
		t.Errorf("version %d/%d, %v", cur, newest, err)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) { checkAuditAppendOnly(t, openTestDB(t)) })
	t.Run("postgres", func(t *testing.T) { checkAuditAppendOnly(t, openPostgresTestDB(t)) })
}

func checkAuditAppendOnly(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	_, err := db.Exec(ctx).ExecContext(ctx, `UPDATE audit_log SET action = 'x'`)
	if err != nil {
		t.Fatalf("an update of no rows fails: %v", err)
	}
	for _, q := range []string{
		`INSERT INTO users (id, email, name, created_at) VALUES ('u1', 'u1@x.pl', 'U', 0)`,
		`INSERT INTO households (id, name, base_currency, timezone, month_start_day, created_by, created_at) VALUES ('h1', 'D', 'PLN', 'UTC', 1, 'u1', 0)`,
		`INSERT INTO audit_log (id, household_id, at, actor_kind, actor_id, action, resource_type, resource_id) VALUES ('a1', 'h1', 0, 'user', 'u1', 'x', 'household', 'h1')`,
	} {
		if _, err := db.Exec(ctx).ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx).ExecContext(ctx, `UPDATE audit_log SET action = 'changed'`); err == nil {
		t.Error("audit entry updated")
	}
}

// TestMigrationVersionsMatch keeps the dialects on the same schema
// versions: every migration exists for both, under the same name.
func TestMigrationVersionsMatch(t *testing.T) {
	names := func(d Dialect) []string {
		entries, err := fs.ReadDir(migrationsFor(d), ".")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if lite, pg := names(SQLite), names(Postgres); !slices.Equal(lite, pg) {
		t.Errorf("migrations differ:\nsqlite   %v\npostgres %v", lite, pg)
	}
}
