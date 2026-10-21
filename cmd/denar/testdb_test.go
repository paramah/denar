package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// With DENAR_TEST_E2E_POSTGRES set, the end-to-end tests run on a fresh
// schema of the PostgreSQL server in DENAR_TEST_POSTGRES instead of a SQLite
// file (task test:postgres).
const (
	postgresEnv    = "DENAR_TEST_POSTGRES"
	e2ePostgresEnv = "DENAR_TEST_E2E_POSTGRES"
)

// testDatabase returns the DENAR_DB of a fresh, empty database for one test.
func testDatabase(t *testing.T) string {
	t.Helper()
	if os.Getenv(e2ePostgresEnv) == "" {
		return filepath.Join(t.TempDir(), "denar.db")
	}
	return postgresSchema(t)
}

// postgresSchema creates a schema with a random name on the server in
// DENAR_TEST_POSTGRES, drops it when the test ends and returns a URL whose
// search_path is that schema. Without the server the test is skipped.
func postgresSchema(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(postgresEnv)
	if dsn == "" {
		t.Skip(postgresEnv + " not set")
	}
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	schema := "e2e_" + hex.EncodeToString(raw)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
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
	return u.String()
}
