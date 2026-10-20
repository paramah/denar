package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.TxManager = (*DB)(nil)

type txKey struct{}

// WithinTx implements usecase.TxManager (design §5.4). If ctx already carries
// a transaction, fn joins it. Otherwise it takes the single writer connection,
// runs fn in a transaction and commits, or rolls back on error or panic.
// Inside fn always use the ctx passed to fn.
func (d *DB) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}
	tx, err := d.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin: %w", err)
	}
	// Rolls back on every way out that is not a commit or an explicit
	// rollback: a panic and runtime.Goexit (t.Fatal inside fn) included.
	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback()
		}
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		finished = true
		_ = tx.Rollback()
		return err
	}
	finished = true
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// Exec returns the executor for writes: the transaction in ctx, else the writer pool.
func (d *DB) Exec(ctx context.Context) dbtx {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return d.writer
}

// Query returns the executor for reads: the transaction in ctx (read-your-writes), else the reader pool.
func (d *DB) Query(ctx context.Context) dbtx {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return d.reader
}
