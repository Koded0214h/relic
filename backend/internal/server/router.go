package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Koded0214h/relic/backend/internal/config"
)

type Server struct {
	cfg		config.Config
	Router	chi.Router
}

func New(cfg config.Config) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string {cfg.CORSOrigin},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge: 300,
	}))

	s:= &Server{cfg: cfg, Router: r}

	r.Get("/healthz", s.health)
	r.Route("/api", func(r chi.Router) {
		// auth.Mount(r)     — Ridwan
		// shoots.Mount(r)   — Ridwan
		// archive.Mount(r)  — you
		// files.Mount(r)    — you
	})

	return s
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string {
		"status": "ok",
		"env": s.cfg.Env,
	})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, map[string]string{"error": msg, code: code})
}