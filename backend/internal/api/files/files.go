package files

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router) {
	r.Get("/files/{fileID}/download", download)
}

func download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "fileID")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.bin"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write([]byte("fixture file contents for " +id))
}