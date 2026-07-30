package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lingshu/lingshu/pkg/logger"
)

// migrationSearchPaths lists directories to search for migration SQL files.
var migrationSearchPaths = []string{
	"migrations",
	"../migrations",
	"../../migrations",
	filepath.Join("pkg", "db", "migrations"),
}

// runMigrations applies any pending database migrations from the migrations directory.
func (d *Database) runMigrations() error {
	db := d.DB()
	if db == nil {
		return fmt.Errorf("no database connection available for migrations")
	}

	// SQLite fallback does not support PostgreSQL-specific DDL (extensions, PL/pgSQL, triggers, JSONB, etc.).
	// The application already handles missing tables gracefully via file-based fallbacks.
	if d.IsFallback() {
		logger.Info("SQLite fallback active, skipping PostgreSQL migrations (file-based storage will be used)")
		return nil
	}

	// Find migration directory
	migrationDir := findMigrationDir()
	if migrationDir == "" {
		logger.Warn("Migration directory not found, skipping auto-migration")
		return nil
	}

	// Create migration tracking table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		logger.Warn("Failed to create migration tracking table", "error", err)
		return nil
	}

	// Read up migration files
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		logger.Warn("Cannot read migration directory", "dir", migrationDir, "error", err)
		return nil
	}

	var versions []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			versions = append(versions, name)
		}
	}
	sort.Strings(versions)

	for _, version := range versions {
		// Check if already applied
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version)
		if err != nil {
			_ = db.Get(&count, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version)
		}
		if count > 0 {
			continue
		}

		// Read migration file
		content, err := os.ReadFile(filepath.Join(migrationDir, version))
		if err != nil {
			logger.Warn("Failed to read migration file", "version", version, "error", err)
			continue
		}

		logger.Info("Applying migration", "version", version)
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction for %s: %w", version, err)
		}

		// Execute each statement separately (split on semicolons)
		for _, stmt := range splitSQLStatements(string(content)) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				logger.Warn("Migration statement failed", "version", version, "error", err)
				return fmt.Errorf("migration %s failed: %w", version, err)
			}
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_, _ = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}
		logger.Info("Migration applied successfully", "version", version)
	}

	return nil
}

// findMigrationDir searches known paths for the migrations directory.
func findMigrationDir() string {
	for _, p := range migrationSearchPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

// splitSQLStatements splits SQL text into individual statements by semicolons,
// ignoring semicolons inside quotes, dollar-quoted strings ($$...$$), or comments.
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inDollarQuote := false
	dollarTag := "" // the tag between $ markers, e.g. "func$" or empty for $$
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		next := byte(0)
		if i+1 < len(sql) {
			next = sql[i+1]
		}

		// Handle dollar-quoted strings (PostgreSQL $$...$$ or $tag$...$tag$)
		// Only check when not already inside single/double quotes or comments
		if !inSingleQuote && !inDoubleQuote && !inLineComment && !inBlockComment {
			if ch == '$' && !inDollarQuote {
				// Check if this is a dollar-quote start: $tag$ or just $$
				// Scan forward to find the matching closing $
				j := i + 1
				tag := ""
				for j < len(sql) && sql[j] != '$' {
					tag += string(sql[j])
					j++
				}
				if j < len(sql) {
					// Found closing $, this is a dollar quote
					inDollarQuote = true
					dollarTag = tag
					current.WriteString("$" + tag + "$")
					i = j
					continue
				}
			} else if ch == '$' && inDollarQuote {
				// Potentially closing a dollar quote
				// Check if it matches: $tag$ (where tag matches the opening tag)
				j := i + 1
				tag := ""
				for j < len(sql) && sql[j] != '$' {
					tag += string(sql[j])
					j++
				}
				if j < len(sql) && tag == dollarTag {
					inDollarQuote = false
					dollarTag = ""
					current.WriteString("$" + tag + "$")
					i = j
					continue
				}
			}
		}

		// Handle comments (only outside quotes)
		if !inSingleQuote && !inDoubleQuote && !inDollarQuote {
			if !inBlockComment && ch == '-' && next == '-' {
				inLineComment = true
				current.WriteByte(ch)
				continue
			}
			if inLineComment && ch == '\n' {
				inLineComment = false
				current.WriteByte(ch)
				continue
			}
			if inLineComment {
				current.WriteByte(ch)
				continue
			}
			if !inBlockComment && ch == '/' && next == '*' {
				inBlockComment = true
				current.WriteByte(ch)
				i++
				current.WriteByte(next)
				continue
			}
			if inBlockComment && ch == '*' && next == '/' {
				inBlockComment = false
				current.WriteByte(ch)
				i++
				current.WriteByte(next)
				continue
			}
			if inBlockComment {
				current.WriteByte(ch)
				continue
			}
		}

		// Handle single/double quotes
		if ch == '\'' && !inDoubleQuote && !inDollarQuote {
			inSingleQuote = !inSingleQuote
		}
		if ch == '"' && !inSingleQuote && !inDollarQuote {
			inDoubleQuote = !inDoubleQuote
		}

		if ch == ';' && !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			statements = append(statements, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}

	// Last statement (may not end with semicolon)
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}
