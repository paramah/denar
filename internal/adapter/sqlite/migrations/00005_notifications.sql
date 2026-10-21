-- +goose Up
-- Stage 005-notifications: notifications of the app, the channels of
-- people, their preferences per household and event type, and the outbox
-- of deliveries to channels. Times are INTEGER microseconds. event_id has
-- no foreign key: events are kept and pruned on their own.

CREATE TABLE notifications (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id  TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    event_id      TEXT NOT NULL,
    type          TEXT NOT NULL,
    title         TEXT NOT NULL,
    body          TEXT NOT NULL DEFAULT '',
    link          TEXT NOT NULL DEFAULT '',
    wallet_id     TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    read_at       INTEGER,
    UNIQUE (event_id, user_id)
);
CREATE INDEX idx_notifications_user ON notifications (user_id, id DESC);
CREATE INDEX idx_notifications_unread ON notifications (user_id, household_id) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_read ON notifications (read_at) WHERE read_at IS NOT NULL;

CREATE TABLE notification_channels (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type          TEXT NOT NULL CHECK (type IN ('email', 'webhook', 'ntfy', 'telegram', 'matrix')),
    name          TEXT NOT NULL,
    config        TEXT NOT NULL DEFAULT '{}',
    secrets       BLOB,
    enabled       INTEGER NOT NULL DEFAULT 1,
    verified_at   INTEGER,
    last_error    TEXT NOT NULL DEFAULT '',
    last_error_at INTEGER,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_notification_channels_name ON notification_channels (user_id, name COLLATE NOCASE);

CREATE TABLE notification_preferences (
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    type         TEXT NOT NULL,
    in_app       INTEGER NOT NULL,
    channel_ids  TEXT NOT NULL DEFAULT '[]',
    position     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, household_id, type)
);

CREATE TABLE notification_deliveries (
    id              TEXT PRIMARY KEY,
    event_id        TEXT NOT NULL,
    user_id         TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id    TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    channel_id      TEXT NOT NULL REFERENCES notification_channels (id) ON DELETE CASCADE,
    type            TEXT NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    url             TEXT NOT NULL DEFAULT '',
    data            TEXT NOT NULL DEFAULT '{}',
    occurred_at     INTEGER NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    sent_at         INTEGER,
    UNIQUE (event_id, user_id, channel_id)
);
CREATE INDEX idx_notification_deliveries_due ON notification_deliveries (next_attempt_at, id) WHERE status = 'pending';
CREATE INDEX idx_notification_deliveries_channel ON notification_deliveries (channel_id);

CREATE INDEX idx_domain_events_dispatched ON domain_events (dispatched_at) WHERE dispatched_at IS NOT NULL;

-- +goose Down
DROP INDEX idx_domain_events_dispatched;
DROP TABLE notification_deliveries;
DROP TABLE notification_preferences;
DROP TABLE notification_channels;
DROP TABLE notifications;
