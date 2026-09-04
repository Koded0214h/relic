package raw_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/internal/codec/jpg"
	"github.com/Koded0214h/relic/backend/internal/codec/raw"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

// TestRAWPreviewRoundTrip needs a real camera RAW dropped in as
// internal/testdata/sample.arw (or .cr2/.nef/... — adjust the name).
func TestRAWPreviewRoundTrip(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.arw")
	raww, err := os.ReadFile(path)
	if err != nil {
		t.Skip("no sample RAW in testdata, skipping")
	}

	reg := codec.NewRegistry(generic.New(), jpg.New(), raw.New())
	f := types.File{Path: path, Size: int64(len(raww)), Ext: filepath.Ext(path), Head: raww[:min(len(raww), 64*1024)]}

	var encoded bytes.Buffer
	rec, err := reg.EncodeVerified(f, bytes.NewReader(raww), &encoded)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded bytes.Buffer
	if err := reg.Decode(rec, bytes.NewReader(encoded.Bytes()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(raww, decoded.Bytes()) {
		t.Fatal("round-trip mismatch")
	}

	ratio := 100 * float64(encoded.Len()) / float64(len(raww))
	t.Logf("codec: %s | %d -> %d bytes (%.1f%%)", rec.Codec, len(raww), encoded.Len(), ratio)
}

// TestRAWSyntheticRoundTrip fabricates a RAW-shaped container (opaque
// header + a real embedded JPEG + trailer) so the codec path has real
// coverage without needing a camera file checked in.
func TestRAWSyntheticRoundTrip(t *testing.T) {
	jpegBytes, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.jpg"))
	if err != nil {
		t.Skip("no sample.jpg fixture")
	}

	prefix := bytes.Repeat([]byte{0x2A, 0x00, 0x11}, 400) // no 0xFF -> no false SOI
	suffix := bytes.Repeat([]byte{0x7F, 0x10}, 300)
	container := bytes.Join([][]byte{prefix, jpegBytes, suffix}, nil)

	reg := codec.NewRegistry(generic.New(), jpg.New(), raw.New())
	f := types.File{Path: "synthetic.dng", Size: int64(len(container)), Ext: ".dng", Head: container[:min(len(container), 64*1024)]}

	var encoded bytes.Buffer
	rec, err := reg.EncodeVerified(f, bytes.NewReader(container), &encoded)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded bytes.Buffer
	if err := reg.Decode(rec, bytes.NewReader(encoded.Bytes()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(container, decoded.Bytes()) {
		t.Fatal("round-trip mismatch")
	}

	// The preview transcode only runs when cjxl is present; otherwise the
	// registry correctly falls back to generic for the whole container.
	if _, lookErr := exec.LookPath("cjxl"); lookErr == nil && rec.Codec != raw.Name {
		t.Fatalf("codec = %q, want %q", rec.Codec, raw.Name)
	}

	ratio := 100 * float64(encoded.Len()) / float64(len(container))
	t.Logf("codec: %s | %d -> %d bytes (%.1f%%)", rec.Codec, len(container), encoded.Len(), ratio)
}
