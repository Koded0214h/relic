package server

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Koded0214h/relic/backend/internal/api/archive"
	authapi "github.com/Koded0214h/relic/backend/internal/api/auth"
	"github.com/Koded0214h/relic/backend/internal/api/files"
	"github.com/Koded0214h/relic/backend/internal/api/shoots"
	"github.com/Koded0214h/relic/backend/internal/auth"
	"github.com/Koded0214h/relic/backend/internal/config"
	"github.com/Koded0214h/relic/backend/internal/httpx"
)

type Server struct {
	cfg    config.Config
	Router chi.Router
}

// New builds the HTTP router. The api packages must already have had Init
// called (from main) — this only mounts their routes. database backs the
// session-auth middleware that guards the private routes.
func New(cfg config.Config, database *sql.DB) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	s := &Server{cfg: cfg, Router: r}

	r.Get("/healthz", s.health)
	r.Route("/api", func(r chi.Router) {
		authapi.Mount(r) // signup/login are public; /me is guarded inside authapi itself

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(database))
			shoots.Mount(r)
			archive.Mount(r)
			files.Mount(r)
		})
	})

	return s
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"env":    s.cfg.Env,
	})
}
