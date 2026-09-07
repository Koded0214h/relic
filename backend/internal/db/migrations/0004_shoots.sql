CREATE TABLE IF NOT EXISTS shoots (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    archived_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS shoot_files (
    id           TEXT PRIMARY KEY,
    shoot_id     TEXT NOT NULL REFERENCES shoots(id),
    filename     TEXT NOT NULL,       -- original name, display only
    size         INTEGER NOT NULL,
    staging_path TEXT NOT NULL,       -- where it actually lives on disk
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_shoots_user ON shoots(user_id);
CREATE INDEX IF NOT EXISTS idx_shoot_files_shoot ON shoot_files(shoot_id);
