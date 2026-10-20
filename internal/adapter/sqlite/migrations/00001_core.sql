-- +goose Up
-- Stage 001-core: accounts, sessions, households, memberships, invitations,
-- bots and the audit log. IDs are TEXT (ULID); times are INTEGER
-- microseconds since the Unix epoch (UTC). Only hashes of session,
-- invitation and bot tokens are stored.

CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    instance_admin INTEGER NOT NULL DEFAULT 0 CHECK (instance_admin IN (0, 1)),
    oidc_issuer    TEXT,
    oidc_subject   TEXT,
    password_hash  TEXT,
    active         INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at     INTEGER NOT NULL,
    CHECK ((oidc_issuer IS NULL) = (oidc_subject IS NULL))
);
CREATE UNIQUE INDEX idx_users_oidc ON users (oidc_issuer, oidc_subject) WHERE oidc_subject IS NOT NULL;

CREATE TABLE sessions (
    token_hash   TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user    ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

CREATE TABLE oidc_login_states (
    state_hash    TEXT PRIMARY KEY,
    nonce         TEXT NOT NULL,
    code_verifier TEXT NOT NULL,
    return_to     TEXT NOT NULL,
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL
);
CREATE INDEX idx_oidc_login_states_expires ON oidc_login_states (expires_at);

CREATE TABLE households (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    base_currency   TEXT NOT NULL CHECK (length(base_currency) = 3),
    timezone        TEXT NOT NULL,
    month_start_day INTEGER NOT NULL CHECK (month_start_day BETWEEN 1 AND 28),
    created_by      TEXT NOT NULL REFERENCES users (id),
    created_at      INTEGER NOT NULL
);

CREATE TABLE memberships (
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users (id),
    role         TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at   INTEGER NOT NULL,
    PRIMARY KEY (household_id, user_id)
);
CREATE INDEX idx_memberships_user ON memberships (user_id);

CREATE TABLE invitations (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    email        TEXT NOT NULL,
    role         TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    token_hash   TEXT NOT NULL UNIQUE,
    created_by   TEXT NOT NULL REFERENCES users (id),
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    accepted_at  INTEGER,
    accepted_by  TEXT REFERENCES users (id),
    revoked_at   INTEGER
);
CREATE INDEX idx_invitations_household ON invitations (household_id, created_at);
CREATE INDEX idx_invitations_email     ON invitations (email);

CREATE TABLE bots (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    -- JSON array of scopes, for example ["wallets:read"].
    scopes       TEXT NOT NULL,
    created_by   TEXT NOT NULL REFERENCES users (id),
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER,
    disabled     INTEGER NOT NULL DEFAULT 0 CHECK (disabled IN (0, 1)),
    token_hash   TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    last_used_at INTEGER
);
CREATE UNIQUE INDEX idx_bots_name ON bots (household_id, name COLLATE NOCASE);
CREATE INDEX idx_bots_creator ON bots (household_id, created_by);

-- The wallets a bot is limited to; none means every wallet its creator
-- sees. Stage 002 checks the ids against its wallets in the use case.
CREATE TABLE bot_wallets (
    bot_id    TEXT NOT NULL REFERENCES bots (id) ON DELETE CASCADE,
    wallet_id TEXT NOT NULL,
    position  INTEGER NOT NULL,
    PRIMARY KEY (bot_id, wallet_id)
);

CREATE TABLE audit_log (
    id            TEXT PRIMARY KEY,
    household_id  TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    at            INTEGER NOT NULL,
    actor_kind    TEXT NOT NULL CHECK (actor_kind IN ('user', 'bot', 'system')),
    actor_id      TEXT NOT NULL,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    -- JSON object: field -> {"from": …, "to": …}
    changes       TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_household ON audit_log (household_id, id);

-- The audit log is append-only: entries change only by going with their
-- household.
-- +goose StatementBegin
CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER audit_log_no_update;
DROP TABLE audit_log;
DROP TABLE bot_wallets;
DROP TABLE bots;
DROP TABLE invitations;
DROP TABLE memberships;
DROP TABLE households;
DROP TABLE oidc_login_states;
DROP TABLE sessions;
DROP TABLE users;
