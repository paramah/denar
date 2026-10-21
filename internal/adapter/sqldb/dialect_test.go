package sqldb

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRebind(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`SELECT 1`, `SELECT 1`},
		{`a = ? AND b IN (?, ?)`, `a = $1 AND b IN ($2, $3)`},
		{`note LIKE ? ESCAPE '\' OR name LIKE ? ESCAPE '\'`, `note LIKE $1 ESCAPE '\' OR name LIKE $2 ESCAPE '\'`},
		{`x = '?' AND y = ?`, `x = '?' AND y = $1`},
		{`x = 'it''s ?' AND y = ?`, `x = 'it''s ?' AND y = $1`},
		{`"odd?col" = ?`, `"odd?col" = $1`},
		{"-- why?\nx = ?", "-- why?\nx = $1"},
		{`tags::jsonb @> jsonb_build_array(?::text)`, `tags::jsonb @> jsonb_build_array($1::text)`},
	} {
		if got := rebind(c.in); got != c.want {
			t.Errorf("rebind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDialectOf(t *testing.T) {
	for in, want := range map[string]Dialect{
		"denar.db":                       SQLite,
		"/data/denar.db":                 SQLite,
		"postgres://u@h/db":              Postgres,
		"postgresql://u@h/db?sslmode=on": Postgres,
	} {
		if got := DialectOf(in); got != want {
			t.Errorf("DialectOf(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestPostgresUniqueViolations(t *testing.T) {
	unique := func(table, detail string) error {
		return &pgconn.PgError{Code: pgUnique, TableName: table, Detail: detail}
	}
	for _, c := range []struct {
		err    error
		target string
		want   bool
	}{
		{unique("users", "Key (email)=(a@b.pl) already exists."), "users.email", true},
		{unique("users", "Key (email)=(a@b.pl) already exists."), "users.oidc_", false},
		{unique("users", "Key (oidc_issuer, oidc_subject)=(x, y) already exists."), "users.oidc_", true},
		{unique("wallets", "Key (household_id, lower(name))=(h, a) already exists."), "wallets.name", true},
		{unique("wallets", "Key (household_id, lower(name::text))=(h, a) already exists."), "wallets.name", true},
		{unique("wallets", "Key (household_id, lower(name))=(h, a) already exists."), "wallets.", true},
		{unique("wallets", "Key (id)=(w) already exists."), "payees.", false},
		{&pgconn.PgError{Code: pgForeignKey, TableName: "wallets"}, "wallets.", false},
		{errors.New("other"), "wallets.", false},
	} {
		if got := isUnique(c.err, c.target); got != c.want {
			t.Errorf("isUnique(%v, %q) = %v, want %v", c.err, c.target, got, c.want)
		}
	}
	if !isForeignKey(&pgconn.PgError{Code: pgForeignKey}) || !isCheck(&pgconn.PgError{Code: pgCheck}) {
		t.Error("foreign key or check violation not recognised")
	}
	if got := keyColumns("Key (household_id, lower(name))=(h, a) already exists."); !slices.Equal(got, []string{"household_id", "name"}) {
		t.Errorf("keyColumns = %v", got)
	}
}

// TestPostgresWriteTransactionsRunOneAtATime checks the advisory lock of
// WithinTx: concurrent read-then-write transactions lose no update.
func TestPostgresWriteTransactionsRunOneAtATime(t *testing.T) {
	db := openPostgresSchema(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx).ExecContext(ctx, `CREATE TABLE counter (n BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx).ExecContext(ctx, `INSERT INTO counter (n) VALUES (0)`); err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Go(func() {
			errs <- db.WithinTx(ctx, func(ctx context.Context) error {
				var n int64
				if err := db.Query(ctx).QueryRowContext(ctx, `SELECT n FROM counter`).Scan(&n); err != nil {
					return err
				}
				_, err := db.Exec(ctx).ExecContext(ctx, `UPDATE counter SET n = ?`, n+1)
				return err
			})
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	if err := db.Query(ctx).QueryRowContext(ctx, `SELECT n FROM counter`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != workers {
		t.Errorf("counter = %d, want %d: transactions overlapped", n, workers)
	}
}
