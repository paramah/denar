package sqldb

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// provider builds a goose provider on the writer pool. fsys must contain the
// *.sql files at its root (use fs.Sub for an embedded directory). The provider
// is deliberately not closed: Close would close the shared writer pool.
func provider(d *DB, fsys fs.FS) (*goose.Provider, error) {
	dialect := goose.DialectSQLite3
	if d.dialect == Postgres {
		dialect = goose.DialectPostgres
	}
	return goose.NewProvider(dialect, d.writer, fsys)
}

// Migrate applies all pending migrations from fsys. Repeated calls are no-ops.
func Migrate(ctx context.Context, d *DB, fsys fs.FS) error {
	p, err := provider(d, fsys)
	if err != nil {
		return fmt.Errorf("sqldb: migrate: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("sqldb: migrate: %w", err)
	}
	return nil
}

// SchemaVersion returns the version recorded in the database and the newest
// version available in fsys.
func SchemaVersion(ctx context.Context, d *DB, fsys fs.FS) (current, newest int64, err error) {
	p, err := provider(d, fsys)
	if err != nil {
		return 0, 0, fmt.Errorf("sqldb: schema version: %w", err)
	}
	current, newest, err = p.GetVersions(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("sqldb: schema version: %w", err)
	}
	return current, newest, nil
}

// CheckSchemaCurrent fails unless the database is at the newest version in
// fsys. A database that is behind needs migrate; one that is ahead was
// written by a newer binary, and only that binary can serve it.
func CheckSchemaCurrent(ctx context.Context, d *DB, fsys fs.FS) error {
	cur, newest, err := SchemaVersion(ctx, d, fsys)
	if err != nil {
		return err
	}
	switch {
	case cur < newest:
		return fmt.Errorf("database schema is at version %d, this binary needs %d: run `denar migrate`", cur, newest)
	case cur > newest:
		return fmt.Errorf("database schema is at version %d, this binary knows only %d: the database was migrated by a newer denar, run that version", cur, newest)
	}
	return nil
}
