package sqldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// copyTables lists every table of the schema in an order that inserts rows
// after the rows their foreign keys point to. TestCopyTablesCoverSchema
// keeps it complete.
var copyTables = []string{
	"users", "households", "sessions", "oidc_login_states", "memberships", "invitations",
	"bots", "bot_wallets", "audit_log",
	"wallets", "wallet_members", "payees", "envelope_groups", "envelopes",
	"transactions", "transaction_splits", "envelope_assignments", "fx_rates", "fx_rate_overrides",
	"subscriptions", "subscription_prices", "subscription_charges", "domain_events",
	"notifications", "notification_channels", "notification_preferences", "notification_deliveries",
	"import_batches", "import_batch_transactions", "connections", "connection_accounts",
}

// maxCopyArgs bounds the bound values of one INSERT, below the limits of
// both databases (SQLite 32766, PostgreSQL 65535).
const maxCopyArgs = 30000

// CopyCount is the number of rows one table has in the source and the
// target after a copy.
type CopyCount struct {
	Table          string
	Source, Target int64
}

// ErrTargetNotEmpty means the target of Copy already holds data.
var ErrTargetNotEmpty = errors.New("the target database is not empty")

// Copy copies every row of src into dst, which must be empty. Both must be
// at the newest schema version (migrate dst first). It reads src in one
// read transaction and writes dst in one transaction, so a failure leaves
// dst empty; then it counts the rows of every table on both sides.
func Copy(ctx context.Context, src, dst *DB) ([]CopyCount, error) {
	for _, db := range []*DB{src, dst} {
		if err := CheckSchemaCurrent(ctx, db, db.Migrations()); err != nil {
			return nil, fmt.Errorf("%s: %w", db.dialect, err)
		}
	}
	read, err := src.reader.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("copy: read: %w", err)
	}
	defer func() { _ = read.Rollback() }()
	from := src.wrap(read)

	err = dst.WithinTx(ctx, func(ctx context.Context) error {
		to := dst.Exec(ctx)
		for _, table := range copyTables {
			n, err := count(ctx, to, table)
			if err != nil {
				return err
			}
			if n > 0 {
				return fmt.Errorf("%w: %s has %d rows", ErrTargetNotEmpty, table, n)
			}
		}
		for _, table := range copyTables {
			if err := copyTable(ctx, from, to, table); err != nil {
				return fmt.Errorf("copy %s: %w", table, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	counts := make([]CopyCount, 0, len(copyTables))
	var mismatch []string
	for _, table := range copyTables {
		c := CopyCount{Table: table}
		if c.Source, err = count(ctx, from, table); err != nil {
			return nil, err
		}
		if c.Target, err = count(ctx, dst.Query(ctx), table); err != nil {
			return nil, err
		}
		if c.Source != c.Target {
			mismatch = append(mismatch, fmt.Sprintf("%s %d → %d", table, c.Source, c.Target))
		}
		counts = append(counts, c)
	}
	if len(mismatch) > 0 {
		return counts, fmt.Errorf("copy: row counts differ: %s", strings.Join(mismatch, ", "))
	}
	return counts, nil
}

func count(ctx context.Context, q dbtx, table string) (int64, error) {
	var n int64
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return n, nil
}

// copyTable copies the rows of one table by column name, in batches. The
// target's column types decide the conversions: SQLite keeps booleans as
// 0 and 1, which go into PostgreSQL BOOLEAN columns as false and true.
func copyTable(ctx context.Context, from, to dbtx, table string) error {
	boolean, err := booleanColumns(ctx, to, table)
	if err != nil {
		return err
	}
	rows, err := from.QueryContext(ctx, `SELECT * FROM `+table)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	perBatch := max(1, maxCopyArgs/len(cols))
	row := "(" + placeholders(len(cols)) + ")"
	prefix := `INSERT INTO ` + table + ` (` + strings.Join(cols, ", ") + `) VALUES `

	var (
		args []any
		n    int
	)
	flush := func() error {
		if n == 0 {
			return nil
		}
		_, err := to.ExecContext(ctx, prefix+strings.TrimSuffix(strings.Repeat(row+", ", n), ", "), args...)
		args, n = args[:0], 0
		return err
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, c := range cols {
			if v, ok := vals[i].(int64); ok && boolean[c] {
				vals[i] = v != 0
			}
		}
		args = append(args, vals...)
		if n++; n == perBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return flush()
}

// booleanColumns names the BOOLEAN columns of table in the target.
func booleanColumns(ctx context.Context, to dbtx, table string) (map[string]bool, error) {
	rows, err := to.QueryContext(ctx, `SELECT * FROM `+table+` WHERE 1 = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, ct := range types {
		switch strings.ToUpper(ct.DatabaseTypeName()) {
		case "BOOL", "BOOLEAN":
			out[ct.Name()] = true
		}
	}
	return out, rows.Err()
}
