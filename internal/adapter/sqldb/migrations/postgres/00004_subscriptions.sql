-- +goose Up
-- Stage 004-subscriptions (PostgreSQL): subscriptions with their price history, the
-- charges expected from them and the outbox of domain events. Amounts are
-- BIGINT minor units of the currency next to them; dates are TEXT
-- 'YYYY-MM-DD'; times BIGINT microseconds. Foreign keys to wallets,
-- envelopes and transactions have no action: checked at the end of the
-- statement, so deleting a household removes everything at once, while the
-- use cases refuse or unlink first otherwise.

CREATE TABLE subscriptions (
    id                   TEXT PRIMARY KEY,
    household_id         TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name                 TEXT NOT NULL,
    vendor               TEXT NOT NULL,
    external_ref         TEXT,
    cycle_unit           TEXT NOT NULL CHECK (cycle_unit IN ('day', 'week', 'month', 'year')),
    cycle_every          BIGINT NOT NULL CHECK (cycle_every BETWEEN 1 AND 365),
    cycle_anchor         TEXT NOT NULL,
    trial_ends_at        TEXT,
    status               TEXT NOT NULL CHECK (status IN ('trial', 'active', 'paused', 'cancelled')),
    cancelled_at         BIGINT,
    ends_at              TEXT,
    wallet_id            TEXT REFERENCES wallets (id),
    envelope_id          TEXT REFERENCES envelopes (id),
    reminder_offsets     TEXT NOT NULL DEFAULT '[]',
    payee_pattern        TEXT NOT NULL DEFAULT '',
    amount_tolerance_pct BIGINT NOT NULL,
    date_window_days     BIGINT NOT NULL,
    note                 TEXT NOT NULL DEFAULT '',
    manage_url           TEXT,
    created_by_kind      TEXT NOT NULL,
    created_by_id        TEXT NOT NULL,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL
);
CREATE UNIQUE INDEX idx_subscriptions_name ON subscriptions (household_id, lower(name));
CREATE INDEX idx_subscriptions_envelope ON subscriptions (envelope_id);
CREATE INDEX idx_subscriptions_wallet ON subscriptions (wallet_id);

CREATE TABLE subscription_prices (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    effective_from  TEXT NOT NULL,
    amount          BIGINT NOT NULL CHECK (amount > 0),
    currency        TEXT NOT NULL,
    created_at      BIGINT NOT NULL,
    UNIQUE (subscription_id, effective_from)
);

CREATE TABLE subscription_charges (
    id              TEXT PRIMARY KEY,
    household_id    TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    subscription_id TEXT NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    due_date        TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('expected', 'matched', 'missed', 'skipped', 'posted')),
    transaction_id  TEXT REFERENCES transactions (id),
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL,
    CHECK ((status IN ('matched', 'posted')) = (transaction_id IS NOT NULL)),
    UNIQUE (subscription_id, due_date)
);
CREATE UNIQUE INDEX idx_subscription_charges_transaction ON subscription_charges (transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX idx_subscription_charges_household ON subscription_charges (household_id, due_date);
CREATE INDEX idx_subscription_charges_status ON subscription_charges (status, due_date);

CREATE TABLE domain_events (
    id            TEXT PRIMARY KEY,
    household_id  TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    type          TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    dedupe_key    TEXT NOT NULL,
    payload       TEXT NOT NULL DEFAULT '{}',
    occurred_at   BIGINT NOT NULL,
    dispatched_at BIGINT,
    UNIQUE (household_id, dedupe_key)
);
CREATE INDEX idx_domain_events_pending ON domain_events (id) WHERE dispatched_at IS NULL;

-- +goose Down
DROP TABLE domain_events;
DROP TABLE subscription_charges;
DROP TABLE subscription_prices;
DROP TABLE subscriptions;
