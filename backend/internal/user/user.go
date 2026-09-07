package user

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/Koded0214h/relic/backend/internal/auth"
)

var (
	ErrEmailTaken         = errors.New("user: email already registered")
	ErrInvalidCredentials = errors.New("user: invalid email or password")
)

type User struct {
	ID    string
	Email string
}

func Create(db *sql.DB, email, password string) (User, error) {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&exists); err != nil {
		return User{}, err
	}
	if exists > 0 {
		return User{}, ErrEmailTaken
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return User{}, err
	}

	id := uuid.NewString()
	if _, err := db.Exec(`INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)`, id, email, hash); err != nil {
		return User{}, err
	}
	return User{ID: id, Email: email}, nil
}

func Authenticate(db *sql.DB, email, password string) (User, error) {
	var u User
	var hash string
	err := db.QueryRow(
		`SELECT id, email, password_hash FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &hash)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	ok, err := auth.VerifyPassword(password, hash)
	if err != nil || !ok {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
}

func Get(db *sql.DB, id string) (User, error) {
	var u User
	err := db.QueryRow(`SELECT id, email FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Email)
	return u, err
}
