package files

import (
	"database/sql"
	"net/http"
	"path"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/auth"
	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/db"
	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/shoot"
	"github.com/Koded0214h/relic/backend/internal/store"
)

var (
	sqlDB    *sql.DB
	objStore *store.Store
	registry codec.Registry
)

func Init(database *sql.DB, s *store.Store, r codec.Registry) {
	sqlDB, objStore, registry = database, s, r
}

func Mount(r chi.Router) {
	r.Get("/files/{fileID}/download", download)
}

func download(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)
	id := chi.URLParam(r, "fileID")

	af, err := db.GetArchivedFile(sqlDB, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "file not found")
		return
	}

	if _, err := shoot.Get(sqlDB, af.ShootID, userID); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found","file not found")
		return
	}

	encoded, err := objStore.Get(af.Hash)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "store_error", "archived object missing")
		return
	}
	defer encoded.Close()

	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(af.Path)+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")

	if err := registry.Decode(af.Recipe, encoded, w); err != nil {
		// KNOWN ROUGH EDGE: Decode streams straight to w, so a failure
		// after the first flush leaves the client with a truncated body
		// under a 200. The integrity-model fix is to decode into a
		// buffer, re-hash, and only then write w. Deferred — tracked in
		// notes.md.
		httpx.Error(w, http.StatusInternalServerError, "decode_error", "restore failed")
		return
	}
}
