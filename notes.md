# Session notes

## Frontend (changed this session)

- **`frontend/src/App.tsx`** — removed the unused `import React from 'react'`.
  Under React 19 + the new JSX transform the import is not needed, and
  `tsconfig.app.json` has `noUnusedLocals` enabled, so `tsc -b` failed with
  `TS6133: 'React' is declared but its value is never read`. That broke
  `npm run build` locally and on Vercel. Build passes after removal.

### Vercel settings for the frontend

- Build Command: `npm run build` (default is fine)
- Root Directory: `frontend` (repo root is the monorepo)
- Output Directory: `dist`
- Framework Preset: Vite

---

## Backend (uncommitted work already in the tree — not authored this session)

Working-tree state as of this session: `backend/go.mod` / `backend/go.sum`
modified, plus two new untracked packages. Summary of what's there:

### `backend/go.mod`, `backend/go.sum`

- Added dependency `github.com/klauspost/compress v1.19.2` (used for zstd
  in the generic codec).

### `backend/pkg/types/types.go` (new)

Shared value types for the codec layer:

- `File` — describes an input file: `Path`, `Size`, `Ext`, `Head` (leading bytes
  for sniffing).
- `Recipe` — instructions to reconstruct a file: `Codec` name, `Version`,
  `Params` map, `Blob` payload.

### `backend/internal/codec/codec.go` (new)

Core interfaces:

- `Codec` — `Name()`, `CanHandle(File)`, `Encode(...) (Recipe, error)`,
  `Decode(Recipe, ...) error`.
- `Registry` — `EncodeVerified(...)` and `Decode(...)`.

### `backend/internal/codec/registry.go` (new)

Default `Registry` implementation:

- `NewRegistry(generic Codec, codecs ...Codec)` — ordered list of specific
  codecs plus a mandatory generic fallback.
- `pick(File)` — first codec whose `CanHandle` returns true, else the generic.
- `EncodeVerified` — reads the whole source, tries the picked codec, and only
  accepts the result if a full encode→decode round-trip reproduces the original
  bytes (`tryEncode`). Falls back to the generic codec; if that also fails the
  round-trip it errors ("should never happen").
- `Decode` — dispatches to the codec whose `Name()` matches `Recipe.Codec`.

### `backend/internal/codec/generic/generic.go` (new)

Fallback codec of last resort:

- `CanHandle` always true.
- `Encode` — streams the source through a zstd writer.
- `Decode` — streams back through a zstd reader.
- Emits `Recipe{Codec: "generic", Version: 1}`.

### `backend/internal/codec/registry_test.go` (new)

- `TestGenericRoundTrip` — registers only the generic codec, encodes ~24 KB of
  repeating data via `EncodeVerified`, asserts the generic codec was chosen,
  decodes, and checks byte-for-byte equality. Logs the compression ratio.

### Notes / observations

- `registry.go` refers to unexported fields `rec.encoded` / `rec.recipe` on the
  `verifiedEncode` struct — consistent within the file, fine.
- `tryEncode` calls the specific codec, but on fallback the second attempt uses
  `r.generic` directly rather than re-running `pick`; intentional.
- Run `cd backend && go build ./... && go test ./...` to verify before committing.

---

## Backend HTTP API layer (this session)

Commit: f139b14

### Summary of changes

- **`backend/internal/server/router.go` → `backend/internal/server/server.go`**
  — renamed; the `/api` route group now actually mounts `archive.Mount(r)` and
  `files.Mount(r)` (previously commented-out stubs). Auth/shoots still stubbed
  (Ridwan). Health handler now delegates to `httpx.JSON`.
- **`backend/internal/httpx/httpx.go`** (new) — shared `JSON(w, status, v)` and
  `Error(w, status, code, msg)` response helpers, extracted so the api packages
  don't import `server`.
- **`backend/internal/api/archive/archive.go`** (new) — in-memory archive job
  endpoints: `POST /api/shoots/{shootID}/archive` (starts a job, returns
  `job_id`), `GET /api/jobs/{jobID}` (job state), `GET /api/jobs/{jobID}/events`
  (SSE progress stream). Progress is simulated by `fakeProgress` (42 steps,
  ~150ms each). Jobs held in a package-level `map` guarded by a `sync.Mutex`.
- **`backend/internal/api/files/files.go`** (new) — `GET /api/files/{fileID}/download`
  returning fixture `application/octet-stream` bytes.
- **`backend/test/test.sh`** (new) — curl snippets exercising the archive flow.

### Follow-ups / known issues

- `server.go` still contains dead `JSON` / `Error` copies (superseded by
  `httpx`); `server.Error` also has the `map{... code: code}` key bug that
  `httpx.Error` fixed. Delete them and the unused `encoding/json` import.
- `archive.getJob` / `archive.streamJob` read `*jobState` fields after releasing
  the mutex while `fakeProgress` writes under it — data race. Marshal/copy while
  holding the lock. `make test` runs `-race` but nothing exercises it yet.
- `go build ./...` and `go vet ./...` pass; `go test ./... -race` passes
  (no tests cover the new packages).

---

## Content-addressed object store (this session)

Commit: d9c87dc

### Summary of changes

- **`backend/internal/store/store.go`** (new) — `store` package: a
  content-addressed blob store on the local filesystem.
  - `New(root)` — creates the root dir, returns `*Store`.
  - `Put(io.Reader)` — streams the source into a temp file in `root` while
    hashing with SHA-256, then atomically `os.Rename`s it to a sharded path
    `root/ab/cd/<full-hex-hash>`. If the destination already exists it drops the
    temp file and returns the existing hash (dedup). Returns `(hash, size, err)`.
  - `Get(hash)` — opens the object file, returns an `io.ReadCloser`.
  - `Has(hash)` — stat check.
  - `path(hash)` — 2×2 hex-prefix sharding; falls back to a flat path for
    hashes shorter than 4 chars (can't happen with SHA-256).
- **`backend/internal/store/store_test.go`** (new) — `TestPutGetRoundTrip`
  (put then get returns identical bytes, size matches) and `TestDedup`
  (same bytes twice yields the same hash and exactly one file on disk).

### Notes / known issues

- Package was briefly created at repo-root `internal/store/` (outside the
  `backend/` Go module, so the test's import path didn't resolve); now moved
  under `backend/internal/store/`.
- `store.go` mkdir error string has a typo: `"stor: mkdir: %w"` (missing "e").
- Concurrent `Put`s are safe (unique temp names + rename), no mutex needed.
- `go build`, `go vet`, and `go test ./... -race` all pass; store tests green.

---

## Archive job runner (this session)

Commit: c8500e0

### Summary of changes

- **`backend/internal/job/job.go`** (new) — `job` package: runs real archive
  jobs, tying together `codec`, `store`, and `pkg/types`.
  - `Job` — id/state/done/total/error, JSON-tagged, with its own `sync.Mutex`
    and a `snapshot()` that returns a lock-free copy for readers.
  - `State` string enum: `running` / `done` / `error`.
  - `Runner` — holds a `*store.Store`, a `codec.Registry`, and a
    mutex-guarded `map[string]*Job`. `NewRunner(s, r)` constructs it.
  - `Start(jobID, paths, onResult)` — registers a `Job`, kicks off `run` in a
    goroutine, returns the `*Job` immediately.
  - `run` — archives each path in turn; on error sets `StateError` + message
    and stops; otherwise increments `Done`, calls `onResult`, and sleeps 5ms so
    SSE pollers can observe intermediate progress. Sets `StateDone` at the end.
  - `archiveOne(path)` — opens the file, stats it, reads a 64 KB head for
    sniffing, seeks back, builds a `types.File`, runs `registry.EncodeVerified`
    into an OS temp file, then seeks and streams that into `store.Put`. Returns
    a `Result{Path, Hash, Size, StoredSize, Recipe}`.
  - `ext(path)` — local basename-aware extension helper.
- **`backend/internal/job/job_test.go`** (new) — `TestArchiveJobEndToEnd`:
  real `store` + generic-only `codec` registry, archives a sample file, polls
  `Runner.Get` until `done`, asserts one `Result` and that the object landed in
  the store. Logs original→stored sizes and codec name.

### Notes / observations

- `archiveOne` fills the head buffer with a single `f.Read` (may short-read);
  `io.ReadFull`/`ReadAtLeast` would be stricter, but it's only a sniff buffer.
- `ext()` duplicates `path/filepath.Ext` — could use the stdlib one.
- Job progress reads/writes are properly guarded by `Job.mu`; the test's
  `results` slice is written only from the single runner goroutine and read
  after a `snapshot()` lock, so `-race` stays clean.
- `go build`, `go vet`, and `go test ./... -race` all pass; job test green.

---

## Wire the real job runner into the archive API (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

- **`backend/internal/testdata/sample1.txt`, `sample2.txt`** (new) — stand-in
  archive corpus until Ridwan's upload endpoint exists. Not gitignored.
- **`backend/internal/api/archive/archive.go`** — replaced the fake in-memory
  `jobState` map + `fakeProgress` goroutine with a real `*job.Runner`.
  - `Handler{runner}` struct; `Mount(r, runner)` now takes the runner
    (dependency-injected from `server.New`) instead of a package global —
    diverges from the `Init()`/package-var sketch in the task, same effect.
  - `startArchive` globs `internal/testdata/*` (via `testCorpusPaths()`) and
    hands the paths to `runner.Start`; the `onResult` callback is a TODO stub
    for index persistence once `internal/db` exists.
  - `getJob` / `streamJob` read from `runner.Get`; SSE loop marshals the job
    status, flushes, and `time.Sleep(200ms)` between frames until
    `StateDone`/`StateError`.
- **`backend/internal/job/job.go`** — split the reader-facing view out of
  `Job`: new `Status` struct carries the JSON tags, `snapshot()` and
  `Runner.Get` now return `Status` (no `sync.Mutex`). Fixes `go vet`
  "copies lock value" at the two archive.go call sites; `Job` keeps the mutex
  and drops its now-unused JSON tags.
- **`backend/internal/server/server.go`** — `New` constructs the object store
  (`cfg.DataDir + "/objects"`), a generic-only `codec.NewRegistry`, and a
  `job.NewRunner`, then passes the runner to `archive.Mount`. Also fixed
  `log.Fatal` → `log.Fatalf` (was a Printf-directive-without-format bug, also
  flagged by vet).

### Verified live

`RELIC_DATA_DIR=… RELIC_PORT=8080 go run ./cmd/relic`, then:

- `POST /api/shoots/abc123/archive` → `{"job_id":"job_abc123"}` (202)
- `GET /api/jobs/job_abc123/events` → two SSE frames, `state:"running" done:2`
  then `state:"done" done:2 total:2`
- `find data/objects -type f` → two content-addressed blobs in the
  `ab/cd/<hash>` sharded layout.

Full path is live: HTTP → job runner → codec verify → content-addressed store,
streamed back over SSE in the shape Ridwan is coding against.

### Still open

- No index yet — nothing records which hash/recipe belongs to which file, so
  `restore` has nothing to look up. That's `internal/db`, shared with Ridwan
  (`users`/`shoots` tables). Needs a migration-split sync with him before
  building `objects`/`recipes`.
- `archiveOne` short-read on the 64 KB head buffer (pre-existing).
- `ext()` still duplicates `path/filepath.Ext` (pre-existing).
- `server.go` still carries dead `JSON`/`Error` copies with the `code: code`
  map-key bug (pre-existing; `httpx` is the real one).
- `go build`, `go vet`, `go test ./... -race` all pass.

---

## JPEG codec (jpg-jxl) + registry wiring (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

- **`backend/internal/codec/jpg/jpg.go`** (new, was pre-existing untracked) —
  `jpg` codec (`Name = "jpg-jxl"`), a lossless JPEG↔JXL transcoder that shells
  out to `cjxl` / `djxl`.
  - `New()` runs `exec.LookPath` for both binaries once; if either is missing,
    `available` is false and `CanHandle` always returns false, so JPEGs fall
    through to `generic` with no special-casing.
  - `CanHandle` — requires `available`, a `.jpg`/`.jpeg` ext, and an
    `FF D8` magic-byte prefix before it will shell out.
  - `Encode` — temp-file in, `cjxl … --lossless_jpeg=1` → `.jxl`, stream to
    `dst`, `Recipe{Codec:"jpg-jxl", Version:1}`.
  - `Decode` — temp-file in, `djxl` → `.jpg`, stream to `dst`.
  - Fixed two bugs in the pre-existing file: import path was
    `github.com/Koded0214/…` (missing `h`), and the encode command used the
    bare `-j` flag which no longer parses in libjxl 0.11.x — silently failed
    the round-trip so every JPEG fell back to `generic`. Now
    `--lossless_jpeg=1`, which `djxl` reconstructs byte-for-byte.
- **`backend/internal/codec/jpg/jpg_test.go`** (new) — `TestJPEGRoundTripAndRatio`
  reads `internal/testdata/sample.jpg` (`t.Skip` if absent), runs it through
  `EncodeVerified` + `Decode`, asserts an exact byte round-trip, and logs the
  codec used and the compression ratio.
- **`backend/internal/server/server.go`** — registry is now
  `codec.NewRegistry(generic.New(), jpg.New())`. `NewRegistry(generic, codecs…)`
  tries `codecs` in order before the generic fallback, so `jpg` gets first
  refusal on anything it claims. (Task said "currently main.go" — the registry
  actually moved to `server.New` last session; the stray `jpg` import added to
  `main.go` was reverted.)
- **`backend/internal/testdata/sample.jpg`** (new, ~44 KB) — real baseline JPEG
  (413×531, from `~/Pictures/koded.jpeg`) for the jpg test and the `make run`
  corpus. (An earlier 5.6 MB wallpaper `sample3.jpg` was used during bring-up,
  then dropped in favour of this smaller fixture.)

### Verified

`go test ./internal/codec/jpg/... -v`:

```
codec used: jpg-jxl | 44658 -> 36905 bytes (82.6%)
--- PASS: TestJPEGRoundTripAndRatio
```

`jpg-jxl` (not `generic` → no fallback), 17.4% saving, byte-exact round-trip.
Also confirmed end to end through the archive API during bring-up: a JPEG in
the corpus stores as its `.jxl` transcode and restores identically, `.txt`
files still go through generic zstd.

### Still open

- Same as previous section: no index (`internal/db`) yet — needs the
  migration-split sync with Ridwan.
- `go build`, `go vet`, `go test ./... -race` all pass.

---

## RAW codec (raw-preview) + registry wiring (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

- **`backend/internal/codec/raw/raw.go`** (new, was pre-existing untracked but
  did NOT compile — ~8 syntax/typo errors). What it does: finds the largest
  embedded JPEG in a camera RAW (`.cr2/.cr3/.nef/.arw/.dng/.raf/.orf/.rw2`),
  transcodes just that preview with the `jpg` codec, and leaves the
  surrounding RAW bytes untouched. `Decode` splices the reconstructed preview
  back in → bit-exact round-trip. `Recipe.Params["preview_codec"]` records the
  inner codec; `Recipe.Blob` holds `"before, preview, after"` byte lengths.
  - Fixed to compile: `stringa`→`strings`; unexported `canHandle`→`CanHandle`
    (interface wasn't satisfied); missing struct-literal commas in
    `previewFile`; `type.Recipe`→`types.Recipe`; `var previewbytes.Buffer`→
    `var preview bytes.Buffer`; `fmt.Sprintf`→`fmt.Sscanf` in `decodeOffsets`
    (and it had `_, err =` off a single-return call); label-with-no-statement
    from the `goto`.
  - **Real logic bug fixed:** `largestJPEG` stopped at the first `FFD9`. Every
    real RAW nests a thumbnail JPEG inside the preview's EXIF, so the first
    `FFD9` is the *thumbnail's* — the old scan extracted a broken
    header+thumbnail fragment, `cjxl` choked on it, and the registry silently
    fell back to `generic` for basically all real RAWs. Rewrote the scan to
    track SOI/EOI nesting depth (also drops the `goto`, per the note in the
    handoff). Documented the remaining assumption (a stray `FFD8/FFD9` in a
    maker-note/ICC blob could still skew it — fine for a preview heuristic).
  - Dropped the redundant `func min` (builtin since Go 1.21; go.mod is 1.26);
    `encodeOffsets` now uses `fmt.Appendf`.
- **`backend/internal/codec/raw/raw_test.go`** (new) —
  `TestRAWPreviewRoundTrip` is the handoff's test: reads
  `internal/testdata/sample.arw` and `t.Skip`s if absent (no real camera RAW
  checked in). `TestRAWSyntheticRoundTrip` (added) fabricates a RAW-shaped
  container — opaque header + the real `sample.jpg` (which itself carries an
  EXIF thumbnail, so it exercises the nesting fix) + trailer, ext `.dng` —
  and asserts an exact round-trip through the registry with
  `rec.Codec == "raw-preview"` (only enforced when `cjxl` is on PATH).
- **`backend/internal/server/server.go`** — registry is now
  `codec.NewRegistry(generic.New(), jpg.New(), raw.New())`.

### Verified

`go test ./internal/codec/raw/... -v`:

```
--- SKIP: TestRAWPreviewRoundTrip (no sample RAW in testdata)
    codec: raw-preview | 46458 -> 38705 bytes (83.3%)
--- PASS: TestRAWSyntheticRoundTrip
```

Synthetic RAW: `raw-preview` engaged (not the generic fallback), byte-exact
round-trip, ~17% saved (all from the embedded-preview transcode). Full
`go test ./... -race` green.

### Still open

- **Not yet tested against a real camera RAW.** Drop a `.arw`/`.cr2`/`.nef`
  into `internal/testdata/sample.arw` (adjust the name in the test) and rerun
  to confirm `largestJPEG` picks the right preview on real-world marker soup.
- Offset encoding in `Blob` is crude comma-separated text — replace with a
  compact binary form before shipping (flagged in the handoff, left as-is).
- Same index (`internal/db`) gap as the previous sections.