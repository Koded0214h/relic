package job_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/internal/job"
	"github.com/Koded0214h/relic/backend/internal/store"
)

func TestArchiveJobEndToEnd(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "objects"))
	if err != nil {
		t.Fatal(err)
	}
	reg := codec.NewRegistry(generic.New())
	rn := job.NewRunner(s, reg)

	f := filepath.Join(dir, "sample.txt")
	os.WriteFile(f, []byte("relic end to end archive test, repeated. relic end to end archive test, repeated."), 0o644)

	var results []job.Result
	j := rn.Start("job1", []string{f}, func(r job.Result) {
		results = append(results, r)
	})
	_ = j

	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := rn.Get("job1")
		if got.State == job.StateDone || got.State == job.StateError {
			if got.State == job.StateError {
				t.Fatalf("job failed: %s", got.Error)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for job")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !s.Has(r.Hash) {
		t.Fatal("archived object not found in store")
	}
	t.Logf("archived %s: %d -> %d bytes (codec: %s)", r.Path, r.Size, r.StoredSize, r.Recipe.Codec)
}