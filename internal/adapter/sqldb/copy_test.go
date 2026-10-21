package sqldb

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

// TestCopyTablesCoverSchema: Copy knows every table the migrations create.
func TestCopyTablesCoverSchema(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	rows, err := db.Query(ctx).QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'goose_db_version' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	listed := slices.Sorted(slices.Values(copyTables))
	if !slices.Equal(tables, listed) {
		t.Errorf("schema tables %v\ncopyTables    %v", tables, listed)
	}
}

func seedCopySource(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`INSERT INTO users (id, email, name, instance_admin, active, created_at) VALUES ('u1', 'a@x.pl', 'Ala', TRUE, TRUE, 1), ('u2', 'b@x.pl', 'Ola', FALSE, FALSE, 2)`,
		`INSERT INTO households (id, name, base_currency, timezone, month_start_day, created_by, created_at) VALUES ('h1', 'Dom', 'PLN', 'Europe/Warsaw', 1, 'u1', 3)`,
		`INSERT INTO wallets (id, household_id, name, type, currency, on_budget, visibility, created_by, created_at) VALUES ('w1', 'h1', 'Konto', 'checking', 'PLN', TRUE, 'shared', 'u1', 4)`,
		`INSERT INTO notification_channels (id, user_id, type, name, secrets, enabled, created_at, updated_at) VALUES ('c1', 'u1', 'email', 'Poczta', X'00ff10', FALSE, 5, 5)`,
		`INSERT INTO audit_log (id, household_id, at, actor_kind, actor_id, action, resource_type, resource_id) VALUES ('a1', 'h1', 6, 'user', 'u1', 'household.create', 'household', 'h1')`,
	} {
		if _, err := db.Exec(ctx).ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCopy(t *testing.T) {
	targets := map[string]func(*testing.T) *DB{
		"sqlite":   openTestDB,
		"postgres": openPostgresTestDB,
	}
	for name, open := range targets {
		t.Run("sqlite to "+name, func(t *testing.T) {
			ctx := context.Background()
			src := openTestDB(t)
			seedCopySource(t, src)
			dst := open(t)
			counts, err := Copy(ctx, src, dst)
			if err != nil {
				t.Fatal(err)
			}
			if i := slices.IndexFunc(counts, func(c CopyCount) bool { return c.Table == "users" }); i < 0 || counts[i].Target != 2 {
				t.Errorf("counts = %+v", counts)
			}
			var (
				admin, active, enabled bool
				secrets                []byte
			)
			if err := dst.Query(ctx).QueryRowContext(ctx, `SELECT instance_admin, active FROM users WHERE id = 'u2'`).Scan(&admin, &active); err != nil {
				t.Fatal(err)
			}
			if err := dst.Query(ctx).QueryRowContext(ctx, `SELECT enabled, secrets FROM notification_channels WHERE id = 'c1'`).Scan(&enabled, &secrets); err != nil {
				t.Fatal(err)
			}
			if admin || active || enabled || string(secrets) != "\x00\xff\x10" {
				t.Errorf("copied values: admin %v active %v enabled %v secrets %x", admin, active, enabled, secrets)
			}
			u, err := NewUserRepository(dst).ByID(ctx, "u1")
			if err != nil || !u.InstanceAdmin || !u.Active || u.Email != "a@x.pl" {
				t.Errorf("user read back = %+v, %v", u, err)
			}

			if _, err := Copy(ctx, src, dst); !errors.Is(err, ErrTargetNotEmpty) {
				t.Errorf("second copy: err = %v, want ErrTargetNotEmpty", err)
			}
		})
	}
}

func TestCopyNeedsMigratedDatabases(t *testing.T) {
	src := openTestDB(t)
	dst, err := Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if _, err := Copy(context.Background(), src, dst); err == nil {
		t.Error("copy into an unmigrated database succeeded")
	}
}
