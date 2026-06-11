package translations

import (
	"embed"
	"testing"

	"fyne.io/fyne/v2/lang"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/broken
var brokenCatalogs embed.FS

func TestRegisterLoadsEmbeddedCatalog(t *testing.T) {
	// When
	err := Register()

	// Then the catalog parses and a known key resolves to itself
	// (English strings double as keys).
	require.NoError(t, err)
	require.Equal(t, "Export CSV", lang.L("Export CSV"))
}

func TestRegisterReportsCorruptCatalog(t *testing.T) {
	// When loading a fixture that is not valid JSON
	err := register(brokenCatalogs, "testdata/broken")

	// Then
	require.ErrorContains(t, err, "register translations")
}
