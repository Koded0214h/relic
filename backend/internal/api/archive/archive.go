package archive

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/job"
)

type Handler struct {
	runner *job.Runner
}

func Mount(r chi.Router, runner *job.Runner) {
	h := &Handler{runner: runner}
	r.Post("/shoots/{shootID}/archive", h.startArchive)
	r.Get("/jobs/{jobID}", h.getJob)
	r.Get("/jobs/{jobID}/events", h.streamJob)
}

func (h *Handler) startArchive(w http.ResponseWriter, r *http.Request) {
	shootID := chi.URLParam(r, "shootID")
	jobID := "job_" + shootID

	// TODO: once Ridwan's upload endpoint exists, look up this shoot's
	// real uploaded file paths from the DB instead of the test corpus.
	paths, err := testCorpusPaths()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "corpus_error", err.Error())
		return
	}

	h.runner.Start(jobID, paths, func(res job.Result) {
		// TODO: once the index exists, persist res.Hash / res.Recipe
		// keyed by file ID here. For now just visible via logs.
	})

	httpx.JSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobID")
	j, ok := h.runner.Get(id)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	httpx.JSON(w, http.StatusOK, j)
}

func (h *Handler) streamJob(w http.ResponseWriter, r *http.Request) {
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

		j, ok := h.runner.Get(id)
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