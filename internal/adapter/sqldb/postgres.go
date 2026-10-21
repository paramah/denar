package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // pure-Go driver, registered as "pgx"
)

// writeLockKey is the advisory lock every PostgreSQL write transaction takes
// first (014 design AD-2): the transactions run one at a time, as they do on
// SQLite's single writer, also across processes.
const writeLockKey int64 = 0x64656e6172 // "denar"

func openPostgres(dsn string) (*DB, error) {
	pool, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	return &DB{dialect: Postgres, writer: pool, reader: pool}, nil
}
