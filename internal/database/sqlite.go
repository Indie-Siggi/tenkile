// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLite represents the SQLite database connection
type SQLite struct {
	conn *sql.DB
}

// OpenSQLite opens a new SQLite database connection
func OpenSQLite(dsn string) (*SQLite, error) {
	// Create data directory if it doesn't exist
	if idx := strings.LastIndex(dsn, "/"); idx != -1 {
		dir := dsn[:idx]
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode = WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	return &SQLite{conn: conn}, nil
}

// RunMigrations executes all pending migrations
func (s *SQLite) RunMigrations() error {
	// Create migrations table if it doesn't exist
	_, err := s.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			dirty BOOLEAN DEFAULT FALSE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := s.getAppliedMigrations()
	if err != nil {
		return err
	}

	// Read migration files
	migrations, err := readMigrations()
	if err != nil {
		return err
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}

		tx, err := s.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(migration.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration.Version, err)
		}
	}

	return nil
}

// getAppliedMigrations returns a map of applied migration versions
func (s *SQLite) getAppliedMigrations() (map[string]bool, error) {
	rows, err := s.conn.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// Migration represents a database migration
type Migration struct {
	Version string
	SQL     string
}

// readMigrations reads all migration files from the embedded filesystem
func readMigrations() ([]Migration, error) {
	var migrations []Migration

	err := fs.WalkDir(migrationsFS, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := migrationsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", path, err)
		}

		// Extract version from filename (e.g., 20260322120000_init.sql)
		version := strings.TrimSuffix(filepath.Base(path), ".sql")

		migrations = append(migrations, Migration{
			Version: version,
			SQL:     string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Close closes the database connection
func (s *SQLite) Close() error {
	return s.conn.Close()
}

// DB returns the underlying sql.DB connection
func (s *SQLite) DB() *sql.DB {
	return s.conn
}
