-- +goose Up
-- Stage 008-import: import batches with the transactions each created, and
-- connections to outside services with their accounts. Times are INTEGER
-- microseconds. Deleting a transaction drops its link to the batch;
-- deleting a household removes everything.

CREATE TABLE import_batches (
    id               TEXT PRIMARY KEY,
    household_id     TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    wallet_id        TEXT NOT NULL REFERENCES wallets (id) ON DELETE CASCADE,
    source           TEXT NOT NULL CHECK (source IN ('csv', 'connection')),
    source_ref       TEXT NOT NULL DEFAULT '',
    preset           TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL CHECK (status IN ('completed', 'reverted')),
    rows_total       INTEGER NOT NULL DEFAULT 0,
    created_count    INTEGER NOT NULL DEFAULT 0,
    duplicate_count  INTEGER NOT NULL DEFAULT 0,
    error_count      INTEGER NOT NULL DEFAULT 0,
    matched_count    INTEGER NOT NULL DEFAULT 0,
    reverted_count   INTEGER NOT NULL DEFAULT 0,
    errors           TEXT NOT NULL DEFAULT '[]',
    created_by_kind  TEXT NOT NULL,
    created_by_id    TEXT NOT NULL,
    created_at       INTEGER NOT NULL,
    reverted_by_kind TEXT NOT NULL DEFAULT '',
    reverted_by_id   TEXT NOT NULL DEFAULT '',
    reverted_at      INTEGER
);
CREATE INDEX idx_import_batches_household ON import_batches (household_id, id DESC);
CREATE INDEX idx_import_batches_wallet ON import_batches (wallet_id, id DESC);

CREATE TABLE import_batch_transactions (
    batch_id       TEXT NOT NULL REFERENCES import_batches (id) ON DELETE CASCADE,
    transaction_id TEXT NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    PRIMARY KEY (batch_id, transaction_id)
);
CREATE INDEX idx_import_batch_transactions_tx ON import_batch_transactions (transaction_id);

CREATE TABLE connections (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('bank', 'billing')),
    status       TEXT NOT NULL CHECK (status IN ('active', 'error', 'disabled')),
    display_name TEXT NOT NULL,
    credentials  BLOB,
    last_sync_at INTEGER,
    last_error   TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL REFERENCES users (id),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX idx_connections_household ON connections (household_id);

CREATE TABLE connection_accounts (
    connection_id TEXT NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    currency      TEXT NOT NULL DEFAULT '',
    wallet_id     TEXT REFERENCES wallets (id) ON DELETE SET NULL,
    PRIMARY KEY (connection_id, external_id)
);

-- +goose Down
DROP TABLE connection_accounts;
DROP TABLE connections;
DROP TABLE import_batch_transactions;
DROP TABLE import_batches;
