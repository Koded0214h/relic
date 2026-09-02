package job

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/store"
	"github.com/Koded0214h/relic/backend/pkg/types"
)


type State string

const (
	StateRunning State = "running"
	StateDone    State = "done"
	StateError   State = "error"
)

type Job struct {
	ID    string `json:"id"`
	State State  `json:"state"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Error string `json:"error,omitempty"`

	mu sync.Mutex
}

func (j *Job) snapshot() Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return Job{
		ID: j.ID,
		State: j.State,
		Done: j.Done,
		Total: j.Total,
		Error: j.Error,
	}
}

type Result struct {
	Path       string
	Hash       string
	Size       int64
	StoredSize int64
	Recipe     types.Recipe
}

type Runner struct {
	store    *store.Store
	registry codec.Registry

	mu   sync.Mutex
	jobs map[string]*Job
}

func NewRunner(s *store.Store, r codec.Registry) *Runner {
	return &Runner{store: s, registry: r, jobs: map[string]*Job{}}
}

func (rn *Runner) Start(jobID string, paths []string, onResult func(Result)) *Job {
	j := &Job{ID: jobID, State: StateRunning, Total: len(paths)}

	rn.mu.Lock()
	rn.jobs[jobID] = j
	rn.mu.Unlock()

	go rn.run(j, paths, onResult)
	return j
}

func (rn *Runner) Get(jobID string) (Job, bool) {
	rn.mu.Lock()
	j, ok := rn.jobs[jobID]
	rn.mu.Unlock()
	if !ok {
		return Job{}, false
	}
	return j.snapshot(), true
}

func (rn *Runner) run(j *Job, paths []string, onResult func(Result)) {
	for _, p := range paths {
		res, err := rn.archiveOne(p)
		if err != nil {
			j.mu.Lock()
			j.State = StateError
			j.Error = fmt.Sprintf("%s: %v", p, err)
			j.mu.Unlock()
			return
		}
		if onResult != nil {
			onResult(res)
		}
		j.mu.Lock()
		j.Done++
		j.mu.Unlock()

		// tiny yield so SSE polling sees intermediate states even on
		// very fast/small local test runs
		time.Sleep(5 * time.Millisecond)
	}

	j.mu.Lock()
	j.State = StateDone
	j.mu.Unlock()

}


func (rn *Runner) archiveOne(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Result{}, fmt.Errorf("stat: %w", err)
	}

	head := make([]byte, 64*1024)
	n, _ := f.Read(head)
	head = head[:n]
	if _, err := f.Seek(0, 0); err != nil {
		return Result{}, fmt.Errorf("seek: %w", err)
	}

	tf := types.File{
		Path: path,
		Size: info.Size(),
		Ext:  ext(path),
		Head: head,
	}

	// Encode into a temp buffer file so we can hash the *encoded*
	// bytes while streaming them into the store in one pass.
	tmp, err := os.CreateTemp("", "relic-encode-*")
	if err != nil {
		return Result{}, fmt.Errorf("temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	recipe, err := rn.registry.EncodeVerified(tf, f, tmp)
	if err != nil {
		return Result{}, fmt.Errorf("encode: %w", err)
	}

	if _, err := tmp.Seek(0, 0); err != nil {
		return Result{}, fmt.Errorf("seek encoded: %w", err)
	}
	hash, storedSize, err := rn.store.Put(tmp)
	if err != nil {
		return Result{}, fmt.Errorf("store: %w", err)
	}

	return Result{
		Path:       path,
		Hash:       hash,
		Size:       info.Size(),
		StoredSize: storedSize,
		Recipe:     recipe,
	}, nil
}

func ext(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

