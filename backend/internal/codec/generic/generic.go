package generic

import (
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

const Name = "generic"

type Codec struct{}

func New() codec.Codec { return Codec{} }

func (Codec) Name() string { return Name }

// CanHandle is always true — it's the fallback of last resort.
func (Codec) CanHandle(types.File) bool { return true }

func (Codec) Encode(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error) {
	w, err := zstd.NewWriter(dst)
	if err != nil {
		return types.Recipe{}, err
	}
	if _, err := io.Copy(w, src); err != nil {
		_ = w.Close()
		return types.Recipe{}, err
	}
	if err := w.Close(); err != nil {
		return types.Recipe{}, err
	}
	return types.Recipe{Codec: Name, Version: 1}, nil
}

func (Codec) Decode(_ types.Recipe, src io.Reader, dst io.Writer) error {
	r, err := zstd.NewReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(dst, r)
	return err
}