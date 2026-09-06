package authapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Koded0214h/relic/backend/internal/auth"
	"github.com/Koded0214h/relic/backend/internal/httpx"
	"github.com/Koded0214h/relic/backend/internal/user"
)

var (
	sqlDB        *sql.DB
	cookieSecure bool
)

// Init wires the shared dependencies. Called once from main before Mount.
func Init(db *sql.DB, secure bool) {
	sqlDB, cookieSecure = db, secure
}

func Mount(r chi.Router) {
	r.Post("/auth/signup", signup)
	r.Post("/auth/login", login)
	r.Post("/auth/logout", logout)
	r.With(auth.Middleware(sqlDB)).Get("/me", me)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func signup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}

	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if !strings.Contains(c.Email, "@") || len(c.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "invalid_input", "email must be valid and password at least 8 characters")
		return
	}

	u, err := user.Create(sqlDB, c.Email, c.Password)
	if err != nil {
		if err == user.ErrEmailTaken {
			httpx.Error(w, http.StatusConflict, "email_taken", "email already registered")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not create account")
		return
	}

	sessionID, expires, err := auth.CreateSession(sqlDB, u.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not start session")
		return
	}
	auth.SetCookie(w, sessionID, expires, cookieSecure)
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": u.ID, "email": u.Email})
}

func login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}

	u, err := user.Authenticate(sqlDB, strings.ToLower(strings.TrimSpace(c.Email)), c.Password)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	sessionID, expires, err := auth.CreateSession(sqlDB, u.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "could not start session")
		return
	}
	auth.SetCookie(w, sessionID, expires, cookieSecure)
	httpx.JSON(w, http.StatusOK, map[string]string{"id": u.ID, "email": u.Email})
}

func logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		_ = auth.DeleteSession(sqlDB, cookie.Value)
	}
	auth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	u, err := user.Get(sqlDB, userID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"id": u.ID, "email": u.Email})
}
