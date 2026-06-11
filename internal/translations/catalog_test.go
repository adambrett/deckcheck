package translations_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// langLiteral matches lang.L("...") call sites, tolerating whitespace
// or a line break between the paren and the key, and trailing
// template-data arguments. Keys must be passed as string literals
// (not consts or variables) precisely so this gate can see them.
var langLiteral = regexp.MustCompile(`lang\.L\(\s*"((?:[^"\\]|\\.)*)"`)

// messageCopy matches the user-facing copy fields of usererror.Message
// literals, which the error dialog localises at render time.
var messageCopy = regexp.MustCompile(`(?:Summary|Description|Impact|Resolution):\s+"((?:[^"\\]|\\.)*)"`)

// TestCatalogContainsEveryUsedKey walks the source tree, extracts every
// localisation key the code can ask for, and fails if en.json is
// missing any; this is the gate that stops catalog drift.
func TestCatalogContainsEveryUsedKey(t *testing.T) {
	catalog := loadCatalog(t)

	for key, file := range usedKeys(t) {
		require.Contains(t, catalog, key, "key used in %s is missing from en.json", file)
	}
}

func loadCatalog(t *testing.T) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("translations", "en.json"))
	require.NoError(t, err)

	catalog := map[string]string{}
	require.NoError(t, json.Unmarshal(raw, &catalog))
	return catalog
}

// usedKeys returns every localisation key found in the source tree,
// mapped to the file it was found in for the failure message.
func usedKeys(t *testing.T) map[string]string {
	t.Helper()

	root := repoRoot(t)
	keys := map[string]string{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "mocks" || name == "website" || name == "bin" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range langLiteral.FindAllStringSubmatch(string(src), -1) {
			keys[unquote(m[1])] = path
		}
		if strings.HasSuffix(path, filepath.Join("usererror", "message.go")) {
			for _, m := range messageCopy.FindAllStringSubmatch(string(src), -1) {
				keys[unquote(m[1])] = path
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, keys, "no lang.L call sites found; extraction broken?")

	return keys
}

func unquote(s string) string {
	unquoted, err := strconv.Unquote(`"` + s + `"`)
	if err != nil {
		return s
	}
	return unquoted
}

// repoRoot walks up from the package directory to the go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found above test directory")
		dir = parent
	}
}
