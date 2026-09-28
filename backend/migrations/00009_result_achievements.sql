-- +goose Up
ALTER TABLE results
    ADD COLUMN achievements jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE results
    DROP COLUMN achievements;
