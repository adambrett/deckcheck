package translations

import (
	"embed"
	"fmt"

	"fyne.io/fyne/v2/lang"
)

// catalogs holds one JSON file per supported locale. English strings
// double as the lookup keys, so en.json is an identity catalog and a
// new locale is added by dropping in its translated copy.
//
//go:embed translations
var catalogs embed.FS

// Register loads every embedded catalog into Fyne's localisation
// table. It must run before any UI string is rendered.
func Register() error {
	return register(catalogs, "translations")
}

func register(catalogs embed.FS, dir string) error {
	if err := lang.AddTranslationsFS(catalogs, dir); err != nil {
		return fmt.Errorf("register translations: %w", err)
	}

	return nil
}
