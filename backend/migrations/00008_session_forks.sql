-- +goose Up
ALTER TABLE negotiation_sessions
    ADD COLUMN parent_session_id text REFERENCES negotiation_sessions(id),
    ADD COLUMN forked_from_turn integer CHECK (forked_from_turn >= 0),
    ADD CONSTRAINT negotiation_sessions_fork_pair CHECK (
        (parent_session_id IS NULL AND forked_from_turn IS NULL) OR
        (parent_session_id IS NOT NULL AND forked_from_turn IS NOT NULL)
    );

CREATE INDEX negotiation_sessions_parent_idx
    ON negotiation_sessions(parent_session_id)
    WHERE parent_session_id IS NOT NULL;

-- +goose Down
DROP INDEX negotiation_sessions_parent_idx;
ALTER TABLE negotiation_sessions
    DROP CONSTRAINT negotiation_sessions_fork_pair,
    DROP COLUMN forked_from_turn,
    DROP COLUMN parent_session_id;
