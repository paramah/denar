package sqldb

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// Dialect names the database a DB runs on.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

// DialectOf tells by the target of DENAR_DB which database it names: a
// postgres:// or postgresql:// URL is PostgreSQL, anything else a SQLite file.
func DialectOf(target string) Dialect {
	if strings.HasPrefix(target, "postgres://") || strings.HasPrefix(target, "postgresql://") {
		return Postgres
	}
	return SQLite
}

// Repositories write SQL that both databases run, with ? placeholders. The
// few constructs that differ come from these methods.

// fold lower-cases text for case-insensitive search; SQLite's lower() is
// ASCII only, so it uses the fold function registered in db.go.
func (d Dialect) fold(expr string) string {
	if d == Postgres {
		return "lower(" + expr + ")"
	}
	return "fold(" + expr + ")"
}

// nocase orders text without regard to case, as COLLATE NOCASE does.
func (d Dialect) nocase(expr string) string {
	if d == Postgres {
		return "lower(" + expr + ")"
	}
	return expr + " COLLATE NOCASE"
}

// hasTag matches rows whose JSON array column holds the bound string.
func (d Dialect) hasTag(column string) string {
	if d == Postgres {
		return "(" + column + ")::jsonb @> jsonb_build_array(?::text)"
	}
	return "EXISTS (SELECT 1 FROM json_each(" + column + ") WHERE json_each.value = ?)"
}

// rebind turns ? placeholders into PostgreSQL's $1, $2, …. A ? inside a
// string literal, a quoted identifier or a comment stays as it is.
func rebind(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(query) {
				if query[j] == c {
					if j+1 < len(query) && query[j+1] == c { // doubled quote
						j += 2
						continue
					}
					break
				}
				j++
			}
			b.WriteString(query[i:min(j+1, len(query))])
			i = j
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			j := strings.IndexByte(query[i:], '\n')
			if j < 0 {
				j = len(query) - i
			}
			b.WriteString(query[i : i+j])
			i += j - 1
		case c == '?':
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// pgExec rewrites placeholders before handing statements to a PostgreSQL
// pool or transaction.
type pgExec struct{ inner dbtx }

func (p pgExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.inner.ExecContext(ctx, rebind(query), args...)
}

func (p pgExec) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.inner.QueryContext(ctx, rebind(query), args...)
}

func (p pgExec) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.inner.QueryRowContext(ctx, rebind(query), args...)
}
