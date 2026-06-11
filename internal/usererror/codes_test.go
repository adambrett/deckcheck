package usererror_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// codePattern matches the literal call-site code strings, e.g.
// `"DC06"`. The leading quote anchors against accidental matches in
// identifiers or comments.
var codePattern = regexp.MustCompile(`"DC\d+"`)

// TestUniqueCallSiteCodes scans the project's Go sources for
// `"DCnnn"` literals and fails when the same code appears at more
// than one location. Codes are bound to call sites by convention;
// reusing a code makes a user-reported identifier ambiguous.
//
// The test skips _test.go files (test fixtures often repeat codes)
// and line comments (doc examples mention codes in prose).
func TestUniqueCallSiteCodes(t *testing.T) {
	// Given
	root := repoRoot(t)

	type hit struct {
		file string
		line int
	}
	seen := make(map[string][]hit)

	// When
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for lineNo, line := range strings.Split(string(content), "\n") {
			// Skip line comments; example codes mentioned in doc
			// comments are not real call-site uses.
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
				continue
			}
			for _, match := range codePattern.FindAllString(line, -1) {
				code := strings.Trim(match, `"`)
				rel, _ := filepath.Rel(root, path)
				seen[code] = append(seen[code], hit{file: rel, line: lineNo + 1})
			}
		}
		return nil
	})
	require.NoError(t, walkErr)

	// Then
	var dups []string
	for code, hits := range seen {
		if len(hits) > 1 {
			locs := make([]string, 0, len(hits))
			for _, h := range hits {
				locs = append(locs, h.file+":"+strconv.Itoa(h.line))
			}
			sort.Strings(locs)
			dups = append(dups, code+" -> "+strings.Join(locs, ", "))
		}
	}
	sort.Strings(dups)

	require.Emptyf(t, dups, "duplicate call-site codes:\n  %s", strings.Join(dups, "\n  "))

	// Retired codes must not come back: a reused retired code would
	// appear only once and slip past the duplicate check above.
	for _, retired := range []string{"DC01", "DC02"} {
		require.NotContainsf(t, seen, retired, "support code %s is retired and must not be reused", retired)
	}
}

// repoRoot walks up from this test file until it sees go.mod. Robust
// to test invocation from anywhere inside the module.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine caller path")

	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqualf(t, parent, dir, "walked above filesystem root without finding go.mod")
		dir = parent
	}
}

// skipDir filters directories that should not be scanned. The build
// caches and generated assets carry no source.
func skipDir(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}
	switch base {
	case "bin", "node_modules", "graphify-out":
		return true
	}
	return false
}
