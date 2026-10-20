package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"strings"
	"time"

	"modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

// The driver keeps registered functions for every connection it opens
// later, so registering once at start-up is enough (design §4.4.2). fold
// lower-cases text, also outside ASCII, which SQLite's own lower() does not.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("fold", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			switch v := args[0].(type) {
			case string:
				return strings.ToLower(v), nil
			case []byte:
				return strings.ToLower(string(v)), nil
			}
			return args[0], nil
		})
}

const busyTimeoutMS = 5000

// dbtx is the subset of *sql.DB / *sql.Tx used by repositories.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB holds two pools on one SQLite file in WAL mode: a single-connection
// writer and a read-only reader (design §5.4).
type DB struct {
	writer *sql.DB
	reader *sql.DB
}

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	pragmas := []string{
		fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS),
		"foreign_keys(1)",
		"journal_mode(WAL)",
		"synchronous(FULL)",
		// Makes REPLACE fire delete triggers, so that the triggers keeping
		// the audit log append-only cannot be bypassed with it.
		"recursive_triggers(1)",
	}
	if readOnly {
		pragmas = append(pragmas, "query_only(1)")
	} else {
		// A write transaction takes the write lock when it begins. Use
		// cases read before they write; with a deferred lock a commit by
		// another process in between fails the write at once
		// (SQLITE_BUSY_SNAPSHOT), without waiting for busy_timeout.
		q.Set("_txlock", "immediate")
	}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	u := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath(), RawQuery: q.Encode()}
	return u.String()
}

// Open opens (creating if needed) the database file at path.
func Open(path string) (*DB, error) {
	writer, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, fmt.Errorf("sqlite: open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)
	// Force the first connection now: creates the file and switches it to WAL
	// before the reader opens it.
	if err := writer.Ping(); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("sqlite: open writer: %w", err)
	}
	reader, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("sqlite: open reader: %w", err)
	}
	reader.SetMaxOpenConns(4)
	reader.SetConnMaxIdleTime(5 * time.Minute)
	if err := reader.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("sqlite: open reader: %w", err)
	}
	return &DB{writer: writer, reader: reader}, nil
}

// Close checkpoints the WAL and closes both pools.
func (d *DB) Close() error {
	var first error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := d.writer.ExecContext(ctx, "pragma wal_checkpoint(TRUNCATE)"); err != nil {
		first = fmt.Errorf("sqlite: checkpoint: %w", err)
	}
	if err := d.reader.Close(); err != nil && first == nil {
		first = err
	}
	if err := d.writer.Close(); err != nil && first == nil {
		first = err
	}
	return first
}

// Ping checks that both pools reach the database.
func (d *DB) Ping(ctx context.Context) error {
	if err := d.writer.PingContext(ctx); err != nil {
		return err
	}
	return d.reader.PingContext(ctx)
}
