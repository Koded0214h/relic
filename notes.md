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
