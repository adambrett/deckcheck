package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// CurrentSchemaVersion is the highest migration version the binary knows
// about. A database that records a version greater than this is from a
// newer DeckCheck and we refuse to open it (rather than risk silent
// corruption); a database that records a lower version is migrated
// forward on open.
const CurrentSchemaVersion = 5

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration describes a single SQL file under migrations/.
type migration struct {
	version int
	name    string
	body    string
}

// loadMigrations returns every embedded migration sorted by version.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("migration %s: %w", entry.Name(), err)
		}

		body, err := fs.ReadFile(migrationsFS, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			body:    string(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

// parseMigrationVersion extracts the leading integer from a migration
// filename like "0001_initial.sql" → 1.
func parseMigrationVersion(name string) (int, error) {
	prefix := name
	if idx := strings.IndexAny(name, "_."); idx >= 0 {
		prefix = name[:idx]
	}
	v, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("unparseable version prefix %q: %w", prefix, err)
	}
	return v, nil
}

// migrate applies every embedded migration that has not yet been recorded
// against db, in version order. It returns [ErrSchemaTooNew] if db is at a
// schema version newer than this binary supports.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_versions (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_versions: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	if highest := maxVersion(applied); highest > CurrentSchemaVersion {
		return fmt.Errorf("%w: file is at v%d, binary supports v%d", ErrSchemaTooNew, highest, CurrentSchemaVersion)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
	}

	return nil
}

// appliedVersions reads schema_versions and returns the set of recorded
// migration versions.
func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_versions")
	if err != nil {
		return nil, fmt.Errorf("query schema_versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema_versions: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_versions: %w", err)
	}
	return applied, nil
}

// maxVersion returns the highest key in applied, or 0 if empty.
func maxVersion(applied map[int]bool) int {
	highest := 0
	for v := range applied {
		if v > highest {
			highest = v
		}
	}
	return highest
}

// applyMigration runs a single migration's SQL inside a transaction and
// records its version in schema_versions on success.
func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.body); err != nil {
		return fmt.Errorf("exec migration body: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_versions (version) VALUES (?)", m.version); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
