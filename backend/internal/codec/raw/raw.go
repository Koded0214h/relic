package raw

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/jpg"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

const Name = "raw-preview"

var extensions = map[string]bool{
	".cr2": true, ".cr3": true, ".nef": true, ".arw": true,
	".dng": true, ".raf": true, ".orf": true, ".rw2": true,
}

// Codec shrinks a camera RAW by transcoding its largest embedded JPEG
// preview (usually the bulk of the file) with the jpg codec, leaving the
// surrounding RAW bytes untouched. Decode splices the reconstructed
// preview back in, so the round-trip is bit-exact.
type Codec struct {
	jpeg codec.Codec
}

func New() codec.Codec {
	return Codec{jpeg: jpg.New()}
}

func (c Codec) Name() string { return Name }

func (c Codec) CanHandle(f types.File) bool {
	return extensions[strings.ToLower(f.Ext)]
}

func (c Codec) Encode(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return types.Recipe{}, err
	}

	start, end, ok := largestJPEG(raw)
	if !ok {
		return types.Recipe{}, fmt.Errorf("raw: no embedded JPEG preview found")
	}
	preview := raw[start:end]

	previewFile := types.File{
		Path: f.Path + "#preview",
		Size: int64(len(preview)),
		Ext:  ".jpg",
		Head: preview[:min(len(preview), 64*1024)],
	}

	var compressedPreview bytes.Buffer
	previewRecipe, err := c.jpeg.Encode(previewFile, bytes.NewReader(preview), &compressedPreview)
	if err != nil {
		return types.Recipe{}, fmt.Errorf("raw: compress preview: %w", err)
	}

	before := raw[:start]
	after := raw[end:]

	if _, err := dst.Write(before); err != nil {
		return types.Recipe{}, err
	}
	if _, err := dst.Write(compressedPreview.Bytes()); err != nil {
		return types.Recipe{}, err
	}
	if _, err := dst.Write(after); err != nil {
		return types.Recipe{}, err
	}

	return types.Recipe{
		Codec:   Name,
		Version: 1,
		Params: map[string]string{
			"preview_codec": previewRecipe.Codec,
		},
		Blob: encodeOffsets(len(before), compressedPreview.Len(), len(after)),
	}, nil
}

func (c Codec) Decode(r types.Recipe, src io.Reader, dst io.Writer) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	beforeLen, previewLen, _, err := decodeOffsets(r.Blob)
	if err != nil {
		return fmt.Errorf("raw: bad offsets %q: %w", r.Blob, err)
	}
	if beforeLen < 0 || previewLen < 0 || beforeLen+previewLen > len(data) {
		return fmt.Errorf("raw: offsets out of range for %d-byte payload", len(data))
	}

	before := data[:beforeLen]
	compressedPreview := data[beforeLen : beforeLen+previewLen]
	after := data[beforeLen+previewLen:]

	previewRecipe := types.Recipe{Codec: r.Params["preview_codec"], Version: 1}

	var preview bytes.Buffer
	if err := c.jpeg.Decode(previewRecipe, bytes.NewReader(compressedPreview), &preview); err != nil {
		return fmt.Errorf("raw: decompress preview: %w", err)
	}

	if _, err := dst.Write(before); err != nil {
		return err
	}
	if _, err := dst.Write(preview.Bytes()); err != nil {
		return err
	}
	_, err = dst.Write(after)
	return err
}

// largestJPEG returns the byte range [start,end) of the largest
// FFD8..FFD9-delimited stream in data. RAW files carry more than one
// embedded JPEG — a full-size preview whose own EXIF block usually nests a
// smaller thumbnail — so the scan tracks SOI/EOI nesting depth rather than
// stopping at the first EOI. Assumes well-formed JPEGs: a stray FFD8/FFD9
// inside a maker-note or ICC blob would skew the match (acceptable for a
// preview heuristic, not a general JPEG parser).
func largestJPEG(data []byte) (start, end int, ok bool) {
	bestStart, bestEnd := -1, -1
	i := 0
	for i+1 < len(data) {
		if data[i] != 0xFF || data[i+1] != 0xD8 {
			i++
			continue
		}
		// SOI at i — walk to its matching EOI, counting nested SOIs.
		depth, j := 1, i+2
		for j+1 < len(data) && depth > 0 {
			if data[j] == 0xFF && data[j+1] == 0xD8 {
				depth++
				j += 2
			} else if data[j] == 0xFF && data[j+1] == 0xD9 {
				depth--
				j += 2
			} else {
				j++
			}
		}
		if depth != 0 {
			i++ // unterminated — not a usable JPEG
			continue
		}
		if j-i > bestEnd-bestStart {
			bestStart, bestEnd = i, j
		}
		i = j
	}
	if bestStart < 0 {
		return 0, 0, false
	}
	return bestStart, bestEnd, true
}

// encodeOffsets / decodeOffsets use a deliberately crude comma-separated
// text form. Fine for prototyping; swap for a compact binary layout in
// Blob before shipping.
func encodeOffsets(before, preview, after int) []byte {
	return fmt.Appendf(nil, "%d, %d, %d", before, preview, after)
}

func decodeOffsets(b []byte) (before, preview, after int, err error) {
	_, err = fmt.Sscanf(string(b), "%d, %d, %d", &before, &preview, &after)
	return
}
