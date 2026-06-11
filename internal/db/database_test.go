package db_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/db"
)

func TestCreateInitializesSchema(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	// When
	conn, err := db.Create(context.Background(), dbPath)

	// Then
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NotNil(t, conn)
}

func TestCreateReturnsCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	conn, err := db.Create(ctx, filepath.Join(t.TempDir(), "test.duckdb"))

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCreateReturnsConnectError(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "missing", "test.duckdb")

	// When
	conn, err := db.Create(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.Error(t, err)
}

func TestOpenExistingDatabase(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	db1, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	insertProjectRow(t, db1)
	require.NoError(t, db1.Close())

	// When
	db2, err := db.Open(context.Background(), dbPath)

	// Then
	require.NoError(t, err)
	defer func() { _ = db2.Close() }()
	require.NotNil(t, db2)
}

func TestOpenReturnsCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, context.Canceled)
}

func TestOpenMissingDatabaseDoesNotCreateFile(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "missing.duckdb")

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
	_, statErr := os.Stat(dbPath)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestOpenRejectsNonDeckCheckDatabaseWithoutMigrating(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "other.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec("CREATE TABLE notes (body VARCHAR)")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrNotDeckCheckProject)

	raw, err = sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var count int
	require.NoError(t, raw.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_name = 'schema_versions'
	`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, raw.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count))
}

func TestOpenRejectsDatabaseWithUnrelatedProjectTableWithoutMigrating(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "other.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec("CREATE TABLE project (external_id VARCHAR)")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrNotDeckCheckProject)

	raw, err = sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var count int
	require.NoError(t, raw.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_name = 'schema_versions'
	`).Scan(&count))
	require.Zero(t, count)
}

func TestOpenRejectsDeckCheckSchemaWithoutProjectRowWithoutMigrating(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "projectless.duckdb")
	raw, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	_, err = raw.Exec("DELETE FROM schema_versions WHERE version = ?", db.CurrentSchemaVersion)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrNotDeckCheckProject)

	raw, err = sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var count int
	require.NoError(t, raw.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = ?", db.CurrentSchemaVersion).Scan(&count))
	require.Zero(t, count)
}

func TestOpenRejectsUnrelatedFutureVersionMarker(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "future-marker.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE schema_versions (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = raw.Exec("INSERT INTO schema_versions (version) VALUES (?)", db.CurrentSchemaVersion+1)
	require.NoError(t, err)
	_, err = raw.Exec("CREATE TABLE project (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrNotDeckCheckProject)
}

func TestOpenReturnsTooNewForFutureDeckCheckProjectBeforeFullSchemaShape(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "future.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE schema_versions (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = raw.Exec("INSERT INTO schema_versions (version) VALUES (?)", db.CurrentSchemaVersion+1)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE project (
		id INTEGER PRIMARY KEY,
		name VARCHAR NOT NULL,
		dataset_type VARCHAR NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = raw.Exec("INSERT INTO project (id, name, dataset_type) VALUES (1, 'Future', 'csv')")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrSchemaTooNew)
}

func TestOpenRejectsProjectIdentityWithoutDeckCheckSchemaWithoutMigrating(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "partial.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE schema_versions (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = raw.Exec("INSERT INTO schema_versions (version) VALUES (1)")
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE project (
		id INTEGER PRIMARY KEY,
		name VARCHAR NOT NULL,
		dataset_type VARCHAR NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = raw.Exec("INSERT INTO project (id, name, dataset_type) VALUES (1, 'Partial', 'csv')")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	conn, err := db.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, conn)
	require.ErrorIs(t, err, db.ErrNotDeckCheckProject)

	raw, err = sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var count int
	require.NoError(t, raw.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = ?", db.CurrentSchemaVersion).Scan(&count))
	require.Zero(t, count)
}

func TestCreateMakesAllExpectedTables(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	// When
	conn, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Then
	tables := []string{"project", "questions", "answers", "dataset_rows", "classifications", "schema_versions"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var count int
			err := conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
			require.NoError(t, err, "table %s", table)
		})
	}
}

func TestCreateRecordsSchemaVersion(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	// When
	conn, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Then
	var version int
	require.NoError(t, conn.QueryRow("SELECT MAX(version) FROM schema_versions").Scan(&version))
	require.Equal(t, db.CurrentSchemaVersion, version)
}

func TestMigrateIsIdempotent(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	conn1, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	insertProjectRow(t, conn1)
	require.NoError(t, conn1.Close())

	// When
	conn2, err := db.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = conn2.Close() }()

	// Then
	var count int
	require.NoError(t, conn2.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&count))
	require.Equal(t, db.CurrentSchemaVersion, count)
}

// TestOpenMigratesOlderProjectFilesForward pins the product's "open
// old project files" promise: a genuine v1 file (only migration 0001
// applied, no singleton guard) is migrated to the current schema on
// open, with the existing project row backfilled and its data intact.
func TestOpenMigratesOlderProjectFilesForward(t *testing.T) {
	// Given a v1 project file, built from the real 0001 migration.
	dbPath := filepath.Join(t.TempDir(), "legacy.duckdb")

	initial, err := os.ReadFile(filepath.Join("migrations", "0001_initial.sql"))
	require.NoError(t, err)

	conn, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	for _, statement := range []string{
		`CREATE TABLE schema_versions (
			version    INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		string(initial),
		"INSERT INTO schema_versions (version) VALUES (1)",
		"INSERT INTO project (name, dataset_type) VALUES ('Test', 'csv')",
	} {
		_, execErr := conn.Exec(statement)
		require.NoError(t, execErr)
	}
	require.NoError(t, conn.Close())

	// When
	migrated, err := db.Open(context.Background(), dbPath)

	// Then every pending migration was applied and recorded...
	require.NoError(t, err)
	defer func() { _ = migrated.Close() }()

	var versions int
	require.NoError(t, migrated.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&versions))
	require.Equal(t, db.CurrentSchemaVersion, versions)

	// ...the 0003 backfill ran against the existing row...
	var guard int
	require.NoError(t, migrated.QueryRow("SELECT singleton_guard FROM project").Scan(&guard))
	require.Equal(t, 1, guard)

	// ...and the original project data survived the migration.
	var name string
	require.NoError(t, migrated.QueryRow("SELECT name FROM project").Scan(&name))
	require.Equal(t, "Test", name)
}

func TestOpenReportsFailedMigrationWithoutRecordingVersion(t *testing.T) {
	// Given a project file missing the 0002 version record, with a
	// conflicting table that makes re-applying 0002 fail mid-body.
	dbPath := filepath.Join(t.TempDir(), "broken.duckdb")

	conn, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	insertProjectRow(t, conn)
	require.NoError(t, conn.Close())

	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec("DELETE FROM schema_versions WHERE version = 2")
	require.NoError(t, err)
	_, err = raw.Exec("CREATE TABLE classifications_v2 (conflict_marker INTEGER)")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	_, err = db.Open(context.Background(), dbPath)

	// Then the failure names the migration that broke...
	require.ErrorContains(t, err, "apply 0002_dataset_row_identity")

	// ...and no partial version record was left behind.
	raw, err = sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var recorded int
	require.NoError(t, raw.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = 2").Scan(&recorded))
	require.Zero(t, recorded)
}

func insertProjectRow(t *testing.T, conn *sql.DB) {
	t.Helper()

	_, err := conn.Exec("INSERT INTO project (name, dataset_type, singleton_guard) VALUES (?, ?, 1)", "Test", "csv")
	require.NoError(t, err)
}
