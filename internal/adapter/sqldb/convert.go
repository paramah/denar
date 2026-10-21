package sqldb

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

// pgError returns the PostgreSQL error in err, or nil.
func pgError(err error) *pgconn.PgError {
	var e *pgconn.PgError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// PostgreSQL SQLSTATE codes of constraint violations.
const (
	pgUnique     = "23505"
	pgCheck      = "23514"
	pgForeignKey = "23503"
)

// isUnique reports whether err is a UNIQUE or PRIMARY KEY violation of
// target: "table.column" or its prefix, as SQLite names it in the message
// (for example "users.email", "users.oidc_" or "wallets.").
func isUnique(err error, target string) bool {
	switch sqliteCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return strings.Contains(err.Error(), target)
	}
	if e := pgError(err); e != nil && e.Code == pgUnique {
		table, column, _ := strings.Cut(target, ".")
		return e.TableName == table && (column == "" || slices.ContainsFunc(keyColumns(e.Detail), func(c string) bool {
			return strings.HasPrefix(c, column)
		}))
	}
	return false
}

// keyColumns reads the columns of a PostgreSQL unique violation from its
// detail, "Key (household_id, lower(name))=(…) already exists.", without
// the functions of an expression index around them.
func keyColumns(detail string) []string {
	rest, ok := strings.CutPrefix(detail, "Key (")
	if !ok {
		return nil
	}
	list, _, ok := strings.Cut(rest, ")=(")
	if !ok {
		return nil
	}
	cols := strings.Split(list, ", ")
	for i, c := range cols {
		if open := strings.LastIndexByte(c, '('); open >= 0 {
			c = c[open+1:]
		}
		c, _, _ = strings.Cut(c, "::")
		cols[i] = strings.TrimRight(c, ")")
	}
	return cols
}

func isCheck(err error) bool {
	if e := pgError(err); e != nil {
		return e.Code == pgCheck
	}
	return sqliteCode(err) == sqlite3.SQLITE_CONSTRAINT_CHECK
}

func isForeignKey(err error) bool {
	if e := pgError(err); e != nil {
		return e.Code == pgForeignKey
	}
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
