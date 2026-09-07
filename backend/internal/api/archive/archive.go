package archive

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/auth"
	"github.com/Koded0214h/relic/backend/internal/db"
	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/job"
	"github.com/Koded0214h/relic/backend/internal/shoot"
)

var (
	runner *job.Runner
	sqlDB  *sql.DB
)

// Init wires the shared dependencies. Called once from main before Mount.
func Init(rn *job.Runner, database *sql.DB) {
	runner = rn
	sqlDB = database
}

func Mount(r chi.Router) {
	r.Post("/shoots/{shootID}/archive", startArchive)
	r.Get("/jobs/{jobID}", getJob)
	r.Get("/jobs/{jobID}/events", streamJob)
}

func startArchive(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)
	shootID := chi.URLParam(r, "shootID")

	s, err := shoot.Get(sqlDB, shootID, userID)
	if errors.Is(err, shoot.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "shoot not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not load shoot")
		return
	}

	files, err := shoot.ListFiles(sqlDB, s.ID)
	if err != nil || len(files) == 0 {
		httpx.Error(w, http.StatusBadRequest, "no_files", "shoot has no uploaded files")
		return
	}

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.StagingPath
	}

	jobID := "job_" + s.ID

	pathToID := map[string]string{}
	for _, f := range files {
		pathToID[f.StagingPath] = f.ID
	}

	runner.Start(jobID, paths, func(res job.Result) {
		if _, err := db.InsertArchiveFile(sqlDB, s.ID, res.Path, res.Size, res.StoredSize, res.Hash, res.Recipe); err != nil {
			fmt.Printf("index write failed for %s: %v\n", res.Path, err)
		}
		if fileID, ok := pathToID[res.Path]; ok {
			if err := db.UpdateFielMetadata(sqlDB, fileID, res.Meta); err != nil {
				fmt.Printf("metadata write failed for %s: %v\n", res.Path, err)
			}
		}
	})

	httpx.JSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func getJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobID")
	j, ok := runner.Get(id)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	httpx.JSON(w, http.StatusOK, j)
}

func streamJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobID")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, "no_stream", "streaming unsupported")
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		j, ok := runner.Get(id)
		if !ok {
			return
		}

		b, _ := json.Marshal(j)
		w.Write([]byte("data: "))
		w.Write(b)
		w.Write([]byte("\n\n"))
		flusher.Flush()

		if j.State == job.StateDone || j.State == job.StateError {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
