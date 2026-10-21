-- +goose Up
-- Stage 013-i18n (PostgreSQL): the language a person chose. NULL means automatic (the
-- browser decides in the Web UI, Polish on the server).

ALTER TABLE users ADD COLUMN locale TEXT NULL CHECK (locale IN ('pl', 'en'));

-- +goose Down
ALTER TABLE users DROP COLUMN locale;
