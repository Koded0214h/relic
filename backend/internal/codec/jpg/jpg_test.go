package jpg_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/internal/codec/jpg"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

func TestJPEGRoundTripAndRatio(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.jpg")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("no sample.jpg in testdata, skipping")
	}

	reg := codec.NewRegistry(generic.New(), jpg.New())
	f := types.File{Path: path, Size: int64(len(raw)), Ext: ".jpg", Head: raw[:min(len(raw), 64*1024)]}

	var encoded bytes.Buffer
	rec, err := reg.EncodeVerified(f, bytes.NewReader(raw), &encoded)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded bytes.Buffer
	if err := reg.Decode(rec, bytes.NewReader(encoded.Bytes()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(raw, decoded.Bytes()) {
		t.Fatal("round-trip mismatch")
	}

	ratio := 100 * float64(encoded.Len()) / float64(len(raw))
	t.Logf("codec used: %s | %d -> %d bytes (%.1f%%)", rec.Codec, len(raw), encoded.Len(), ratio)
}
