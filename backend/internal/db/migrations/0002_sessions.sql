CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,      -- the cookie value
    user_id    TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
