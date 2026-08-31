package codec_test

import (
	"bytes"
	"testing"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

func TestGenericRoundTrip(t *testing.T) {
	reg := codec.NewRegistry(generic.New())

	original := bytes.Repeat([]byte("relic archive test data "), 1000)
	f := types.File{Path: "test.bin", Size: int64(len(original)), Ext: ".bin"}

	var encoded bytes.Buffer
	rec, err := reg.EncodeVerified(f, bytes.NewReader(original), &encoded)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if rec.Codec != generic.Name {
		t.Fatalf("expected generic codec, got %s", rec.Codec)
	}

	var decoded bytes.Buffer
	if err := reg.Decode(rec, bytes.NewReader(encoded.Bytes()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(original, decoded.Bytes()) {
		t.Fatal("round-trip mismatch")
	}

	t.Logf("original: %d bytes, encoded: %d bytes (%.1f%%)",
		len(original), encoded.Len(), 100*float64(encoded.Len())/float64(len(original)))
}