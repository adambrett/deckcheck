package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newSchemaVersionsDB(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := connect(filepath.Join(t.TempDir(), "test.duckdb"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec(`CREATE TABLE schema_versions (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	return conn
}

func TestApplyMigrationRollsBackOnBodyFailure(t *testing.T) {
	// Given a migration whose SQL body cannot execute
	conn := newSchemaVersionsDB(t)

	// When
	err := applyMigration(context.Background(), conn, migration{
		version: 99,
		name:    "0099_bad.sql",
		body:    "THIS IS NOT SQL",
	})

	// Then the failure is reported and no version record survives.
	require.ErrorContains(t, err, "exec migration body")

	var recorded int
	require.NoError(t, conn.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = 99").Scan(&recorded))
	require.Zero(t, recorded)
}

func TestApplyMigrationRollsBackBodyWhenRecordFails(t *testing.T) {
	// Given a version row that already exists, so recording the
	// migration violates the primary key after its body has run
	conn := newSchemaVersionsDB(t)
	_, err := conn.Exec("INSERT INTO schema_versions (version) VALUES (99)")
	require.NoError(t, err)

	// When
	err = applyMigration(context.Background(), conn, migration{
		version: 99,
		name:    "0099_probe.sql",
		body:    "CREATE TABLE rollback_probe (id INTEGER)",
	})

	// Then the record failure is reported and the body's table was
	// rolled back with it.
	require.ErrorContains(t, err, "record migration")

	var tables int
	require.NoError(t, conn.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'rollback_probe'",
	).Scan(&tables))
	require.Zero(t, tables)
}
