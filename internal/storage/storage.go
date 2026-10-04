package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"reactoropt/internal/model"
)

type Store struct{ DB *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    objective TEXT NOT NULL,
    direction TEXT NOT NULL CHECK(direction IN ('min', 'max')),
    variables_json TEXT NOT NULL,
    parameters_json TEXT NOT NULL,
    constraints_json TEXT NOT NULL,
    derived_metrics_json TEXT NOT NULL,
    is_builtin INTEGER NOT NULL CHECK(is_builtin IN (0, 1)),
    objective_label TEXT NOT NULL,
    objective_unit TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS methods (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK(enabled IN (0, 1)),
    defaults_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('user', 'admin')),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    session_version INTEGER NOT NULL DEFAULT 0
);
`

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("Не указан путь к базе данных")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	success := false
	defer func() {
		if !success {
			database.Close()
		}
	}()
	if _, err = database.Exec(`
        PRAGMA busy_timeout = 5000;
        PRAGMA foreign_keys = ON;
        PRAGMA journal_mode = WAL;
    `); err != nil {
		return nil, err
	}
	transaction, err := database.Begin()
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	if _, err = transaction.Exec(schema); err != nil {
		return nil, err
	}
	for _, task := range model.DefaultTasks() {
		values, err := taskValues(task)
		if err != nil {
			return nil, err
		}
		if _, err = transaction.Exec(strings.Replace(insertTask, "INSERT INTO", "INSERT OR IGNORE INTO", 1), values...); err != nil {
			return nil, err
		}
	}
	if err = transaction.Commit(); err != nil {
		return nil, err
	}
	success = true
	return &Store{DB: database}, nil
}

func (store *Store) Close() error { return store.DB.Close() }
