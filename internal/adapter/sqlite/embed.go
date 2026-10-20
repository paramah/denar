package sqlite

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations returns the embedded goose migrations with the *.sql files at the
// root, ready for Migrate, SchemaVersion and CheckSchemaCurrent.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}
