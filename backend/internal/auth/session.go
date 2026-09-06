package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const (
	CookieName = "relic_session"
	sessionTTL = 30 * 24 * time.Hour
)

var ErrNoSession = errors.New("auth: no valid session")

type ctxKey int

const userIDKey ctxKey = 0

func CreateSession(db *sql.DB, userID string) (id string, expires time.Time, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	id = hex.EncodeToString(raw)
	expires = time.Now().Add(sessionTTL)

	_, err = db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		id, userID, expires,
	)
	return id, expires, err
}

func UserIDFromSession(db *sql.DB, sessionID string) (string, error) {
	var userID string
	var expiresAt time.Time
	err := db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE id = ?`, sessionID,
	).Scan(&userID, &expiresAt)
	if err != nil {
		return "", ErrNoSession
	}
	if time.Now().After(expiresAt) {
		return "", ErrNoSession
	}
	return userID, nil
}

func DeleteSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

func SetCookie(w http.ResponseWriter, sessionID string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// Middleware rejects requests without a valid session cookie; on success it
// stashes the user id in the request context for UserID to read.
func Middleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				http.Error(w, `{"error":"not authenticated","code":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			userID, err := UserIDFromSession(db, cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"session expired","code":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(userIDKey).(string)
	return id, ok
}
