package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/internal/codec/jpg"
	"github.com/Koded0214h/relic/backend/internal/codec/raw"
	"github.com/Koded0214h/relic/backend/pkg/types"
)


type result struct { 
	path      string
	ext       string
	codec     string
	original  int64
	encoded   int64
	verified  bool
	err       error
	duration  time.Duration
}

func main() {
	dir := "internal/testdata"
	if len(os.Args) > 1 { dir = os.Args[1] }

	reg := codec.NewRegistry(generic.New(), jpg.New(), raw.New())

	var results []result
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		results = append(results, bench(reg, path))
		return nil
	})
	
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk error:", err)
		os.Exit(1)
	}

	if len(results) == 0 { 
		fmt.Println("no files found in", dir) 
		return 
	}

	printTable(results)
	printSummary(results)

}

func bench(reg codec.Registry, path string) result {
	raw, err := os.ReadFile(path)
	if err != nil { return result{path: path, err: err} }

	ext := strings.ToLower(filepath.Ext(path))
	f:= types.File{
		Path: path,
		Size: int64(len(raw)),
		Ext: ext,
		Head: raw[:min(len(raw), 64*1024)],
	}

	start := time.Now()

	var encoded bytes.Buffer
	rec, err := reg.EncodeVerified(f, bytes.NewReader(raw), &encoded)
	if err != nil { return result{path: path, ext: ext, err: err, duration: time.Since(start)} }

	var decoded bytes.Buffer
	verifyErr := reg.Decode(rec, bytes.NewReader(encoded.Bytes()), &decoded)
	verified:= verifyErr == nil && bytes.Equal(raw, decoded.Bytes())

	return result {
		path: path,
		ext: ext,
		codec: rec.Codec,
		original: int64(len(raw)),
		encoded: int64(encoded.Len()),
		verified: verified,
		duration: time.Since(start),
	}
}

func printTable(results []result) {
	sort.Slice(results, func(i, j int) bool { return results[i].path < results[j].path })

	fmt.Printf("%-40s %-6s %-12s %10s %10s %8s %6s\n",
		"FILE", "EXT", "CODEC", "ORIGINAL", "ENCODED", "RATIO", "OK")
	fmt.Println(strings.Repeat("-", 100))

	for _, r := range results {
		name := filepath.Base(r.path)
		if r.err != nil {
			fmt.Printf("%-40s %-6s %-12s %10s %10s %8s %6s  (%v)\n",
				name, r.ext, "-", "-", "-", "-", "ERR", r.err)
			continue
		}
		ratio := 100 * float64(r.encoded) / float64(r.original)
		ok := "✓"
		fmt.Printf("%-40s %-6s %-12s %10s %10s %7.1f%% %6s\n",
			name, r.ext, r.codec, humanBytes(r.original), humanBytes(r.encoded), ratio, ok)

	}
}

func printSummary(results []result) {
	byCodec := map[string]struct { orig, enc int64; n int} {}
	var totalOrig, totalEnc int64
	failures := 0
	for _, r := range results {
		if r.err != nil || !r.verified {
			failures++
			continue
		}
		s := byCodec[r.codec]
		s.orig += r.original
		s.enc += r.encoded
		s.n++
		byCodec[r.codec] = s
		totalOrig += r.original
		totalEnc += r.encoded
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 100))
	fmt.Println("BY CODEC")
	names := make([]string, 0, len(byCodec))
	for name := range byCodec {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := byCodec[name]
		ratio := 100 * float64(s.enc) / float64(s.orig)
		saved := 100 - ratio
		fmt.Printf("  %-14s %2d files  %10s -> %10s   saved %.1f%%\n",
			name, s.n, humanBytes(s.orig), humanBytes(s.enc), saved)
	}

	fmt.Println()
	if totalOrig > 0{
		overallRatio := 100 * float64(totalEnc) / float64(totalOrig)
		fmt.Printf("TOTAL: %s -> %s   saved %.1f%%\n",
			humanBytes(totalOrig), humanBytes(totalEnc), 100-overallRatio)
	}

	if failures > 0 {
		fmt.Printf("FAILURES: %d file(s) errored or faild verification\n", failures)
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit { return fmt.Sprintf("%d B",n) }
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
