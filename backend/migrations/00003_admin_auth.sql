-- +goose Up
CREATE TABLE admins (
    email text PRIMARY KEY,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    token_hash text PRIMARY KEY,
    admin_email text NOT NULL REFERENCES admins(email) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX admin_sessions_expires_at_idx ON admin_sessions (expires_at);

-- +goose Down
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admins;
