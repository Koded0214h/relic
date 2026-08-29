# Relic — Work Split

**Koded** and **Ridwan** · Go + React · 10 hrs/week each · 8 weeks

A web app, not a terminal tool. Accounts, upload, a real UI, and a compression engine underneath it.

The point of this document is that neither of you ever waits on the other.

---

## One reality check first

Browsers cannot upload a 186 GB shoot. Not with chunking, not with anything. So the web version is not "archive my whole library" — it is:

> Upload a shoot. Watch it get analysed and packed. See the timeline. Pull any original back, byte-for-byte.

Cap it at something like **2 GB / 200 files per shoot** and enforce it in the API. That's a real, demoable product, and it exercises every part of the system. The "point it at my 4 TB drive" version is a desktop agent that talks to the same API later — it is not in these 8 weeks.

Do not let scope drift here. It is the thing most likely to eat the season.

---

## Architecture

```
  React + Vite (TS)                    Go API                      Engine
 ┌──────────────────┐            ┌──────────────────┐        ┌──────────────┐
 │ auth pages       │  cookies   │ sessions         │        │ codec        │
 │ library          │◄──────────►│ users, shoots    │        │  registry    │
 │ upload + progress│   JSON     │ upload handling  │───────►│  verify      │
 │ shoot timeline   │◄──────────►│ archive jobs     │        │  generic/jpg │
 │ file restore     │    SSE     │ restore + verify │        │  /raw        │
 └──────────────────┘            └──────────────────┘        └──────────────┘
                                          │                          │
                                     SQLite index            content-addressed
                                                              object store
```

**Ridwan** owns the left two-thirds. **Koded** owns the right.

---

## The rule

**One person owns a package. Nobody edits a package they don't own.**

```
web/                        Ridwan     React, Vite, TS, Tailwind
internal/api/auth.go        Ridwan     signup, login, logout, me
internal/api/shoots.go      Ridwan     CRUD, upload intake
internal/auth/              Ridwan     sessions, argon2id, middleware
internal/user/              Ridwan     users, quotas, ownership checks

internal/api/archive.go     Koded      archive + job endpoints
internal/api/files.go       Koded      restore + download
internal/job/               Koded      queue, workers, progress
internal/codec/             Koded      registry, verify harness
internal/codec/generic/     Koded      zstd fallback
internal/codec/jpg/         Koded      jxl transcode
internal/codec/raw/         Koded      preview extraction
internal/store/             Koded      content-addressed objects

internal/db/                SHARED     schema + migrations, both review
pkg/types/                  SHARED     frozen day 1
cmd/autopsy/main.go         SHARED     ~30 lines, rarely touched
```

Each API package exposes `Mount(r chi.Router)` and registers its own routes. Nobody edits a shared router file.

---

## Contract 1 — the HTTP API

Freeze this in session one. It is the seam between the two of you.

```
POST   /api/auth/signup          {email, password}      → 201, sets cookie
POST   /api/auth/login           {email, password}      → 200, sets cookie
POST   /api/auth/logout                                 → 204
GET    /api/me                                          → {id, email}

GET    /api/shoots                                      → [{id, name, files, bytes, saved_pct}]
POST   /api/shoots               {name}                 → 201 {id}
POST   /api/shoots/:id/files     multipart              → 201 [{file_id, name, size}]
DELETE /api/shoots/:id                                  → 204

POST   /api/shoots/:id/archive                          → 202 {job_id}
GET    /api/jobs/:id                                    → {state, done, total, error}
GET    /api/jobs/:id/events                             → SSE progress stream

GET    /api/shoots/:id           → {stats, savings:{compression_pct, dedup_pct}, timeline[], sequences[]}
GET    /api/shoots/:id/files     → [{id, name, size, stored_size, codec, verified}]
GET    /api/files/:id/download   → streams the original, 500 if the hash doesn't match
```

Errors are always `{"error": "...", "code": "..."}`. Auth is an HttpOnly session cookie, not a JWT — you don't need token rotation for this and sessions are ~150 lines.

Ownership checks live in Ridwan's middleware. Koded's handlers assume the user in context already owns the shoot.

## Contract 2 — the codec interface

```go
package types

type File struct {
    Path string
    Size int64
    Ext  string
    Head []byte    // first 64KB, already read
}

type Recipe struct {
    Codec   string            // "generic", "jpg-jxl", "raw-preview"
    Version int
    Params  map[string]string
    Blob    []byte            // codec-private reconstruction data
}
```

```go
package codec

type Codec interface {
    Name() string
    CanHandle(f types.File) bool
    Encode(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error)
    Decode(r types.Recipe, src io.Reader, dst io.Writer) error
}

// EncodeVerified runs the chosen codec, immediately checks
// Decode(Encode(x)) == x in memory, and falls back to generic on any
// mismatch. It never returns a result it hasn't proven reversible.
type Registry interface {
    EncodeVerified(f types.File, src io.Reader, dst io.Writer) (types.Recipe, error)
    Decode(r types.Recipe, src io.Reader, dst io.Writer) error
}
```

This one only Koded consumes, but freeze it anyway so the DB schema for `recipes` doesn't move.

---

## Unblocking each other on day one

Three things ship in week one so neither track is ever stuck:

**Koded ships stub endpoints.** Every route he owns returns realistic fixture JSON immediately — fake jobs that count to 100 and finish, fake savings numbers, a fake timeline. Ridwan can build the entire frontend against a running server from day one.

**Ridwan ships auth + a `dev` login.** A `POST /api/auth/dev-login` behind a build tag that gives Koded a session cookie without a UI, so he can curl his own endpoints.

**Both ship fixtures.** `internal/testdata/` with real JPEGs and RAWs. Koded benchmarks against it; Ridwan seeds demo accounts from it.

After week one you could each go dark for a fortnight and both still have something to log.

---

## Koded — engine + archive API

### Week 1
- `types` and `codec` interfaces (with Ridwan)
- `generic` codec: zstd encode/decode
- `EncodeVerified` harness: encode, decode into a buffer, compare, fall back on mismatch
- Stub `archive`/`jobs`/`download` endpoints returning fixture JSON
- Round-trip test over testdata

The verify harness before the codecs. It's what makes everything after it safe.

### Week 2
- Content-addressed object store: `objects/ab/cd/abcd…`, temp-file-then-rename so a crash never leaves a half-written object
- BLAKE3 hashing, file-level dedup falls out for free
- Format sniffing from `File.Head` — magic bytes, not extensions

### Week 3
- Job runner: worker pool, `POST /archive` returns 202 immediately, progress written to the job row
- SSE endpoint streaming progress
- Real `POST /archive` and `GET /files/:id/download` end to end, verified on the way out

**This is the milestone.** End of week 3 someone can upload, archive, and get a byte-identical file back through the browser.

### Weeks 4–5 — JPEG
- `jpg-jxl` codec shelling out to `cjxl -j` / `djxl`
- Detect the binary at startup; if missing, `CanHandle → false` and everything still works
- Measure what fraction of real JPEGs round-trip clean

### Weeks 6–7 — RAW
- Walk the TIFF/IFD structure to find embedded full-res previews
- Extract, transcode through `jpg-jxl`, store offset/length in `Recipe.Blob` for exact reassembly
- Sensor path only if there's time. A measured negative result is a real result — put it in the table.

### Week 8
- Benchmark table with actual numbers
- Concurrency limits so ten uploads don't OOM the box

---

## Ridwan — auth, API, frontend

### Week 1
- Sessions: argon2id password hashing, HttpOnly + SameSite cookie, session table, auth middleware
- `signup` / `login` / `logout` / `me`
- Vite + React + TS + Tailwind scaffold, auth pages wired to real endpoints
- `dev-login` for Koded
- Test corpus in `internal/testdata/`

Roll the sessions yourself. It's a few hundred lines and you'll actually learn how auth works. Do not roll your own password hashing — use `golang.org/x/crypto/argon2`.

### Week 2
- Shoots CRUD, per-user ownership middleware
- Upload intake: multipart, size and count caps enforced server-side, files written to a per-user staging dir
- Library page: list shoots, create one

### Week 3
- Upload UI: drag and drop, per-file progress, cancel
- Wire archive to Koded's real endpoints, consume the SSE stream, live progress bar

### Weeks 4–5
- EXIF via `dsoprea/go-exif/v3` — RAW makernotes are a swamp, take what parses and move on
- Sequence detection: same camera, gaps under ~2s
- Shoot detail page: timeline, camera/lens/ISO summary, sequence groups, savings shown as **compression % and dedup % separately**

### Week 6
- File list with per-file codec and verified badge
- Download / restore flow with the verification result surfaced
- Empty states, error states, loading states — this is what makes it feel finished

### Week 7
- Landing page
- Responsive pass
- Delete a shoot, quota display, account settings

### Week 8
- Demo video, seeded demo account, README, deploy

---

## Security, briefly

It's a web app taking file uploads from strangers now, so:

- Never use the uploaded filename as a path. Store by ID, keep the original name in the DB only.
- Enforce size and count caps **server-side**, streaming — don't read the body into memory first.
- Every shoot and file query filters by the session's user ID. Test this deliberately: log in as A, request B's file, expect 404.
- Rate-limit login and signup.
- Serve uploads back with `Content-Disposition: attachment` and a fixed content type. Never inline.
- Secrets in env vars, not the repo.

---

## Integration points

| When | What |
|---|---|
| Week 1 | Agree the API shape and the `types` package. Merge. Freeze. |
| Week 3 | First real upload → archive → download through the browser. Expect breakage — that's why it's early. |
| Week 5 | Timeline data contract meets real EXIF output. |
| Week 8 | Benchmark numbers into the UI and README. |

Everything between those is independent.

---

## Weekly rhythm

1. Open a PR against `main` every week, even a small one. Never sit on a branch for a fortnight.
2. `go test ./...` and `npm run build` stay green. A broken `main` blocks the other person, which is the one thing this document exists to prevent.
3. Log the hours the same day you work them. The streak is 8 weeks of 10+ hours, and real work that never got logged still breaks it.

If a week goes badly, ship something small rather than nothing. The streak matters more than the feature.

---

## Definition of done

| End of week | Koded | Ridwan |
|---|---|---|
| 1 | verify harness, generic codec, stub API | auth + login/signup UI, test corpus |
| 2 | object store, dedup, sniffing | shoots CRUD, upload intake |
| 3 | **archive job + verified download working** | **upload UI with live progress** |
| 4 | JPEG codec | EXIF ingest |
| 5 | JPEG measured | timeline + sequences UI |
| 6 | RAW preview extraction | file list, restore flow, polish |
| 7 | RAW measured | landing page, responsive, settings |
| 8 | benchmarks, concurrency limits | demo, deploy, README |

Week 3 is the one that matters. If upload → archive → verified download works end to end by then, the remaining five weeks are all upside.
