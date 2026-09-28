-- +goose Up
ALTER TABLE results
    ADD COLUMN analysis jsonb NOT NULL DEFAULT '{"analyzedTurns":0,"techniques":[],"repeatedRisks":[],"priorityRecommendations":[]}'::jsonb;

-- +goose Down
ALTER TABLE results DROP COLUMN analysis;
