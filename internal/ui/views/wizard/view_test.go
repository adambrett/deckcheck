//go:build integration

package wizard_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard"
	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestWizardCompletesForEachDatasetType(t *testing.T) {
	// Given
	tests := []struct {
		name        string
		radioLabel  string
		fixture     func(t *testing.T, dir string) string
		imageColumn string
		wantType    dataset.Type
	}{
		{
			name:       "csv",
			radioLabel: "CSV File",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				return writeFile(t, dir, "data.csv", "text\nhello\n")
			},
			wantType: dataset.TypeCSV,
		},
		{
			name:       "image folder",
			radioLabel: "Image Folder",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				imageDir := filepath.Join(dir, "images")
				require.NoError(t, os.MkdirAll(imageDir, 0o755))
				writeFile(t, imageDir, "photo.png", "fake")
				return imageDir
			},
			wantType: dataset.TypeImages,
		},
		{
			name:       "csv with images",
			radioLabel: "CSV with Image References",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				return writeFile(t, dir, "images.csv", "text,image\nhello,photo.png\n")
			},
			imageColumn: "image",
			wantType:    dataset.TypeCSVWithImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a wizard whose pickers immediately return fixtures.
			test.NewApp()
			tmpDir := t.TempDir()
			projectPath := filepath.Join(tmpDir, tt.name+".deckcheck")
			datasetPath := tt.fixture(t, tmpDir)

			var got wizard.Result
			completed := false
			view := wizard.New(wizard.Config{
				ProjectPicker: &fynetest.StubPicker{SavePath: projectPath},
				DatasetPicker: &fynetest.StubPicker{OpenPath: datasetPath},
				Handlers: wizard.Handlers{
					Complete: func(result wizard.Result) {
						got = result
						completed = true
					},
				},
			})
			require.NoError(t, view.Activate())
			root := view.Container()

			// When walking the three steps like a user would.
			fynetest.TypeEntry(t, root, "My Classification Project", "Flow Project")
			fynetest.TapButton(t, root, "Choose file…")
			fynetest.TapButton(t, root, "Next")

			fynetest.SelectRadio(t, root, tt.radioLabel)
			fynetest.TapButton(t, root, browseLabelFor(tt.wantType))
			view.WaitForPendingOperations()
			if tt.imageColumn != "" {
				fynetest.SelectOption(t, root, tt.imageColumn)
			}
			fynetest.TapButton(t, root, "Next")

			fynetest.TypeEntry(t, root, "e.g., What is the sentiment?", "Useful?")
			fynetest.TypeEntry(t, root, "e.g., Positive, Negative, Neutral", "Yes, No")
			fynetest.TapButton(t, root, "Finish")

			// Then Complete receives the assembled result.
			require.True(t, completed, "Complete handler was not invoked")
			require.Equal(t, "Flow Project", got.ProjectName)
			require.Equal(t, projectPath, got.DBPath)
			require.Equal(t, datasetPath, got.DatasetPath)
			require.Equal(t, tt.wantType, got.DatasetType)
			require.Equal(t, tt.imageColumn, got.ImageColumn)
			require.Equal(t, []project.QuestionDef{
				{Kind: project.QuestionKindChoice, Text: "Useful?", Answers: []string{"Yes", "No"}},
			}, got.Questions)
		})
	}
}

func TestWizardStaysOnInvalidStep(t *testing.T) {
	// Given
	test.NewApp()
	completed := false
	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{},
		DatasetPicker: &fynetest.StubPicker{},
		Handlers: wizard.Handlers{
			Complete: func(wizard.Result) { completed = true },
		},
	})
	require.NoError(t, view.Activate())
	root := view.Container()

	// When advancing with nothing filled in
	fynetest.TapButton(t, root, "Next")

	// Then the wizard stays on the first step and explains each field.
	fynetest.RequireText(t, root, "Enter a project name.")
	fynetest.RequireText(t, root, "Choose where to save the project database.")
	fynetest.RequireText(t, root, "Step 1 of 3")

	// When completing step one but skipping the dataset
	fynetest.TypeEntry(t, root, "My Classification Project", "Validation Project")
	fynetest.TypeEntry(t, root, "/path/to/project.deckcheck", filepath.Join(t.TempDir(), "p.deckcheck"))
	fynetest.TapButton(t, root, "Next")
	fynetest.TapButton(t, root, "Next")

	// Then
	fynetest.RequireText(t, root, "Choose a dataset file or folder.")
	fynetest.RequireText(t, root, "Step 2 of 3")

	// When reaching the questions step with empty questions
	fynetest.TypeEntry(t, root, "Select file or folder…", filepath.Join(t.TempDir(), "data.csv"))
	fynetest.TapButton(t, root, "Next")
	fynetest.TapButton(t, root, "Finish")

	// Then the wizard demands questions rather than completing.
	fynetest.RequireText(t, root, "Enter the question text.")
	fynetest.RequireText(t, root, "Add at least 2 answers separated by commas.")
	require.False(t, completed)
}

func TestWizardCancelConfirmsWhenDirty(t *testing.T) {
	// Given a wizard with typed input and a confirm-capturing host.
	test.NewApp()
	cancelled := false
	var confirmResponse func(bool)
	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{},
		DatasetPicker: &fynetest.StubPicker{},
		Handlers: wizard.Handlers{
			Cancel: func() { cancelled = true },
			Confirm: func(_, _ string, response func(confirmed bool)) {
				confirmResponse = response
			},
		},
	})
	require.NoError(t, view.Activate())
	root := view.Container()
	fynetest.TypeEntry(t, root, "My Classification Project", "Half-typed")

	// When cancelling
	fynetest.TapButton(t, root, "Cancel")

	// Then the host is asked first; confirming completes the cancel.
	require.NotNil(t, confirmResponse, "Confirm handler was not invoked")
	require.False(t, cancelled)

	confirmResponse(true)
	require.True(t, cancelled)
}

func TestWizardCancelSkipsConfirmWhenPristine(t *testing.T) {
	// Given an untouched wizard
	test.NewApp()
	cancelled := false
	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{},
		DatasetPicker: &fynetest.StubPicker{},
		Handlers: wizard.Handlers{
			Cancel:  func() { cancelled = true },
			Confirm: func(_, _ string, _ func(confirmed bool)) { t.Fatal("confirm should not run for pristine wizard") },
		},
	})
	require.NoError(t, view.Activate())

	// When
	fynetest.TapButton(t, view.Container(), "Cancel")

	// Then
	require.True(t, cancelled)
}

func TestWizardNavigatesBackAndRetainsInput(t *testing.T) {
	// Given a wizard on step 2 with a typed project name on step 1.
	test.NewApp()
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "project.deckcheck")
	csvPath := writeFile(t, tmpDir, "data.csv", "name\nhello\n")

	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{SavePath: projectPath},
		DatasetPicker: &fynetest.StubPicker{OpenPath: csvPath},
		Handlers:      wizard.Handlers{},
	})
	require.NoError(t, view.Activate())
	root := view.Container()

	// Fill step 1 and advance.
	fynetest.TypeEntry(t, root, "My Classification Project", "Retain Me")
	fynetest.TapButton(t, root, "Choose file…")
	fynetest.TapButton(t, root, "Next")
	fynetest.RequireText(t, root, "Select dataset")

	// When tapping Previous.
	fynetest.TapButton(t, root, "Previous")

	// Then step 1 is shown again with the previously typed name intact.
	fynetest.RequireText(t, root, "Create new project")
	fynetest.RequireEntryValue(t, root, "My Classification Project", "Retain Me")
}

func TestWizardNavigatesBetweenQuestionsBeforeFinishing(t *testing.T) {
	// Given a wizard on the questions step with two questions
	test.NewApp()
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "project.deckcheck")
	csvPath := writeFile(t, tmpDir, "data.csv", "name\nhello\n")

	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{SavePath: projectPath},
		DatasetPicker: &fynetest.StubPicker{OpenPath: csvPath},
		Handlers:      wizard.Handlers{},
	})
	require.NoError(t, view.Activate())
	root := view.Container()

	fynetest.TypeEntry(t, root, "My Classification Project", "Question Navigation")
	fynetest.TapButton(t, root, "Choose file…")
	fynetest.TapButton(t, root, "Next")
	fynetest.TapButton(t, root, "Choose file…")
	fynetest.TapButton(t, root, "Next")

	fynetest.TypeEntry(t, root, "e.g., What is the sentiment?", "First?")
	fynetest.TypeEntry(t, root, "e.g., Positive, Negative, Neutral", "Yes, No")
	fynetest.TapButton(t, root, "Add Question")
	fynetest.TypeEntry(t, root, "e.g., What is the sentiment?", "Second?")
	fynetest.TypeEntry(t, root, "e.g., Positive, Negative, Neutral", "A, B")

	// When moving back from the second question
	fynetest.TapButton(t, root, "Previous")

	// Then the wizard stays on the questions step and the primary
	// action is Next, not Finish, until the last question is active.
	fynetest.RequireText(t, root, "Question 1 of 2")
	fynetest.ButtonWithText(t, root, "Next")

	// When moving forward again
	fynetest.TapButton(t, root, "Next")

	// Then the final question can finish the wizard.
	fynetest.RequireText(t, root, "Question 2 of 2")
	fynetest.ButtonWithText(t, root, "Finish")
}

func TestWizardQuestionTypesFollowDatasetType(t *testing.T) {
	// Given
	tests := []struct {
		name          string
		radioLabel    string
		fixture       func(t *testing.T, dir string) string
		imageColumn   string
		datasetType   dataset.Type
		wantImageType bool
	}{
		{
			name: "csv",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				return writeFile(t, dir, "data.csv", "text\nhello\n")
			},
			datasetType: dataset.TypeCSV,
		},
		{
			name:       "image folder",
			radioLabel: "Image Folder",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				imageDir := filepath.Join(dir, "images")
				require.NoError(t, os.MkdirAll(imageDir, 0o755))
				writeFile(t, imageDir, "photo.png", "fake")
				return imageDir
			},
			datasetType:   dataset.TypeImages,
			wantImageType: true,
		},
		{
			name:       "csv with images",
			radioLabel: "CSV with Image References",
			fixture: func(t *testing.T, dir string) string {
				t.Helper()
				return writeFile(t, dir, "images.csv", "text,image\nhello,photo.png\n")
			},
			imageColumn:   "image",
			datasetType:   dataset.TypeCSVWithImage,
			wantImageType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a wizard that can reach the questions step for this dataset type.
			test.NewApp()
			tmpDir := t.TempDir()
			view := wizard.New(wizard.Config{
				ProjectPicker: &fynetest.StubPicker{SavePath: filepath.Join(tmpDir, "project.deckcheck")},
				DatasetPicker: &fynetest.StubPicker{OpenPath: tt.fixture(t, tmpDir)},
			})
			require.NoError(t, view.Activate())
			root := view.Container()

			// When navigating to the questions step.
			fynetest.TypeEntry(t, root, "My Classification Project", "Types")
			fynetest.TapButton(t, root, "Choose file…")
			fynetest.TapButton(t, root, "Next")
			if tt.radioLabel != "" {
				fynetest.SelectRadio(t, root, tt.radioLabel)
			}
			fynetest.TapButton(t, root, browseLabelFor(tt.datasetType))
			view.WaitForPendingOperations()
			if tt.imageColumn != "" {
				fynetest.SelectOption(t, root, tt.imageColumn)
			}
			fynetest.TapButton(t, root, "Next")

			// Then image annotation is only offered for image-backed datasets.
			options := questionTypeOptions(t, root)
			require.Equal(t, tt.wantImageType, slices.Contains(options, "Image annotation"))
		})
	}
}

func TestWizardDropsCSVHeaderResultAfterClose(t *testing.T) {
	// Given a wizard whose CSV probe fires after the view is closed.
	// Close() cancels v.ctx, so LoadHeaders returns context.Canceled.
	// Without the closed guard that callback would call the error
	// handler; with it the late delivery is silently dropped.
	test.NewApp()
	tmpDir := t.TempDir()
	csvPath := writeFile(t, tmpDir, "images.csv", "text,image\nhello,photo.png\n")

	errorCalled := false
	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{SavePath: filepath.Join(tmpDir, "project.deckcheck")},
		DatasetPicker: &fynetest.StubPicker{OpenPath: csvPath},
		Handlers: wizard.Handlers{
			Error: func(error) { errorCalled = true },
		},
	})
	require.NoError(t, view.Activate())
	root := view.Container()

	// Navigate to the dataset step and arm the CSV-with-images probe.
	fynetest.TypeEntry(t, root, "My Classification Project", "Test Project")
	fynetest.TapButton(t, root, "Choose file…")
	fynetest.TapButton(t, root, "Next")
	fynetest.SelectRadio(t, root, "CSV with Image References")

	// Close before the browse/probe cycle runs.
	view.Close()

	// When triggering the probe on the now-closed view. The stub picker
	// calls onSelected synchronously; probeCSVHeaders launches its
	// goroutine with an already-cancelled context.
	fynetest.TapButton(t, root, "Choose file…")
	view.WaitForPendingOperations()

	// Then the late callback is dropped and no error dialog appears.
	require.False(t, errorCalled)
}

func questionTypeOptions(t *testing.T, root fyne.CanvasObject) []string {
	t.Helper()

	var options []string
	fynetest.Walk(root, func(obj fyne.CanvasObject) {
		selectWidget, ok := obj.(*widget.Select)
		if ok && slices.Contains(selectWidget.Options, "Multiple choice") {
			options = append([]string(nil), selectWidget.Options...)
		}
	})
	require.NotEmpty(t, options, "question type dropdown not found")

	return options
}

func TestWizardWithParentContextScopesCSVProbe(t *testing.T) {
	// Given a wizard rooted at a parent context that is already cancelled.
	test.NewApp()
	tmpDir := t.TempDir()
	csvPath := writeFile(t, tmpDir, "images.csv", "text,image\nhello,photo.png\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var capturedErr error
	view := wizard.New(wizard.Config{
		ProjectPicker: &fynetest.StubPicker{SavePath: filepath.Join(tmpDir, "project.deckcheck")},
		DatasetPicker: &fynetest.StubPicker{OpenPath: csvPath},
		Handlers: wizard.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	}, wizard.WithParentContext(ctx))
	require.NoError(t, view.Activate())
	root := view.Container()

	// Navigate to the dataset step.
	fynetest.TypeEntry(t, root, "My Classification Project", "Test Project")
	fynetest.TapButton(t, root, "Choose file…")
	fynetest.TapButton(t, root, "Next")
	fynetest.SelectRadio(t, root, "CSV with Image References")

	// When browsing, which triggers a CSV probe using the cancelled context.
	fynetest.TapButton(t, root, "Choose file…")
	view.WaitForPendingOperations()

	// Then the probe failure propagates as a read-headers error.
	require.ErrorIs(t, capturedErr, usererror.ErrReadCSVHeaders)
}

// browseLabelFor mirrors the dataset step's browse-button labelling:
// folder sources get a folder label, file sources a file label.
func browseLabelFor(t dataset.Type) string {
	if t == dataset.TypeImages {
		return "Choose folder…"
	}

	return "Choose file…"
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}
