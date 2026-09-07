package shoot

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("shoot: not found")
	ErrQuotaExceeded = errors.New("shoot: upload would exceed quota")
)

const (
	MaxShootBytes = 2 << 30 // 2 GiB
	MaxShootFiles = 200
)

type Shoot struct {
	ID         string
	UserID     string
	Name       string
	ArchivedAt sql.NullTime
}

type File struct {
	ID          string `json:"id"`
	ShootID     string `json:"shoot_id"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	StagingPath string `json:"-"` // internal disk path, not exposed
}

type Summary struct {
	Shoot
	FileCount int
	TotalSize int64
}

func Create(db *sql.DB, userID, name string) (Shoot, error) {
	id := uuid.NewString()
	if _, err := db.Exec(`INSERT INTO shoots (id, user_id, name) VALUES (?, ?, ?)`, id, userID, name); err != nil {
		return Shoot{}, err
	}
	return Shoot{ID: id, UserID: userID, Name: name}, nil
}

// Get loads a shoot only if userID owns it; a miss (wrong id or wrong
// owner) is ErrNotFound so callers can't distinguish the two.
func Get(db *sql.DB, shootID, userID string) (Shoot, error) {
	var s Shoot
	err := db.QueryRow(
		`SELECT id, user_id, name, archived_at FROM shoots WHERE id = ? AND user_id = ?`,
		shootID, userID,
	).Scan(&s.ID, &s.UserID, &s.Name, &s.ArchivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Shoot{}, ErrNotFound
	}
	if err != nil {
		return Shoot{}, err
	}
	return s, nil
}

func ListForUser(db *sql.DB, userID string) ([]Summary, error) {
	rows, err := db.Query(`
		SELECT s.id, s.name, s.archived_at,
		       COUNT(f.id), COALESCE(SUM(f.size), 0)
		FROM shoots s
		LEFT JOIN shoot_files f ON f.shoot_id = s.id
		WHERE s.user_id = ?
		GROUP BY s.id
		ORDER BY s.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Summary
	for rows.Next() {
		var s Summary
		s.UserID = userID
		if err := rows.Scan(&s.ID, &s.Name, &s.ArchivedAt, &s.FileCount, &s.TotalSize); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func Delete(db *sql.DB, shootID, userID string) error {
	if _, err := Get(db, shootID, userID); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM shoot_files WHERE shoot_id = ?`, shootID); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM shoots WHERE id = ?`, shootID)
	return err
}

// Stats returns the current file count and total byte size for a shoot,
// used to enforce the per-shoot upload caps.
func Stats(db *sql.DB, shootID string) (count int, totalBytes int64, err error) {
	err = db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(size), 0) FROM shoot_files WHERE shoot_id = ?`,
		shootID,
	).Scan(&count, &totalBytes)
	return
}

func AddFile(db *sql.DB, shootID, filename string, size int64, stagingPath string) (File, error) {
	id := uuid.NewString()
	if _, err := db.Exec(
		`INSERT INTO shoot_files (id, shoot_id, filename, size, staging_path) VALUES (?, ?, ?, ?, ?)`,
		id, shootID, filename, size, stagingPath,
	); err != nil {
		return File{}, err
	}
	return File{ID: id, ShootID: shootID, Filename: filename, Size: size, StagingPath: stagingPath}, nil
}

func ListFiles(db *sql.DB, shootID string) ([]File, error) {
	rows, err := db.Query(
		`SELECT id, shoot_id, filename, size, staging_path FROM shoot_files WHERE shoot_id = ? ORDER BY created_at`,
		shootID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.ShootID, &f.Filename, &f.Size, &f.StagingPath); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
