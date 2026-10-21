// Package sqldb implements the usecase repositories on SQLite
// (modernc.org/sqlite) and PostgreSQL (jackc/pgx), both without cgo. Open
// picks the database by the target: a postgres:// URL or a SQLite file.
//
// On SQLite the database runs in WAL mode with two pools on one file: a
// writer with a single connection, whose transactions take the write lock
// when they begin (_txlock=immediate), and a read-only reader pool
// (query_only). On PostgreSQL one pool serves both, and every write
// transaction first takes an advisory lock, so that write transactions run
// one at a time there as well. WithinTx puts the transaction into the
// context; repositories read and write through it when it is there
// (read-your-writes) and through the pools otherwise.
//
// Repositories write SQL both databases run, with ? placeholders (rewritten
// to $n for PostgreSQL); the few constructs that differ come from Dialect.
// Migrations are goose SQL files embedded from migrations/<dialect>/, with
// the same versions for both. IDs are TEXT (ULID), times integer
// microseconds since the Unix epoch (UTC).
package sqldb
