package shoot

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("shoot: nto found")
type Shoot struct {
	ID         string
	UserID     string
	Name       string
	ArchivedAt sql.NullTime
}

type File struct {
	ID          string
	ShootID     string
	Filename    string
	Size        int64
	StagingPath string
}

func Create(db *sql.DB, userID, name string) (Shoot, error) {
	id := uuid.NewString()
	_, err := db.Exec()
}