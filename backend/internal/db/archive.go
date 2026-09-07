package db

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/Koded0214h/relic/backend/internal/meta"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

type ArchivedFile struct {
	ID				string
	ShootID			string
	Path			string
	OriginalSize	int64	
	Hash			string
	StoredSize		int64
	Recipe			types.Recipe
}

type FileStatus struct {
	FileID    		string
	Filename		string
	OriginalSize 	int64
	StoredSize		int64
	Codec			string
	Hash			string
	Archived		bool
}

func InsertArchiveFile(db *sql.DB, shootID, path string, originalSize, storedSize int64, hash string, recipe types.Recipe) (string, error) {
	id := uuid.NewString()
	params, err := json.Marshal(recipe.Params)
	if err != nil { return "", err }

	_, err = db.Exec(`
		INSERT INTO archived_files
			(id, shoot_id, path, original_size, hash, stored_size, codec, codec_version, codec_params, codec_blob)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, shootID, path, originalSize, hash, storedSize, recipe.Codec, recipe.Version, string(params), recipe.Blob,
	)
	return id, err
}

func GetArchivedFile(db *sql.DB, id string) (ArchivedFile, error) {
	var af ArchivedFile
	var params string
	var blob []byte

	err := db.QueryRow(`
		SELECT id, shoot_id, path, original_size, hash, stored_size, codec, codec_version, codec_params, codec_blob
		FROM archived_files WHERE id = ?`, id,
	).Scan(&af.ID, &af.ShootID, &af.Path, &af.OriginalSize, &af.Hash, &af.StoredSize,
		&af.Recipe.Codec, &af.Recipe.Version, &params, &blob)

	if err != nil { return ArchivedFile{}, err }

	if params != "" {
		if err := json.Unmarshal([]byte(params), &af.Recipe.Params); err != nil {
			return ArchivedFile{}, err
		}
	}

	af.Recipe.Blob = blob
	return af, nil
}

func UpdateFielMetadata(db *sql.DB, shootFileID string, m meta.Metadata) error {
	if !m.Valid { return nil }

	_, err := db.Exec(
		`UPDATE shoot_files SET taken_at = ?, camera_make = ?, camera_model = ?,
		focal_length =?, aperature = ?, shutter = ?, iso = ? WHERE id = ?`,
		m.TakenAt, m.CameraMake, m.CameraModel, m.Lens, m.FocalLength, m.Aperture, m.Shutter, m.ISO, shootFileID)
	return err
}


func ListFileStatus(db *sql.DB, shootID string) ([]FileStatus, error) {
	rows, err := db.Query(`
		SELECT sf.id, sf.filename, sf.size,
		       COALESCE(af.stored_size, 0),
		       COALESCE(af.codec, ''),
		       COALESCE(af.hash, '')
		FROM shoot_files sf
		LEFT JOIN archived_files af ON af.path = sf.staging_path AND af.shoot_id = sf.shoot_id
		WHERE sf.shoot_id = ?
		ORDER BY sf.created_at ASC`, shootID)
	
	if err != nil { return nil, err }
	defer rows.Close()

	var out []FileStatus
	for rows.Next() {
		var s FileStatus
		if err := rows.Scan(&s.FileID, &s.Filename, &s.OriginalSize, &s.StoredSize, &s.Codec, &s.Hash); err != nil {
			return nil, err
		}
		s.Archived = s.Hash != ""
		out = append(out, s)
	}

	return out, rows.Err()
}

func ListArchiveFiles(db *sql.DB, shootID string) ([]ArchivedFile, error) {
	rows, err := db.Query(`
		SELECT id, shoot_id, path, original_size, hash, stored_size, codec, codec_version, codec_params, codec_blob
		FROM archived_files WHERE shoot_id = ?`, shootID)	
	
	if err != nil { return nil, err }
	defer rows.Close()

	var out []ArchivedFile
	for rows.Next() {
		var af ArchivedFile
		var params string
		var blob []byte
		if err := rows.Scan(&af.ID, &af.ShootID, &af.Path, &af.OriginalSize, &af.Hash, &af.StoredSize,
			&af.Recipe.Codec, &af.Recipe.Version, &params, &blob); err != nil {
			return nil, err
		}
		if params != "" {
			json.Unmarshal([]byte(params), &af.Recipe.Params)
		}
		af.Recipe.Blob = blob
		out = append(out, af)
	}

	return out, rows.Err()
}