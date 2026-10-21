package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/paramah/denar/internal/adapter/sqldb"
)

// TestDBCopy: denar db copy moves the demo household from a SQLite file into
// an empty database, SQLite or PostgreSQL, with every row of every table
// unchanged, and refuses to copy into it a second time.
func TestDBCopy(t *testing.T) {
	targets := map[string]func(*testing.T) string{
		"sqlite":   func(t *testing.T) string { return filepath.Join(t.TempDir(), "target.db") },
		"postgres": postgresSchema,
	}
	for name, target := range targets {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			from := filepath.Join(t.TempDir(), "source.db")
			src, err := sqldb.Open(from)
			if err != nil {
				t.Fatal(err)
			}
			defer src.Close()
			if err := sqldb.Migrate(ctx, src, src.Migrations()); err != nil {
				t.Fatal(err)
			}
			if _, err := seedDemo(ctx, src, slog.New(slog.DiscardHandler), time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), demoPassword, false); err != nil {
				t.Fatal(err)
			}

			to := target(t)
			setEnv(t, "DENAR_DB", to)
			var out bytes.Buffer
			if err := run(ctx, []string{"db", "copy", "--from", from}, &out, io.Discard); err != nil {
				t.Fatalf("%v\n%s", err, out.String())
			}
			if !strings.Contains(out.String(), "copied ") || !strings.Contains(out.String(), "transactions") {
				t.Errorf("output = %s", out.String())
			}

			dst, err := sqldb.Open(to)
			if err != nil {
				t.Fatal(err)
			}
			defer dst.Close()
			for _, table := range tableNames(t, src) {
				want, got := tableRows(t, src, table), tableRows(t, dst, table)
				if !slices.Equal(want, got) {
					t.Errorf("%s differs:\nsource %v\ntarget %v", table, want, got)
				}
			}

			if err := run(ctx, []string{"db", "copy", "--from", from}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "not empty") {
				t.Errorf("second copy: err = %v, want the target to be not empty", err)
			}
		})
	}
}

func tableNames(t *testing.T, db *sqldb.DB) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.Query(ctx).QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'goose_db_version'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	return out
}

// tableRows renders every row of a table, sorted, with booleans as 0 and 1
// and bytes as hex, so that the dialects compare equal.
func tableRows(t *testing.T, db *sqldb.DB, table string) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.Query(ctx).QueryContext(ctx, `SELECT * FROM `+table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			switch v := v.(type) {
			case bool:
				parts[i] = map[bool]string{false: "0", true: "1"}[v]
			case []byte:
				parts[i] = hex.EncodeToString(v)
			default:
				parts[i] = fmt.Sprint(v)
			}
			parts[i] = cols[i] + "=" + parts[i]
		}
		out = append(out, strings.Join(parts, " "))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}
