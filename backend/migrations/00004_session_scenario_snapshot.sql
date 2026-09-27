-- +goose Up
ALTER TABLE negotiation_sessions ADD COLUMN initial_message text;

UPDATE negotiation_sessions AS session
SET initial_message = scenario.initial_message
FROM scenarios AS scenario
WHERE scenario.id = session.scenario_id;

ALTER TABLE negotiation_sessions ALTER COLUMN initial_message SET NOT NULL;

-- +goose Down
ALTER TABLE negotiation_sessions DROP COLUMN initial_message;
