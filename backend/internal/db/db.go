package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects to the SQLite database at path, creating the file if it
// doesn't exist. foreign_keys is enabled per-connection via the DSN so it
// applies to every connection database/sql pools.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return db, nil
}

// Migrate applies every embedded migrations/*.sql file in name order. The
// files use CREATE ... IF NOT EXISTS, so it's safe to run on every boot.
func Migrate(db *sql.DB) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		b, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db: read %s: %w", name, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("db: migrate %s: %w", name, err)
		}
	}
	return nil
}
