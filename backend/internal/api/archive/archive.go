package archive

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/db"
	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/job"
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
	shootID := chi.URLParam(r, "shootID")
	jobID := "job_" + shootID

	// TODO: once Ridwan's upload endpoint exists, look up this shoot's
	// real uploaded file paths from the DB instead of the test corpus.
	paths, err := testCorpusPaths()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "corpus_error", err.Error())
		return
	}

	runner.Start(jobID, paths, func(res job.Result) {
		if _, err := db.InsertArchiveFile(sqlDB, shootID, res.Path, res.Size, res.StoredSize, res.Hash, res.Recipe); err != nil {
			// TODO: surface this on the job's error state once job.Job
			// supports per-file errors, not just fatal ones.
			fmt.Printf("index write failed for %s: %v\n", res.Path, err)
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

func testCorpusPaths() ([]string, error) {
	matches, err := filepath.Glob("internal/testdata/*")
	if err != nil {
		return nil, err
	}
	return matches, nil
}
