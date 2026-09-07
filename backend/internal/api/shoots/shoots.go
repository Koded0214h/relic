package shoots

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"database/sql"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Koded0214h/relic/backend/internal/auth"
	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/shoot"
)

var (
	sqlDB      *sql.DB
	stagingDir string
)

func Init(db *sql.DB, dataDir string) {
	sqlDB = db
	stagingDir = filepath.Join(dataDir, "staging")
	_ = os.MkdirAll(stagingDir, 0o755)
}

func Mount(r chi.Router) {
	r.Get("/shoots", list)
	r.Post("/shoots", create)
	r.Get("/shoots/{shootID}", get)
	r.Delete("/shoots/{shootID}", remove)
	r.Post("/shoots/{shootID}/files", upload)
}

func list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)

	summaries, err := shoot.ListForUser(sqlDB, userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not list shoots")
		return
	}

	out := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		m := map[string]any{"id": s.ID, "name": s.Name, "file_count": s.FileCount, "total_size": s.TotalSize}
		if s.ArchivedAt.Valid {
			m["archived_at"] = s.ArchivedAt.Time
		}
		out = append(out, m)
	}
	httpx.JSON(w, http.StatusOK, out)
}

func create(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}

	s, err := shoot.Create(sqlDB, userID, strings.TrimSpace(body.Name))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not create shoot")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": s.ID, "name": s.Name})
}

func get(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)
	shootID := chi.URLParam(r, "shootID")

	s, err := shoot.Get(sqlDB, shootID, userID)
	if errors.Is(err, shoot.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "shoot not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not load shoot")
		return
	}

	files, err := shoot.ListFiles(sqlDB, s.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not load shoot")
		return
	}

	resp := map[string]any{"id": s.ID, "name": s.Name, "files": files}
	if s.ArchivedAt.Valid {
		resp["archived_at"] = s.ArchivedAt.Time
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func remove(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)
	shootID := chi.URLParam(r, "shootID")

	err := shoot.Delete(sqlDB, shootID, userID)
	if errors.Is(err, shoot.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "shoot not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not delete shoot")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func upload(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r)
	shootID := chi.URLParam(r, "shootID")

	s, err := shoot.Get(sqlDB, shootID, userID)
	if errors.Is(err, shoot.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "shoot not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not load shoot")
		return
	}

	count, total, err := shoot.Stats(sqlDB, s.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not read shoot state")
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "expected multipart/form-data")
		return
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not prepare storage")
		return
	}

	var uploaded []shoot.File
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "bad_request", "malformed upload")
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			part.Close()
			continue
		}
		if count >= shoot.MaxShootFiles {
			part.Close()
			httpx.Error(w, http.StatusRequestEntityTooLarge, "quota_exceeded",
				fmt.Sprintf("shoot is limited to %d files", shoot.MaxShootFiles))
			return
		}

		name := safeName(part.FileName())
		dst, size, err := writeCapped(stagingDir, name, part, shoot.MaxShootBytes-total)
		part.Close()
		if errors.Is(err, errQuota) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "quota_exceeded",
				fmt.Sprintf("shoot is limited to %d bytes", shoot.MaxShootBytes))
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "server_error", "could not store upload")
			return
		}

		sf, err := shoot.AddFile(sqlDB, s.ID, name, size, dst)
		if err != nil {
			os.Remove(dst)
			httpx.Error(w, http.StatusInternalServerError, "server_error", "could not record upload")
			return
		}
		uploaded = append(uploaded, sf)
		count++
		total += size
	}

	httpx.JSON(w, http.StatusCreated, uploaded)
}

var errQuota = errors.New("shoot: quota exceeded mid-write")

// writeCapped streams part to a new file under dir, refusing (and cleaning
// up) once it would write more than remaining bytes. The cap is enforced
// while copying, not after — a client can't blow the quota by lying about
// Content-Length. The staging file keeps the original extension so the
// codec registry can still dispatch on it at archive time.
func writeCapped(dir, name string, part *multipart.Part, remaining int64) (path string, size int64, err error) {
	if remaining <= 0 {
		return "", 0, errQuota
	}

	dst := filepath.Join(dir, uuid.NewString()+strings.ToLower(filepath.Ext(name)))
	out, err := os.Create(dst)
	if err != nil {
		return "", 0, err
	}

	n, err := io.Copy(out, io.LimitReader(part, remaining+1))
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		return "", 0, err
	}
	if n > remaining {
		os.Remove(dst)
		return "", 0, errQuota
	}
	return dst, n, nil
}

func safeName(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "unnamed"
	}
	return name
}
