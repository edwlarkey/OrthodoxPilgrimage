package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// New opens a connection to the database. The DSN should be properly formatted.
func New(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// MigrateUp applies all up migrations from the embedded filesystem.
// It is now idempotent, tracking applied migrations in a 'schema_migrations' table.
func MigrateUp(db *sql.DB) error {
	// 1. Ensure the schema_migrations table exists.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY);`)
	if err != nil {
		return fmt.Errorf("could not create schema_migrations table: %w", err)
	}

	// 2. Get the list of already applied migrations.
	rows, err := db.Query("SELECT version FROM schema_migrations;")
	if err != nil {
		return fmt.Errorf("could not query schema_migrations: %w", err)
	}
	defer rows.Close()

	appliedMigrations := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("could not scan migration version: %w", err)
		}
		appliedMigrations[version] = true
	}

	// 3. Get the list of migration files from the embedded FS.
	migrationFiles, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("could not read migrations directory: %w", err)
	}

	// 4. Sort the files by name to ensure they are run in order.
	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].Name() < migrationFiles[j].Name()
	})

	// 5. Apply migrations that haven't been applied yet.
	for _, file := range migrationFiles {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		if appliedMigrations[fileName] {
			continue // Skip already applied migration
		}

		content, err := migrationsFS.ReadFile("migrations/" + fileName)
		if err != nil {
			return fmt.Errorf("could not read migration file %s: %w", fileName, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("could not begin transaction for migration %s: %w", fileName, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", fileName); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", fileName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("could not commit transaction for migration %s: %w", fileName, err)
		}
	}

	return nil
}
