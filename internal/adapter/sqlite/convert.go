package sqlite

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/paramah/denar/internal/domain"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Times are stored as INTEGER microseconds since the Unix epoch (UTC).

func toMicros(t time.Time) int64 { return t.UTC().UnixMicro() }

func fromMicros(us int64) time.Time { return time.UnixMicro(us).UTC() }

// nullString stores "" as NULL.
func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// sqliteCode returns the extended result code of a driver error, or 0.
func sqliteCode(err error) int {
	var e *sqlite.Error
	if errors.As(err, &e) {
		return e.Code()
	}
	return 0
}

// isUnique reports whether err is a UNIQUE or PRIMARY KEY violation whose
// message names target (for example "users.email").
func isUnique(err error, target string) bool {
	switch sqliteCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return strings.Contains(err.Error(), target)
	}
	return false
}

func isCheck(err error) bool {
	return sqliteCode(err) == sqlite3.SQLITE_CONSTRAINT_CHECK
}

func isForeignKey(err error) bool {
	return sqliteCode(err) == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

// notFound turns "no rows" into domain.ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// affected returns domain.ErrNotFound when the statement changed no row.
func affected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// nullMicros stores the zero time as NULL.
func nullMicros(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: toMicros(t), Valid: true}
}

// fromNullMicros reads NULL as the zero time.
func fromNullMicros(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return fromMicros(v.Int64)
}

// placeholders returns "?, ?, …" for n values.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func idArgs(ids []domain.ID) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
