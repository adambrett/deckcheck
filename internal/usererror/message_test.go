package usererror_test

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/db"
	"github.com/adambrett/deckcheck/internal/projectfile"
	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestForErrorReturnsFriendlyUserProblemForInvalidProjectFile(t *testing.T) {
	// Given
	err := fmt.Errorf("%w: %w", usererror.ErrOpenProject, projectfile.ErrInvalidProjectFile)

	// When
	message := usererror.ForError(err)

	// Then
	require.Equal(t, usererror.SeverityError, message.Severity)
	require.Equal(t, "Could not open project", message.Summary)
	require.Contains(t, bodyOf(message), "not a DeckCheck project")
	require.Contains(t, bodyOf(message), "The file was not changed")
	require.NotContains(t, bodyOf(message), "Impact")
	require.NotContains(t, bodyOf(message), "Resolution")
	require.NotContains(t, bodyOf(message), "invalid project file")
}

func TestForErrorShowsCallSiteCodeForCodedWrap(t *testing.T) {
	// Given a routed error with a WithCode wrap at the call site
	err := usererror.WithCode("TESTING42",
		fmt.Errorf("%w: %w", usererror.ErrCreateProject, errors.New("temp file gone")))

	// When
	message := usererror.ForError(err)

	// Then the code is taken from the wrap; routing copy is preserved.
	require.Equal(t, "TESTING42", message.Code)
	require.Equal(t, "Could not create project", message.Summary)
	require.Contains(t, bodyOf(message), "could not finish creating")
}

func TestForErrorLeavesCodeEmptyWithoutCodedWrap(t *testing.T) {
	// Given a routed error with no call-site code applied
	err := fmt.Errorf("%w: %w", usererror.ErrCreateProject, errors.New("temp file gone"))

	// When
	message := usererror.ForError(err)

	// Then the code is empty; the dialog renders without a code chip.
	require.Empty(t, message.Code)
	require.Equal(t, "Could not create project", message.Summary)
}

func TestForErrorReturnsUpgradeGuidanceForNewerProjectFile(t *testing.T) {
	// Given
	err := fmt.Errorf("%w: %w", usererror.ErrOpenProject, db.ErrSchemaTooNew)

	// When
	message := usererror.ForError(err)

	// Then
	require.Contains(t, bodyOf(message), "newer version of DeckCheck")
	require.Contains(t, bodyOf(message), "Update DeckCheck")
}

func TestForErrorReturnsUserProblemForMissingProjectFile(t *testing.T) {
	// Given
	err := fmt.Errorf("%w: %w", usererror.ErrOpenProject, fs.ErrNotExist)

	// When
	message := usererror.ForError(err)

	// Then
	require.Equal(t, "Could not open project", message.Summary)
	require.Contains(t, bodyOf(message), "could not be found")
	require.NotContains(t, bodyOf(message), "file does not exist")
}

func TestForErrorReturnsProjectCreationGuidanceForDatasetProblem(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "empty csv", err: dataset.ErrEmptyCSV, want: "does not contain any data rows"},
		{name: "bad header", err: dataset.ErrInvalidCSVHeader, want: "unique, non-empty name"},
		{name: "bad row", err: dataset.ErrInvalidCSVRow, want: "every row matches the header"},
		{name: "no images", err: dataset.ErrNoImages, want: "supported image files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := fmt.Errorf("%w: %w", usererror.ErrCreateProject, tt.err)

			// When
			message := usererror.ForError(err)

			// Then
			require.Equal(t, "Could not create project", message.Summary)
			require.Contains(t, bodyOf(message), tt.want)
			require.NotContains(t, bodyOf(message), tt.err.Error())
		})
	}
}

func TestForErrorReturnsFallbackMessageForUnmappedFailure(t *testing.T) {
	// Given
	err := fmt.Errorf("%w: %w", usererror.ErrSaveClassification, errors.New("disk is gone"))

	// When
	message := usererror.ForError(err)

	// Then
	require.Equal(t, usererror.SeverityError, message.Severity)
	require.Equal(t, "Could not save classification", message.Summary)
	require.NotContains(t, bodyOf(message), "Severity:")
	require.NotContains(t, bodyOf(message), "Code:")
	require.NotContains(t, bodyOf(message), "Impact")
	require.NotContains(t, bodyOf(message), "Resolution")
	require.NotContains(t, bodyOf(message), "disk is gone")
}

func TestForErrorPopulatesTechnicalFromUnderlyingError(t *testing.T) {
	// Given an arbitrary wrapped chain
	err := fmt.Errorf("%w: %w", usererror.ErrCreateProject, errors.New("disk full"))

	// When
	message := usererror.ForError(err)

	// Then Technical carries the raw chain for debug-mode rendering;
	// nothing in the public body reveals it.
	require.Equal(t, err.Error(), message.Technical)
	require.NotContains(t, bodyOf(message), "disk full")
}

// TestForErrorMapsEverySentinelChain pins the routing table: every supported
// (error chain → rendered message) pair is exercised here so a
// refactor that loses one of the load-bearing two-axis matches or
// reorders the switch precedence breaks loudly.
func TestForErrorMapsEverySentinelChain(t *testing.T) {
	// Given
	tests := []struct {
		name       string
		err        error
		wantSum    string
		wantInBody string
	}{
		{
			name: "open project missing file (two-axis match)",
			// The classic chain produced when projectfile.Open's os.Stat
			// reports fs.ErrNotExist: outer ErrOpenProject wrap, inner
			// "open database" wrap of the syscall error.
			err: fmt.Errorf("%w: %w",
				usererror.ErrOpenProject,
				fmt.Errorf("open database: %w", fs.ErrNotExist),
			),
			wantSum:    "Could not open project",
			wantInBody: "could not be found",
		},
		{
			name: "missing image column under create wrap (expectedUserProblem precedence)",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				dataset.ErrMissingImageColumn,
			),
			wantSum:    "Could not create project",
			wantInBody: "image column was not selected",
		},
		{
			name: "image column not found under create wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				dataset.ErrImageColumnNotFound,
			),
			wantSum:    "Could not create project",
			wantInBody: "does not contain the chosen image",
		},
		{
			name: "invalid question under create wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				projectfile.ErrInvalidQuestion,
			),
			wantSum:    "Could not create project",
			wantInBody: "questions step",
		},
		{
			name: "csv header read failure under wizard wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrReadCSVHeaders,
				errors.New("io read failed"),
			),
			wantSum:    "Could not read CSV columns",
			wantInBody: "could not read the header row",
		},
		{
			name: "unsupported dataset type under create wrap (fallback precedence)",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				dataset.ErrUnsupportedDatasetType,
			),
			wantSum:    "Could not create project",
			wantInBody: "produced an invalid project setup",
		},
		{
			name: "create project bare wrap routes to create-failed fallback",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				errors.New("temp file gone"),
			),
			wantSum:    "Could not create project",
			wantInBody: "could not finish creating",
		},
		{
			name: "load questions bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrLoadQuestions,
				errors.New("query exploded"),
			),
			wantSum:    "Could not open project",
			wantInBody: "could not load its questions",
		},
		{
			name: "initialize classifier bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrInitializeClassifier,
				errors.New("activate failed"),
			),
			wantSum:    "Could not open classifier",
			wantInBody: "could not prepare the classifier",
		},
		{
			name: "navigation wraps fold into one message",
			err: fmt.Errorf("%w: %w",
				usererror.ErrFindNextUnclassifiedRecord,
				errors.New("scan failed"),
			),
			wantSum:    "Could not move through records",
			wantInBody: "could not find the next record",
		},
		{
			name: "first-unclassified bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrFindFirstUnclassifiedRecord,
				errors.New("scan failed"),
			),
			wantSum:    "Could not move through records",
			wantInBody: "could not find the next record",
		},
		{
			name: "load record bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrLoadRecord,
				errors.New("query failed"),
			),
			wantSum:    "Could not load record",
			wantInBody: "could not load the selected record",
		},
		{
			name: "delete classification bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrDeleteClassification,
				errors.New("constraint failed"),
			),
			wantSum:    "Could not remove classification",
			wantInBody: "could not remove the selected answer",
		},
		{
			name: "export csv bare wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrExportCSV,
				errors.New("disk full"),
			),
			wantSum:    "Could not export CSV",
			wantInBody: "could not write the export file",
		},
		{
			name: "invalid project options under create wrap",
			err: fmt.Errorf("%w: %w",
				usererror.ErrCreateProject,
				projectfile.ErrInvalidProjectOptions,
			),
			wantSum:    "Could not create project",
			wantInBody: "produced an invalid project setup",
		},
		{
			name:       "bare unknown error falls through to UNEXPECTED",
			err:        errors.New("ran out of badgers"),
			wantSum:    "DeckCheck hit a problem",
			wantInBody: "could not complete the action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			message := usererror.ForError(tt.err)

			// Then
			require.Equal(t, usererror.SeverityError, message.Severity, "Severity")
			require.Equal(t, tt.wantSum, message.Summary, "Summary")
			require.Contains(t, bodyOf(message), tt.wantInBody, "Body content")
		})
	}
}

// bodyOf joins the user-facing paragraphs so the assertions below can
// check copy without caring which paragraph carries it.
func bodyOf(m usererror.Message) string {
	return strings.Join([]string{m.Description, m.Impact, m.Resolution}, "\n\n")
}
