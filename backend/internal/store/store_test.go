// internal/store/store_test.go
package store_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Koded0214h/relic/backend/internal/store"
)

func TestPutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("relic object store test payload")

	hash, size, err := s.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(data)) {
		t.Fatalf("size = %d, want %d", size, len(data))
	}

	r, err := s.Get(hash)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("round-trip mismatch")
	}
}

func TestDedup(t *testing.T) {
	dir := t.TempDir()
	s, _ := store.New(dir)

	data := []byte("same bytes twice")
	h1, _, err := s.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	h2, _, err := s.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Fatalf("expected same hash, got %s and %s", h1, h2)
	}

	// Exactly one object file should exist on disk (the hash file).
	var fileCount int
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fileCount != 1 {
		t.Fatalf("expected 1 object file, got %d", fileCount)
	}
}