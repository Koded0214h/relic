package archive

import  (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/httpx"
)

type jobState struct {
	ID		string 	`json:"id"`
	State 	string 	`json:"state"`
	Done	int		`json:"done"`
	Total	int		`json:"total"`
	Error 	string	`json:"error,omitempty"`
}

var (
	mu	sync.Mutex
	jobs = map[string]*jobState{}
)

func Mount(r chi.Router) {
	r.Post("/shoots/{shootID}/archive", startArchive)
	r.Get("/jobs/{jobID}", getJob)
	r.Get("/jobs/{jobID}/events", streamJob)
}

func startArchive(w http.ResponseWriter, r *http.Request) {
	shootID := chi.URLParam(r, "shootID")
	id := "job_" + shootID

	mu.Lock()
	j := &jobState{ID: id, State: "running", Done: 0, Total: 42}
	jobs[id] = j
	mu.Unlock()

	go fakeProgress(j)

	httpx.JSON(w, http.StatusAccepted, map[string]string{"job_id":id})
}

func fakeProgress(j *jobState) {
	for j.Done < j.Total {
		time.Sleep(150 * time.Millisecond)
		mu.Lock()
		j.Done++
		if j.Done == j.Total {
			j.State = "done"
		}
		mu.Unlock()
	}
}


func getJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobID")
	mu.Lock()
	j, ok := jobs[id]
	mu.Unlock()
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
		case <- r.Context().Done():
			return
		default:
		}
		
		mu.Lock()
		j, ok := jobs[id]
		mu.Unlock()
		if !ok { return }

		b, _ := json.Marshal(j)
		w.Write([]byte("data: "))
		w.Write(b)
		w.Write([]byte("\n\n"))
		flusher.Flush()

		if j.State == "done" || j.State == "error" { return }

		time.Sleep(200 * time.Millisecond)
	}
}