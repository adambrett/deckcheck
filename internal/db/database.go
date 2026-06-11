package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/marcboeker/go-duckdb" // register the DuckDB driver with database/sql
)

// Open returns a connection to an existing DeckCheck project database.
// Pending migrations are applied; if the file is at a newer schema
// version than this binary supports, the file is not modified and
// [ErrSchemaTooNew] is returned.
//
// The structural inspection runs over a read-only connection: a
// read-write open can replay a foreign database's WAL and mutate the
// file before the markers are ever checked, so rejected files must
// never see one.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := inspectReadOnly(ctx, path); err != nil {
		return nil, err
	}

	conn, err := connect(path)
	if err != nil {
		return nil, err
	}

	if err := migrate(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

// inspectReadOnly verifies path is an acceptable DeckCheck project
// over a read-only connection, returning the rejection error if not.
func inspectReadOnly(ctx context.Context, path string) error {
	conn, err := connectReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	version, ok, err := inspectDeckCheckDatabase(ctx, conn)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotDeckCheckProject
	}
	if version > CurrentSchemaVersion {
		return fmt.Errorf("%w: file is at v%d, binary supports v%d", ErrSchemaTooNew, version, CurrentSchemaVersion)
	}

	return nil
}

// Create opens (or creates) a DeckCheck project database at path and
// applies every migration in order, so the resulting connection is on
// the current schema version.
func Create(ctx context.Context, path string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := connect(path)
	if err != nil {
		return nil, err
	}

	if err := migrate(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("initialise schema: %w", err)
	}

	return conn, nil
}

func connect(path string) (*sql.DB, error) {
	conn, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return conn, nil
}

func connectReadOnly(path string) (*sql.DB, error) {
	conn, err := sql.Open("duckdb", path+"?access_mode=read_only")
	if err != nil {
		return nil, fmt.Errorf("open database read-only: %w", err)
	}
	return conn, nil
}

func inspectDeckCheckDatabase(ctx context.Context, conn *sql.DB) (int, bool, error) {
	ok, err := hasDeckCheckMarkers(ctx, conn)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}

	ok, err = hasDeckCheckProjectIdentity(ctx, conn)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return 0, false, err
	}
	version := maxVersion(applied)
	if version > CurrentSchemaVersion {
		return version, true, nil
	}

	ok, err = hasKnownDeckCheckSchema(ctx, conn)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}

	return version, true, nil
}

// schemaTables and projectColumns enumerate the structural fingerprint a
// healthy DeckCheck file must carry. The inspection helpers below check
// fingerprints without ever mutating the file, so a foreign DuckDB
// database can be rejected untouched.
var (
	markerTables = []string{"schema_versions", "project"}

	identityColumns = []string{"id", "name", "dataset_type", "created_at"}

	schemaTables = []string{
		"schema_versions", "project", "questions", "answers", "dataset_rows", "classifications",
	}

	projectColumns = []string{
		"id", "name", "dataset_type", "image_column", "data_columns", "created_at",
	}
)

func hasDeckCheckMarkers(ctx context.Context, conn *sql.DB) (bool, error) {
	return hasAllNames(ctx, conn, tablesQuery(markerTables), markerTables, "database markers")
}

func hasDeckCheckProjectIdentity(ctx context.Context, conn *sql.DB) (bool, error) {
	ok, err := hasAllNames(ctx, conn, columnsQuery(identityColumns), identityColumns, "project identity columns")
	if err != nil || !ok {
		return false, err
	}

	var projectRows int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM project").Scan(&projectRows); err != nil {
		return false, fmt.Errorf("count project rows: %w", err)
	}

	return projectRows == 1, nil
}

func hasKnownDeckCheckSchema(ctx context.Context, conn *sql.DB) (bool, error) {
	ok, err := hasAllNames(ctx, conn, tablesQuery(schemaTables), schemaTables, "database tables")
	if err != nil || !ok {
		return false, err
	}

	return hasAllNames(ctx, conn, columnsQuery(projectColumns), projectColumns, "project columns")
}

func tablesQuery(tables []string) string {
	return `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'main'
		AND table_name IN (` + quoteList(tables) + `)
	`
}

func columnsQuery(columns []string) string {
	return `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'main'
		AND table_name = 'project'
		AND column_name IN (` + quoteList(columns) + `)
	`
}

// quoteList renders names as a quoted SQL IN-list. Inputs are the
// package-internal identifier constants above, never user data.
func quoteList(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + name + "'"
	}

	return strings.Join(quoted, ", ")
}

// hasAllNames runs query and reports whether every name in required is
// present in the result set. label gives error messages their context.
func hasAllNames(ctx context.Context, conn *sql.DB, query string, required []string, label string) (bool, error) {
	remaining := make(map[string]struct{}, len(required))
	for _, name := range required {
		remaining[name] = struct{}{}
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", label, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, fmt.Errorf("scan %s: %w", label, err)
		}
		delete(remaining, name)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate %s: %w", label, err)
	}

	return len(remaining) == 0, nil
}
