-- +goose Up
CREATE TABLE scenarios (
    id text PRIMARY KEY,
    title text NOT NULL,
    sphere text NOT NULL,
    topic text NOT NULL,
    difficulty text NOT NULL,
    opponent_role text NOT NULL,
    opponent_tone text NOT NULL,
    player_goal text NOT NULL,
    opponent_goal text NOT NULL,
    initial_message text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE negotiation_sessions (
    id text PRIMARY KEY,
    scenario_id text NOT NULL REFERENCES scenarios(id),
    status text NOT NULL,
    turn integer NOT NULL DEFAULT 0,
    trust_score integer NOT NULL DEFAULT 50,
    argument_score integer NOT NULL DEFAULT 0,
    pressure_score integer NOT NULL DEFAULT 0,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE TABLE messages (
    id bigserial PRIMARY KEY,
    session_id text NOT NULL REFERENCES negotiation_sessions(id) ON DELETE CASCADE,
    sender text NOT NULL,
    content text NOT NULL,
    analysis jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE results (
    session_id text PRIMARY KEY REFERENCES negotiation_sessions(id) ON DELETE CASCADE,
    final_score integer NOT NULL,
    outcome text NOT NULL,
    strengths jsonb NOT NULL DEFAULT '[]',
    mistakes jsonb NOT NULL DEFAULT '[]',
    recommendations jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS results;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS negotiation_sessions;
DROP TABLE IF EXISTS scenarios;

