package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("store: create root: %w", err)
	}

	return &Store{root: root}, nil
}

func (s *Store) Put(src io.Reader) (hash string, size int64, err error) {
	tmp, err := os.CreateTemp(s.root, "tmp-*")
	if err != nil {
		return "", 0, fmt.Errorf("store: temp file: %w", err)
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), src)
	if err != nil {
		tmp.Close()
		return "", 0, fmt.Errorf("store: write: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("store: close temp: %w", err)
	}

	sum := hex.EncodeToString(h.Sum(nil))
	dst := s.path(sum)

	if _, err := os.Stat(dst); err == nil { return sum, n, nil }

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "",0,fmt.Errorf("stor: mkdir: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return "", 0, fmt.Errorf("store: commit: %w", err)
	}
	return sum, n, nil
}

func (s *Store) Get(hash string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(hash))
	if err != nil {
		return nil, fmt.Errorf("store: get %s: %w", hash, err)
	}
	return f, nil
}

func (s *Store) Has(hash string) bool {
	_, err := os.Stat(s.path(hash))
	return err == nil
}

func (s *Store) path(hash string) string {
	if len(hash) < 4 {
		return filepath.Join(s.root, hash) // shouldn't happen with sha256, but don't panic
	}
	return filepath.Join(s.root, hash[:2], hash[2:4], hash)
}


