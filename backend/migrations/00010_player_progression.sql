-- +goose Up
CREATE TABLE player_profiles (
    id text PRIMARY KEY,
    display_name text NOT NULL,
    completed_sessions integer NOT NULL DEFAULT 0 CHECK (completed_sessions >= 0),
    successful_sessions integer NOT NULL DEFAULT 0 CHECK (successful_sessions >= 0),
    current_win_streak integer NOT NULL DEFAULT 0 CHECK (current_win_streak >= 0),
    best_win_streak integer NOT NULL DEFAULT 0 CHECK (best_win_streak >= 0),
    unlocked_difficulty text NOT NULL DEFAULT 'easy'
        CHECK (unlocked_difficulty IN ('easy', 'medium', 'hard')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE player_achievements (
    player_id text NOT NULL REFERENCES player_profiles(id) ON DELETE CASCADE,
    code text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    unlocked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, code)
);

ALTER TABLE negotiation_sessions
    ADD COLUMN player_id text REFERENCES player_profiles(id);

CREATE INDEX negotiation_sessions_player_idx
    ON negotiation_sessions(player_id)
    WHERE player_id IS NOT NULL;

-- +goose Down
DROP INDEX negotiation_sessions_player_idx;

ALTER TABLE negotiation_sessions
    DROP COLUMN player_id;

DROP TABLE player_achievements;
DROP TABLE player_profiles;
