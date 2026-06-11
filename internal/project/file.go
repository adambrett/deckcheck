package project

import (
	"path/filepath"
	"strings"
)

// FileExtension is the file extension used for DeckCheck project databases.
// The underlying format is DuckDB; the distinct extension prevents users from
// accidentally selecting unrelated DuckDB files as a project.
const FileExtension = ".deckcheck"

// MaxNameLength caps project names so the suggested filename derived
// from them (see [DefaultFilename]) stays well within per-filesystem
// path-length limits; most filesystems reject paths above ~255 bytes.
const MaxNameLength = 120

// DefaultFilename returns a safe default filename for a project name.
func DefaultFilename(name string) string {
	filename := strings.Join(strings.Fields(name), " ")
	filename = strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"\x00", "",
	).Replace(filename)
	filename = strings.Trim(filename, ". ")
	if filename == "" {
		filename = "project"
	}
	if strings.EqualFold(filepath.Ext(filename), FileExtension) {
		return filename
	}
	return filename + FileExtension
}
