package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory		= 64 * 1024
	iterations	= 3
	parallelism = 4
	saltLen		= 16
	keyLen		= 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)

	if _, err := rand.Read(salt); err != nil { return "", err } 
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)

	return fmt.Sprintf("argon2id$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" { return false, fmt.Errorf("auth: unrecognized hash format") }

	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil { return false, err }
	want, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil { return false, err}

	got:= argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}