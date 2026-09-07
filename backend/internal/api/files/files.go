package files

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"

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
		httpx.Error(w, http.StatusNotFound, "not_found", "file not found")
		return
	}

	encoded, err := objStore.Get(af.Hash)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "store_error", "archived object missing")
		return
	}
	defer encoded.Close()

	// Restore into a temp file, never straight to the client: if decode
	// fails or the stored object is corrupt we still owe a clean error
	// response, not a truncated 200. This is what makes the integrity
	// promise real.
	tmp, err := os.CreateTemp("", "relic-restore-*")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "restore failed")
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// af.Hash is the hash of the *encoded* bytes in the object store, so
	// hashing the stream as we read it proves the store still holds
	// exactly what archive wrote. The decoded output itself was already
	// checked at archive time by EncodeVerified (full encode->decode->
	// compare); a hash of the *original* upload isn't stored yet, so we
	// can't re-verify that side here.
	h := sha256.New()
	if err := registry.Decode(af.Recipe, io.TeeReader(encoded, h), tmp); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "decode_error", "restore failed")
		return
	}
	if hex.EncodeToString(h.Sum(nil)) != af.Hash {
		httpx.Error(w, http.StatusInternalServerError, "integrity_error", "stored object failed verification")
		return
	}

	size, err := tmp.Seek(0, io.SeekEnd)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "restore failed")
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "restore failed")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(af.Path)+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	_, _ = io.Copy(w, tmp)
}
