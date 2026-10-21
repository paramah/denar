package sqldb

import (
	"embed"
	"io/fs"
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationFiles embed.FS

// Migrations returns the embedded goose migrations of d's dialect with the
// *.sql files at the root, ready for Migrate, SchemaVersion and
// CheckSchemaCurrent. Both dialects have the same versions.
func (d *DB) Migrations() fs.FS { return migrationsFor(d.dialect) }

func migrationsFor(dialect Dialect) fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations/"+string(dialect))
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}
