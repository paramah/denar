-- +goose Up
-- Stage 002-wallets-transactions (PostgreSQL): wallets, payees, transactions with their
-- split lines, the cache of NBP rates and rates set by hand. Amounts are
-- BIGINT minor units; dates of transactions and rates are TEXT
-- 'YYYY-MM-DD'; rates are TEXT decimals. Envelope ids have no foreign key
-- yet: envelopes come with stage 003.

CREATE TABLE wallets (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL CHECK (type IN ('cash', 'checking', 'card', 'savings', 'investment', 'loan')),
    currency     TEXT NOT NULL CHECK (length(currency) = 3),
    on_budget    BOOLEAN NOT NULL,
    visibility   TEXT NOT NULL CHECK (visibility IN ('shared', 'private')),
    created_by   TEXT NOT NULL REFERENCES users (id),
    created_at   BIGINT NOT NULL,
    archived_at  BIGINT
);
CREATE UNIQUE INDEX idx_wallets_name ON wallets (household_id, lower(name));

-- Who sees a private wallet; its creator is always listed.
CREATE TABLE wallet_members (
    wallet_id TEXT NOT NULL REFERENCES wallets (id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL REFERENCES users (id),
    PRIMARY KEY (wallet_id, user_id)
);

CREATE TABLE payees (
    id                  TEXT PRIMARY KEY,
    household_id        TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    -- JSON array of plain substrings.
    patterns            TEXT NOT NULL DEFAULT '[]',
    default_envelope_id TEXT,
    created_at          BIGINT NOT NULL,
    updated_at          BIGINT NOT NULL
);
CREATE UNIQUE INDEX idx_payees_name ON payees (household_id, lower(name));

CREATE TABLE transactions (
    id                 TEXT PRIMARY KEY,
    household_id       TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    wallet_id          TEXT NOT NULL REFERENCES wallets (id) ON DELETE CASCADE,
    date               TEXT NOT NULL CHECK (length(date) = 10),
    amount             BIGINT NOT NULL,
    currency           TEXT NOT NULL CHECK (length(currency) = 3),
    -- The amount in the household's base currency at the rate of the day;
    -- NULL while the rate is pending.
    base_amount        BIGINT,
    rate_pending       BOOLEAN NOT NULL,
    fx_rate            TEXT,
    payee_id           TEXT REFERENCES payees (id) ON DELETE SET NULL,
    envelope_id        TEXT,
    kind               TEXT NOT NULL CHECK (kind IN ('expense', 'income', 'transfer')),
    status             TEXT NOT NULL CHECK (status IN ('pending', 'cleared', 'reconciled')),
    source             TEXT NOT NULL CHECK (source IN ('manual', 'import', 'subscription', 'bot')),
    -- JSON array of lower-case tags.
    tags               TEXT NOT NULL DEFAULT '[]',
    note               TEXT NOT NULL DEFAULT '',
    external_id        TEXT,
    transfer_pair_id   TEXT,
    transfer_wallet_id TEXT,
    created_by_kind    TEXT NOT NULL CHECK (created_by_kind IN ('user', 'bot', 'system')),
    created_by_id      TEXT NOT NULL,
    created_at         BIGINT NOT NULL,
    updated_at         BIGINT NOT NULL,
    CHECK (rate_pending = (base_amount IS NULL)),
    CHECK ((kind = 'transfer') = (transfer_pair_id IS NOT NULL))
);
CREATE UNIQUE INDEX idx_transactions_external ON transactions (wallet_id, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX idx_transactions_household ON transactions (household_id, date, id);
CREATE INDEX idx_transactions_wallet    ON transactions (wallet_id, date, id);
CREATE INDEX idx_transactions_pair      ON transactions (transfer_pair_id) WHERE transfer_pair_id IS NOT NULL;
CREATE INDEX idx_transactions_envelope  ON transactions (envelope_id, date) WHERE envelope_id IS NOT NULL;
CREATE INDEX idx_transactions_payee     ON transactions (payee_id) WHERE payee_id IS NOT NULL;
CREATE INDEX idx_transactions_pending   ON transactions (rate_pending, id) WHERE rate_pending;

CREATE TABLE transaction_splits (
    transaction_id TEXT NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    position       BIGINT NOT NULL,
    envelope_id    TEXT,
    amount         BIGINT NOT NULL,
    base_amount    BIGINT,
    note           TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (transaction_id, position)
);
CREATE INDEX idx_transaction_splits_envelope ON transaction_splits (envelope_id) WHERE envelope_id IS NOT NULL;

-- NBP table A rates for every household: PLN per one unit of currency on
-- rate_date, the business day the requested date falls back to.
CREATE TABLE fx_rates (
    currency   TEXT NOT NULL CHECK (length(currency) = 3),
    date       TEXT NOT NULL,
    rate_date  TEXT NOT NULL,
    rate       TEXT NOT NULL,
    fetched_at BIGINT NOT NULL,
    PRIMARY KEY (currency, date)
);
CREATE INDEX idx_fx_rates_rate_date ON fx_rates (currency, rate_date);

-- Rates a household set by hand: base currency units per one unit of
-- currency on the day.
CREATE TABLE fx_rate_overrides (
    household_id    TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    currency        TEXT NOT NULL CHECK (length(currency) = 3),
    date            TEXT NOT NULL,
    rate            TEXT NOT NULL,
    created_by_kind TEXT NOT NULL,
    created_by_id   TEXT NOT NULL,
    created_at      BIGINT NOT NULL,
    PRIMARY KEY (household_id, currency, date)
);

-- +goose Down
DROP TABLE fx_rate_overrides;
DROP TABLE fx_rates;
DROP TABLE transaction_splits;
DROP TABLE transactions;
DROP TABLE payees;
DROP TABLE wallet_members;
DROP TABLE wallets;
