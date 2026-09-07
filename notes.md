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

---

## Benchmark harness (relic-bench) (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

- **`backend/cmd/relic-bench/main.go`** (new, was pre-existing untracked but
  did NOT compile). Walks a directory (arg, default `internal/testdata`), runs
  every file through the real `codec.NewRegistry(generic, jpg, raw)` via
  `EncodeVerified` + `Decode`, and prints a per-file table plus per-codec and
  total roll-ups (original → encoded, ratio, verify ✓/✗).
  - Fixed to compile: `string.ToLower`→`strings.ToLower`; `func printaTable`
    (never called) → `func printTable`, and its param `[]results`→`[]result`.
  - Dropped the redundant local `func min` (builtin, go.mod is 1.26).
  - Made the "BY CODEC" roll-up iterate in sorted codec-name order so runs are
    comparable (map iteration was random).
- **`backend/Makefile`** — added `bench` target (`go run ./cmd/relic-bench`)
  and put it on `.PHONY`.

### First real run

`go run ./cmd/relic-bench ~/Pictures/tutoring-post` (6 files, mixed):

```
FILE                                   EXT    CODEC        ORIGINAL    ENCODED   RATIO   OK
.DS_Store                              .ds_store generic     6.0 KiB     335 B    5.4%   ✓
Screenshot ...10.18.51.png             .png   generic     210.1 KiB 194.8 KiB   92.7%   ✓
Screenshot ...10.19.15.png             .png   generic    1013.5 KiB 996.0 KiB   98.3%   ✓
Screenshot ...10.19.39.png             .png   generic       1.2 MiB   1.2 MiB   95.4%   ✓
me.HEIC                                .heic  generic    1006.9 KiB 972.5 KiB   96.6%   ✓
stackd1.jpg                            .jpg   jpg-jxl     188.0 KiB 121.6 KiB   64.7%   ✓

BY CODEC
  generic   5 files    3.4 MiB ->   3.3 MiB   saved  3.7%
  jpg-jxl   1 files  188.0 KiB -> 121.6 KiB   saved 35.3%
TOTAL: 3.6 MiB -> 3.4 MiB   saved 5.3%   FAILURES: 0
```

- **`jpg-jxl` saved 35.3%** on the one real JPEG — above the 20–25% doc target
  (small sample; a real shoot will vary).
- PNG / HEIC correctly route to `generic` and barely move (already compressed).
- `.DS_Store` compresses ~95% but that's a junk file, not a signal.
- No RAW in this folder, so `raw-preview` wasn't exercised here.
- All 6 verified byte-exact, 0 failures.

### Still open

- No RAW files handy — need a folder with `.arw`/`.cr2`/`.nef` to get a
  `raw-preview` number against the 30–50% target.
- Table's `%-6s` EXT column is blown out by `.ds_store` (9 chars) — cosmetic.
- Separate: `backend/internal/db/` appeared in the tree mid-session (someone
  started the index) and imports `modernc.org/sqlite`, which isn't in
  `go.mod` yet — so a module-wide `go build ./...` / `go vet ./...` currently
  fails there. Not part of this change; `./cmd/relic-bench` builds fine on its
  own.

---

## Wire SQLite index + download end to end (Features 1+2) (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

- **`go.mod` / `go.sum`** — `go get modernc.org/sqlite` (v1.58.0, pure-Go, no
  cgo) + `github.com/google/uuid`; `go mod tidy` pulled the modernc build-time
  dep tree.
- **`backend/internal/db/db.go`** — was broken pseudocode; rewrote. `Open`
  (`sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")` + `Ping`) and
  `Migrate` (applies every `//go:embed migrations/*.sql` file in name order;
  all `CREATE ... IF NOT EXISTS`, safe every boot).
- **`backend/internal/db/migrations/`** (new) — `0001_users.sql`,
  `0002_sessions.sql`, `0003_objects.sql` (numbering settled in the handoff:
  users/sessions are ours now too, objects is 0003 so it can't collide).
  `0003` is the `archived_files` table: uuid `id`, `shoot_id`, `path`,
  `original_size`, `hash` (object-store key), `stored_size`, `codec` +
  `codec_version` + `codec_params` (JSON) + `codec_blob` (recipe.Blob).
- **`backend/internal/db/archive.go`** — fixed `var af = ArchivedFile` →
  `var af ArchivedFile`. `InsertArchiveFile` / `GetArchivedFile` otherwise
  as written (JSON-marshals `Recipe.Params`, stores `Recipe.Blob` as BLOB).
- **`backend/internal/api/archive/archive.go`** — converged the half-migrated
  handler onto the package-var + `Init` pattern: `Init(rn *job.Runner,
  database *sql.DB)`, `Mount(r)` (no runner arg), dropped the `Handler`
  struct. The job's `onResult` callback now calls `db.InsertArchiveFile`, so
  every archived file gets a row.
- **`backend/internal/api/files/files.go`** — fixed syntax (`af. err :=` →
  `af, err :=`; `s*store.Store` → `s *store.Store`; the broken hand-rolled
  `filenameof` → `path.Base`). `download` looks up the row, streams the
  object through `registry.Decode` to `w`.
- **`backend/internal/server/server.go`** — slimmed to just routing:
  store/codec/job/db construction moved out to `main`; `archive.Mount(r)` /
  `files.Mount(r)`. Deleted the long-dead `server.JSON`/`server.Error` (and
  the `code: code` map-key bug in the latter) — `httpx` is the real one.
- **`backend/cmd/relic/main.go`** — `run()` now opens+migrates the DB, builds
  the store / registry / runner, calls `archive.Init` and `files.Init`, then
  `server.New(cfg)`. Also fixed the `"shtting down"` log typo.

### Verified end to end

`RELIC_PORT=8080 ./relic`, then:

```
POST /api/shoots/abc123/archive        -> {"job_id":"job_abc123"}, job -> done (3/3)

sqlite3 data/relic.db 'select id,path,codec,original_size,stored_size from archived_files'
  b0d81c0f-…  internal/testdata/sample.jpg   jpg-jxl  44658  36905
  c2c83d54-…  internal/testdata/sample1.txt  generic     60     73
  777205ff-…  internal/testdata/sample2.txt  generic     60     73

curl /api/files/<sample1 id>/download -o /tmp/restored.txt
diff /tmp/restored.txt internal/testdata/sample1.txt   -> no output  ✓
```

- `.txt` (generic) and `.jpg` (jpg-jxl) both restore **byte-identical**
  (`diff` / `cmp` clean).
- `Content-Disposition: attachment; filename="sample1.txt"`,
  `Content-Length: 60`. Bad id → 404.
- **Killed and restarted the server, re-downloaded → still byte-identical.**
  The path→hash→recipe mapping is in SQLite, so it survives a restart. That's
  Features 1 and 2 proven together.

### Still open

- **Integrity-model gap (known, deferred per handoff):** `files.download`
  runs `registry.Decode` straight into `w`. A decode failure after the first
  flush leaves the client a truncated body under a 200. Fix = decode into a
  buffer, re-hash against `af.Hash`, then write `w`. Comment left in the code.
- `internal/auth/` is in the tree (Feature 3, in progress) and does **not**
  compile yet (`argon2.IDkey` typo, unused imports, missing return) — so
  module-wide `go build ./...` still fails there. Everything wired this
  session builds/vets clean on its own; `go run ./cmd/relic` is unaffected
  (auth isn't mounted).
- Migrations run unconditionally on boot with no schema-version tracking —
  fine while they're all idempotent `IF NOT EXISTS`, revisit for real
  migrations.

---

## Auth: signup / login / session / middleware (Feature 3) (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

Handed-off auth code was broken pseudocode across four files (plus a
structural mistake) — rewrote to compile and pass the full loop.

- **Structural fix:** `internal/auth/auth.go` was `package authapi` living in
  the `internal/auth/` dir alongside `package auth` files (two packages, one
  dir = won't build) and imported its own directory. Moved it to
  **`internal/api/auth/auth.go`**, which is the path `server.go` / `main.go`
  already import as `authapi`.
- **`internal/api/auth/auth.go`** (`package authapi`) — HTTP layer. `Init(db,
  secure)`; `Mount` registers `POST /auth/{signup,login,logout}` and
  `GET /me` behind `auth.Middleware`. Fixed: `err !+ nil`, `strings.TrimSpce`,
  `maps[string]string`, `func me(w http.http.ResponseWriter…)`, `httpx.Error(W,
  …)`, `errr`/`err` mixups, `"/auth.logout"` route path. `signup` now checks
  `@` in email and password ≥ 8.
- **`internal/auth/session.go`** (`package auth`) — cookie + session store.
  Rewrote the mangled `Middlwware` (bad func literal, `Middlwware` typo,
  `UserIDFromSession(d, …)`); fixed `errors.new`, `time.Tiem`,
  `r.Context.Value`. `ClearCookie` now also sets `MaxAge:-1` so browsers
  actually drop it. 30-day TTL, `HttpOnly`, `SameSite=Lax`, `Secure` gated on
  `!cfg.Dev()`.
- **`internal/auth/password.go`** (`package auth`) — argon2id
  (64 MiB / t=3 / p=4), `salt$hash` base64 format, constant-time verify. Fixed
  the `prallelism` typo (the earlier `argon2.IDkey` is already `IDKey` here).
- **`internal/user/user.go`** — `Create` / `Authenticate` / `Get`. Fixed
  `WHERE emial`, `SELECT id, email DFROM users`, `ErrInavlidCredentials`, and
  an ignored error on the existence check.
- **`internal/db/migrations/`** — added `0003_objects.sql` (archived_files,
  from the previous session), `0004_shoots.sql` (shoots + shoot_files, from
  the handoff — Feature 4, tables only, no handlers yet).
- **`cmd/relic/main.go`** — `authapi.Init(database, !cfg.Dev())`.
- **`internal/server/server.go`** — `authapi.Mount(r)` under `/api` (this edit
  arrived from disk; kept).

### Verified end to end

`RELIC_PORT=8080 ./relic`, fresh DB:

| step | result |
|---|---|
| `POST /api/auth/signup` | `201` `{"email":"koded@relic.dev","id":"459abf43…"}`, sets `relic_session` (HttpOnly) |
| `GET /api/me` (with cookie) | `200`, same `{id,email}` — session resolves |
| `POST /api/auth/logout` | `204`, row deleted, cookie cleared |
| `GET /api/me` (after) | `401` — session gone |
| `POST /api/auth/login` (right pw) | `200` + working `/me` |
| login wrong pw / dup signup / pw < 8 / no cookie | `401` / `409` / `400` / `401` |
| **restart server, reuse cookie** | `/me` still `200` — session is in SQLite |

`.tables` → `archived_files sessions shoot_files shoots users` (migrations
0001–0004 all applied).

### Still open

- `internal/shoot/shoot.go` is an orphan stub in the tree (Feature 4 WIP) with
  unused imports — doesn't compile, isn't imported anywhere, so
  `go run ./cmd/relic` is fine but `go build ./...` module-wide fails there.
- No rate-limiting / lockout on `login`. Session cleanup (expired rows) is
  lazy — checked on read, never swept.
- Feature 4 (shoots + upload, the 2 GB / 200-file streaming cap) is tables
  only so far.

---

## Shoots + upload (Feature 4) (this session)

Commit: _pending — not committed yet; fill in hash after `git commit`_

### Summary of changes

Handed-off Feature-4 drafts were broken pseudocode across five files
(scrambled handler bodies, `htpp`, `chi.URLParams`, `fielpath`, `fund`,
`InsertArchivedFile`, undefined `mr`/`dir`/`sf`, `list()` holding create's
body, `remove()` holding upload's body, …). Rewrote them.

- **`internal/shoot/shoot.go`** (`package shoot`) — the domain layer.
  `Create`, `Get(shootID, userID)` (ownership-scoped; wrong id or wrong
  owner both → `ErrNotFound`), `ListForUser`, `Delete` (cascades
  shoot_files), `Stats` (count + total bytes, for the caps), `AddFile`,
  `ListFiles`. `MaxShootBytes = 2<<30` (2 GiB), `MaxShootFiles = 200`.
  `File.StagingPath` is `json:"-"` so the internal disk path isn't leaked.
- **`internal/api/shoots/shoots.go`** (`package shoots`, new) — HTTP layer.
  `Init(db, dataDir)` (creates `<dataDir>/staging`). Routes: `GET/POST
  /shoots`, `GET/DELETE /shoots/{id}`, `POST /shoots/{id}/files`.
  - `upload` streams `multipart` parts through `writeCapped`, which enforces
    the per-shoot byte cap **while copying** (`io.LimitReader(part,
    remaining+1)`, delete + `quota_exceeded` if it goes over) — a client
    can't beat it with a lying Content-Length. File-count cap checked per
    part. Staging files are `<uuid><original ext>` so the codec registry can
    still dispatch on extension at archive time.
- **`internal/api/archive/archive.go`** — `startArchive` now ownership-checks
  the shoot, pulls its `shoot_files` staging paths (404 / `no_files` as
  appropriate), and feeds those to `runner.Start` instead of the test glob.
  `job_<shootID>` job id. `onResult` → `db.InsertArchiveFile`.
- **`internal/api/files/files.go`** — `download` adds an ownership check
  (`shoot.Get(af.ShootID, userID)`), 404 on miss.
- **`internal/server/server.go`** — `New(cfg, database)`; `/api` splits into
  public (`authapi.Mount` — signup/login, plus its own guarded `/me`) and a
  `r.Group` behind `auth.Middleware(database)` wrapping shoots + archive +
  files.
- **`cmd/relic/main.go`** — `shoots.Init(database, cfg.DataDir)`;
  `server.New(cfg, database)`.

### Verified end to end

`RELIC_PORT=8080 ./relic`, fresh DB:

```
signup A → POST /api/shoots {"name":"Test Shoot"} → 201 {id,name}
POST /api/shoots/<id>/files  -F sample.jpg -F sample1.txt
  → 201 [{id,shoot_id,filename,size}, …]
GET  /api/shoots/<id>        → {id,name,files:[…]}
POST /api/shoots/<id>/archive → {"job_id":"job_<id>"}; job → done 2/2

archived_files (shoot_id=<id>):
  …  data/staging/<uuid>.jpg  jpg-jxl     ← ext preserved, specialized codec fired
  …  data/staging/<uuid>.txt  generic

download <jpg id> → cmp vs sample.jpg  → identical ✓
download <txt id> → cmp vs sample1.txt → identical ✓
```

**Security check (user B against user A's shoot):**

| request | result |
|---|---|
| `GET /api/shoots/<A's id>` | `404` |
| `GET /api/files/<A's archived id>/download` | `404` |
| `POST /api/shoots/<A's id>/archive` | `404` |
| any private route, no cookie | `401` |

### Bug caught + fixed this session

Uploads first stored staging files as bare `<uuid>` (no extension), so
`jpg.CanHandle` (ext + magic gated) never matched and the JPEG archived as
`generic` — silently losing the jpg-jxl / raw-preview compression from
Features 1–2. Staging name now carries the original extension.

### Still open

- `archived_files.path` is the staging path (`data/staging/<uuid>.jpg`), so
  the download `Content-Disposition` filename is a uuid, not the original
  name. Carry `shoot_files.filename` through if that matters.
- Orphan staging files: nothing deletes them after a successful archive, and
  a failed `AddFile` mid-batch leaves earlier parts on disk + in the DB.
- Integrity-model gap in `files.download` still deferred (buffer + re-hash).
- `login` still has no rate-limiting; expired sessions never swept.