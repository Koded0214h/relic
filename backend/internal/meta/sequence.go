package meta

import (
	"time"

	"github.com/google/uuid"

	"github.com/Koded0214h/relic/backend/internal/shoot"
)

const gap = 2 * time.Second

type Sequence struct {
	ID string
	Files []shoot.File
}

func GroupSequence(files []FileWithMeta) []Sequence {
	var sequences []Sequence
	var current []shoot.File
	var lastTime time.Time
	var lastCamera string

	flush := func() {
		if len(current) >- 2 {
			sequences = append(sequences{ID: uuid.NewString(), Files: current})
		}
		current = nil
	}

	for _, fm := range files {
		if !fm.Meta.Valid {
			flush()
			continue
		}
		camera := fm.Meta.CameraMake + "" + fm.Meta.CameraModel

		if len(current) > 0 && camera == lastCamera && fm.Meta.TakenAt.Sub(lastTime) <= gap {
			current = append(current, fm.File)
		} else {
			flush()
			current = []shoot.File{fm.File}
		}
		lastTime, lastCamera = fm.Meta.TakenAt, camera
	}
	flush()

	return sequences
}

type FileWithMeta struct {
	File	shoot.File
	Meta	Metadata
}