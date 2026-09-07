ALTER TABLE shoot_files ADD COLUMN taken_at     TIMESTAMP;
ALTER TABLE shoot_files ADD COLUMN camera_make  TEXT;
ALTER TABLE shoot_files ADD COLUMN camera_model TEXT;
ALTER TABLE shoot_files ADD COLUMN lens         TEXT;
ALTER TABLE shoot_files ADD COLUMN focal_length TEXT;
ALTER TABLE shoot_files ADD COLUMN aperture     TEXT;
ALTER TABLE shoot_files ADD COLUMN shutter      TEXT;
ALTER TABLE shoot_files ADD COLUMN iso          INTEGER;
ALTER TABLE shoot_files ADD COLUMN sequence_id  TEXT;

CREATE INDEX IF NOT EXISTS idx_shoot_files_taken_at ON shoot_files(shoot_id, taken_at);