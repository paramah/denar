-- +goose Up
-- Stage 003-envelope-budget: envelope groups, envelopes with their goals
-- and monthly assignments, all in the household's base currency (INTEGER
-- minor units). Months are TEXT 'YYYY-MM'. Transactions and split lines
-- keep envelope_id without a foreign key (adding one needs a table
-- rebuild); the use cases check it.

CREATE TABLE envelope_groups (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_envelope_groups_name ON envelope_groups (household_id, name COLLATE NOCASE);

CREATE TABLE envelopes (
    id           TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    -- NO ACTION, checked at the end of the statement: deleting a household
    -- removes its groups and envelopes together.
    group_id     TEXT NOT NULL REFERENCES envelope_groups (id),
    name         TEXT NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    hidden       INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    goal_type    TEXT CHECK (goal_type IN ('monthly', 'target_by_date')),
    goal_amount  INTEGER,
    goal_date    TEXT,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    CHECK ((goal_type IS NULL) = (goal_amount IS NULL)),
    CHECK (CASE WHEN goal_type = 'target_by_date' THEN goal_date IS NOT NULL ELSE goal_date IS NULL END)
);
CREATE UNIQUE INDEX idx_envelopes_name ON envelopes (household_id, name COLLATE NOCASE);
CREATE INDEX idx_envelopes_group ON envelopes (group_id);

CREATE TABLE envelope_assignments (
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    envelope_id  TEXT NOT NULL REFERENCES envelopes (id) ON DELETE CASCADE,
    month        TEXT NOT NULL CHECK (length(month) = 7),
    amount       INTEGER NOT NULL CHECK (amount <> 0),
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (envelope_id, month)
);
CREATE INDEX idx_envelope_assignments_household ON envelope_assignments (household_id, month);

-- +goose Down
DROP TABLE envelope_assignments;
DROP TABLE envelopes;
DROP TABLE envelope_groups;
