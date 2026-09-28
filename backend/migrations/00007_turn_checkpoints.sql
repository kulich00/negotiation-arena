-- +goose Up
CREATE TABLE session_checkpoints (
    session_id text NOT NULL REFERENCES negotiation_sessions(id) ON DELETE CASCADE,
    turn integer NOT NULL CHECK (turn >= 0),
    trust_score integer NOT NULL CHECK (trust_score BETWEEN 0 AND 100),
    argument_score integer NOT NULL CHECK (argument_score BETWEEN 0 AND 100),
    pressure_score integer NOT NULL CHECK (pressure_score BETWEEN 0 AND 100),
    state jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, turn)
);

INSERT INTO session_checkpoints (session_id,turn,trust_score,argument_score,pressure_score,state,created_at)
SELECT id,0,50,0,0,'{"phase":"opening"}'::jsonb,started_at
FROM negotiation_sessions;

INSERT INTO session_checkpoints (session_id,turn,trust_score,argument_score,pressure_score,state,created_at)
SELECT id,turn,trust_score,argument_score,pressure_score,state,COALESCE(finished_at,now())
FROM negotiation_sessions
WHERE turn > 0
ON CONFLICT (session_id,turn) DO NOTHING;

-- +goose Down
DROP TABLE session_checkpoints;
