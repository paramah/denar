// Package sqlite implements the usecase repositories on SQLite
// (modernc.org/sqlite, no cgo).
//
// The database runs in WAL mode with two pools on one file: a writer with a
// single connection, whose transactions take the write lock when they begin
// (_txlock=immediate), and a read-only reader pool (query_only). WithinTx
// puts the transaction into the context; repositories read and write
// through it when it is there (read-your-writes) and through the pools
// otherwise. Migrations are goose SQL files embedded from migrations/.
// IDs are TEXT (ULID), times INTEGER microseconds since the Unix epoch (UTC).
package sqlite
