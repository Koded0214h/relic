package codec

import (
	"io"
	
	"github.com/Koded0214h/relic/backend/pkg/types"
)

type Codec interface {
	Name() string
	CanHandle(f types.File) bool
	Encode(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error)
	Decode(r types.Recipe, src io.Reader, dst io.Writer) error
}

type Registry interface {
	EncodeVerified(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error)
	Decode(r types.Recipe, src io.Reader, dst io.Writer) error
}