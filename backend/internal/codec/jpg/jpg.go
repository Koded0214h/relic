package jpg

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/pkg/types"
)

const Name = "jpg-jxl"

type Codec struct {
	available bool
}

// New checks once at startup whether cjxl/djxl exist. If not, CanHandle
// always returns false and every JPEG falls through to generic — the
// registry's fallback path handles this with zero special-casing.
func New() codec.Codec {
	_, errC := exec.LookPath("cjxl")
	_, errD := exec.LookPath("djxl")
	return Codec{available: errC == nil && errD == nil}
}

func (c Codec) Name() string { return Name }

func (c Codec) CanHandle(f types.File) bool {
	if !c.available {
		return false
	}
	ext := strings.ToLower(f.Ext)
	if ext != ".jpg" && ext != ".jpeg" {
		return false
	}
	// magic bytes: JPEG starts FF D8, ends FF D9 — cheap sanity check
	// before shelling out to a subprocess.
	return len(f.Head) >= 2 && f.Head[0] == 0xFF && f.Head[1] == 0xD8
}

func (c Codec) Encode(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error) {
	inFile, err := os.CreateTemp("", "relic-jxl-in-*.jpg")
	if err != nil {
		return types.Recipe{}, err
	}
	defer os.Remove(inFile.Name())
	if _, err := io.Copy(inFile, src); err != nil {
		inFile.Close()
		return types.Recipe{}, err
	}
	inFile.Close()

	outPath := inFile.Name() + ".jxl"
	defer os.Remove(outPath)

	// --lossless_jpeg=1: bit-exact JPEG transcode (djxl reconstructs the
	// original byte stream). It's cjxl's implicit default for JPEG input,
	// but the flag spelling changed across libjxl versions (the old bare
	// -j no longer parses in 0.11.x), so pass it explicitly.
	cmd := exec.Command("cjxl", inFile.Name(), outPath, "--lossless_jpeg=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return types.Recipe{}, fmt.Errorf("cjxl: %w: %s", err, stderr.String())
	}

	out, err := os.Open(outPath)
	if err != nil {
		return types.Recipe{}, err
	}
	defer out.Close()
	if _, err := io.Copy(dst, out); err != nil {
		return types.Recipe{}, err
	}

	return types.Recipe{Codec: Name, Version: 1}, nil
}

func (c Codec) Decode(_ types.Recipe, src io.Reader, dst io.Writer) error {
	inFile, err := os.CreateTemp("", "relic-jxl-dec-in-*.jxl")
	if err != nil {
		return err
	}
	defer os.Remove(inFile.Name())
	if _, err := io.Copy(inFile, src); err != nil {
		inFile.Close()
		return err
	}
	inFile.Close()

	outPath := inFile.Name() + ".jpg"
	defer os.Remove(outPath)

	cmd := exec.Command("djxl", inFile.Name(), outPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("djxl: %w: %s", err, stderr.String())
	}

	out, err := os.Open(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(dst, out)
	return err
}