package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func DefaultPath() string {
	if p := os.Getenv("TODO_DB"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "todo", "todo.db")
}

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// Incremental migrations (safe to re-run)
	for _, m := range migrations {
		db.Exec(m) // ignore "duplicate column" errors
	}
	return db, nil
}

var migrations = []string{
	"ALTER TABLE tasks ADD COLUMN due_date TEXT",
	"ALTER TABLE tasks ADD COLUMN inbox_position INTEGER NOT NULL DEFAULT 0",
	"ALTER TABLE tasks ADD COLUMN notes TEXT NOT NULL DEFAULT ''",
	"ALTER TABLE tasks ADD COLUMN requested_by TEXT NOT NULL DEFAULT ''",
}
