package db

import "errors"

var (
	// ErrSchemaTooNew is returned by Open when a project database records a
	// schema version greater than [CurrentSchemaVersion]. The file is left
	// untouched so a newer build of DeckCheck can still open it.
	ErrSchemaTooNew = errors.New("project file is from a newer DeckCheck")

	// ErrNotDeckCheckProject is returned by Open when the file is a DuckDB
	// database, but not a recognizable DeckCheck project database. The file is
	// left untouched.
	ErrNotDeckCheckProject = errors.New("not a DeckCheck project database")
)
