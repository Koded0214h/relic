package codec

import (
	"bytes"
	"fmt"
	"io"

	"github.com/Koded0214h/relic/backend/pkg/types"
)

type registry struct {
	codecs []Codec
	generic Codec
}

func NewRegistry(generic Codec, codecs ...Codec) Registry {
	return &registry{codecs: codecs, generic: generic}
}

func (r *registry) pick(f types.File) Codec {
	for _, c := range r.codecs {
		if c.CanHandle(f) {
			return c
		}
	}

	return r.generic
}

func (r* registry) EncodeVerified(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return types.Recipe{}, fmt.Errorf("read source: %w", err)
	}

	c := r.pick(f)
	if rec, ok := tryEncode(c, f, raw); ok {
		_, err := dst.Write(rec.encoded)
		return rec.recipe, err
	}

	rec, ok := tryEncode(r.generic, f, raw)
	if !ok {
		return types.Recipe{}, fmt.Errorf("generic codec failed round-trip for %s — this should never happen", f.Path)
	}
	_, err = dst.Write(rec.encoded)
	return rec.recipe, err
}

type verifiedEncode struct {
	recipe types.Recipe
	encoded []byte
}

func tryEncode(c Codec, f types.File, raw []byte) (verifiedEncode, bool) {
	var encoded bytes.Buffer

	recipe, err := c.Encode(f, bytes.NewReader(raw), &encoded)

	if err != nil { return verifiedEncode{}, false }

	var decoded bytes.Buffer
	if err := c.Decode(recipe, bytes.NewReader(encoded.Bytes()), &decoded); err != nil {
		return verifiedEncode{}, false
	}

	if !bytes.Equal(raw, decoded.Bytes()) { return verifiedEncode{}, false }

	return verifiedEncode{recipe: recipe, encoded: encoded.Bytes()}, true
}


func (r *registry) Decode(rec types.Recipe, src io.Reader, dst io.Writer) error {
	for _, c := range append(r.codecs, r.generic) {
		if c.Name() == rec.Codec {
			return c.Decode(rec, src, dst)
		}
	}
	return fmt.Errorf("no codec registered for %q", rec.Codec)
}