//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	_ "github.com/marcboeker/go-duckdb" // register the DuckDB driver for the file-damage helpers
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/go-fyne/pkg/browse"
	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	rootmocks "github.com/adambrett/deckcheck/internal/mocks"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/projectfile"
	deckapp "github.com/adambrett/deckcheck/internal/ui/app"
	"github.com/adambrett/deckcheck/internal/ui/mocks"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard"
	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestActivateWelcomeAndWizardViews(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)

	// When
	controller.ActivateWelcomeView()

	// Then
	fynetest.RequireTextContains(t, deps.root(), "New Project")
	fynetest.RequireTextContains(t, deps.root(), "Open Project")

	// When
	controller.ActivateWizardView()

	// Then
	fynetest.RequireTextContains(t, deps.root(), "Create new project")
	fynetest.RequireTextContains(t, deps.root(), "My Classification Project")
}

func TestActivateClassifierViewSwapsActiveViewAndClosesPreviousProject(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	previousProject := expectProjectForClassifier(t, "first")
	nextProject := expectProjectForClassifier(t, "second")

	// The release is asynchronous: the App lets the outgoing view's
	// background writes drain before closing the previous project.
	released := make(chan struct{})
	previousProject.EXPECT().Close().RunAndReturn(func() error {
		close(released)
		return nil
	}).Once()

	require.NoError(t, controller.ActivateClassifierView(previousProject))
	previousRoot := deps.root()
	fynetest.RequireTextContains(t, previousRoot, "first")

	// When
	require.NoError(t, controller.ActivateClassifierView(nextProject))

	// Then
	require.NotSame(t, previousRoot, deps.root())
	fynetest.RequireTextContains(t, deps.root(), "second")
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("previous project was never closed")
	}
}

func TestActivateClassifierViewPreservesCurrentProjectOnFailure(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	currentProject := expectProjectForClassifier(t, "current")
	brokenProject := mocks.NewAppProject(t)
	brokenProject.EXPECT().Questions(mock.Anything).Return(nil, errors.New("boom")).Once()

	require.NoError(t, controller.ActivateClassifierView(currentProject))
	currentRoot := deps.root()

	// When
	err := controller.ActivateClassifierView(brokenProject)

	// Then
	require.ErrorIs(t, err, usererror.ErrLoadQuestions)
	require.Same(t, currentRoot, deps.root())
	fynetest.RequireTextContains(t, deps.root(), "current")
}

func TestCreateProjectCreatesRemembersAndActivatesProject(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	tmpDir := t.TempDir()
	datasetPath := filepath.Join(tmpDir, "source.csv")
	projectPath := filepath.Join(tmpDir, "project.deckcheck")
	require.NoError(t, os.WriteFile(datasetPath, []byte("text\nHello\n"), 0o644))

	// When
	controller.CreateProject(wizard.Result{
		ProjectName: "Test",
		DBPath:      projectPath,
		DatasetPath: datasetPath,
		DatasetType: dataset.TypeCSV,
		Questions: []project.QuestionDef{
			{Text: "Valid?", Answers: []string{"Yes", "No"}},
		},
	})
	controller.WaitForPendingOperations()

	// Then
	fynetest.RequireTextContains(t, deps.root(), "Hello")
	require.Contains(t, deps.app.Preferences().StringList(recent.DefaultPreferencesKey), projectPath)
}

func TestCreateProjectLeavesViewUnchangedOnCreateError(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()
	existingRoot := deps.root()

	// When
	controller.CreateProject(wizard.Result{
		ProjectName: "Test",
		DBPath:      filepath.Join(t.TempDir(), "project.deckcheck"),
		DatasetPath: filepath.Join(t.TempDir(), "missing.csv"),
		DatasetType: dataset.TypeCSV,
	})
	controller.WaitForPendingOperations()

	// Then
	require.Same(t, existingRoot, deps.root())
}

func TestOpenProjectLoadsRemembersAndActivatesProject(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "project.deckcheck")
	createCSVProject(t, projectPath, "Opened")

	// When
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()

	// Then
	fynetest.RequireTextContains(t, deps.root(), "Opened")
	require.Contains(t, deps.app.Preferences().StringList(recent.DefaultPreferencesKey), projectPath)
}

func TestProjectHandlersLeaveViewUnchangedOnInputErrors(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()
	existingRoot := deps.root()

	// When
	controller.CreateProject(wizard.Result{
		ProjectName: "Test",
		DBPath:      filepath.Join(t.TempDir(), "project.deckcheck"),
		DatasetType: dataset.Type("unknown"),
	})
	controller.OpenProject("missing.deckcheck")
	controller.WaitForPendingOperations()

	// Then
	require.Same(t, existingRoot, deps.root())
}

func TestRequestCloseWithOpenProjectReturnsToWelcome(t *testing.T) {
	// Given an open project showing in the classifier
	controller, deps := newTestApp(t)
	currentProject := expectProjectForClassifier(t, "open row")
	released := make(chan struct{})
	currentProject.EXPECT().Close().RunAndReturn(func() error {
		close(released)
		return nil
	}).Once()
	require.NoError(t, controller.ActivateClassifierView(currentProject))

	// When the user asks to close the window
	controller.RequestClose()

	// Then the launcher is shown and the project is released once the
	// view's background work has drained.
	fynetest.RequireTextContains(t, deps.root(), "New Project")
	fynetest.RequireTextContains(t, deps.root(), "Open Project")
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("project was never closed")
	}
}

func TestRequestCloseDuringDirtyWizardConfirmsFirst(t *testing.T) {
	// Given a wizard with typed input
	controller, deps := newTestApp(t)
	closed := false
	deps.window.SetOnClosed(func() { closed = true })
	controller.ActivateWizardView()
	fynetest.TypeEntry(t, deps.root(), "My Classification Project", "Half-typed")

	// When the user asks to close the window
	controller.RequestClose()

	// Then a confirmation dialog appears instead of silently
	// discarding the input.
	require.False(t, closed)
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay, "expected a confirm dialog before close")
	fynetest.RequireTextContains(t, overlay, "Discard wizard progress?")

	// When confirming the discard
	fynetest.TapButton(t, overlay, "Yes")

	// Then the close proceeds for real.
	require.True(t, closed)
}

func TestRequestCloseIsDroppedWhileConfirmDialogShows(t *testing.T) {
	// Given a dirty wizard over an open project with its discard
	// confirmation already on screen
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "stacked.deckcheck")
	createCSVProject(t, projectPath, "Stacked row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()
	fileMenuItem(t, deps.window, "New Project").Action()
	fynetest.TypeEntry(t, deps.root(), "My Classification Project", "Half-typed")

	closed := false
	deps.window.SetOnClosed(func() { closed = true })
	controller.RequestClose()
	first := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, first)

	// When the user hits Cmd+W again while the dialog is up (Fyne
	// dispatches shortcuts even with an overlay showing)
	controller.RequestClose()

	// Then no second confirm stacked on top of the first...
	require.Same(t, first, deps.window.Canvas().Overlays().Top())

	// ...and confirming once performs exactly one close: project to
	// launcher, window alive; not a double-run that would quit.
	fynetest.TapButton(t, first, "Yes")
	require.False(t, closed)
	fynetest.RequireTextContains(t, deps.root(), "Open Project")
}

func TestRequestCloseDuringPristineWizardClosesImmediately(t *testing.T) {
	// Given an untouched wizard
	controller, deps := newTestApp(t)
	closed := false
	deps.window.SetOnClosed(func() { closed = true })
	controller.ActivateWizardView()

	// When
	controller.RequestClose()

	// Then no confirmation is needed.
	require.True(t, closed)
}

func TestNewProjectOverOpenProjectCancelsBackToClassifier(t *testing.T) {
	// Given an open project showing in the classifier
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "keep-me.deckcheck")
	createCSVProject(t, projectPath, "Keep row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()

	// When starting (then cancelling) a new project
	fileMenuItem(t, deps.window, "New Project").Action()
	fynetest.RequireTextContains(t, deps.root(), "Create new project")
	fynetest.TapButton(t, deps.root(), "Cancel")

	// Then the wizard returns to the still-open project, not the
	// launcher: the project survives a peek at New Project.
	fynetest.RequireTextContains(t, deps.root(), "Keep row")
	require.False(t, fileMenuItem(t, deps.window, "Close Project").Disabled)
}

func TestRequestCloseDuringWizardWithProjectReturnsToLauncher(t *testing.T) {
	// Given a pristine wizard opened over a project
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "behind-wizard.deckcheck")
	createCSVProject(t, projectPath, "Behind row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()
	fileMenuItem(t, deps.window, "New Project").Action()

	closed := false
	deps.window.SetOnClosed(func() { closed = true })

	// When the user asks to close the window
	controller.RequestClose()

	// Then the close downgrades: launcher shown, window still alive.
	require.False(t, closed)
	fynetest.RequireTextContains(t, deps.root(), "New Project")
	fynetest.RequireTextContains(t, deps.root(), "Open Project")

	// And the project really was released: a second close request now
	// takes the no-project branch and closes the window for real.
	controller.RequestClose()
	require.True(t, closed)
}

func TestCloseProjectMenuDuringDirtyWizardConfirmsFirst(t *testing.T) {
	// Given a dirty wizard over an open project
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "guarded.deckcheck")
	createCSVProject(t, projectPath, "Guarded row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()
	fileMenuItem(t, deps.window, "New Project").Action()
	fynetest.TypeEntry(t, deps.root(), "My Classification Project", "Half-typed")

	// When choosing File > Close Project
	fileMenuItem(t, deps.window, "Close Project").Action()

	// Then the wizard's confirmation gate fires before anything is
	// destroyed.
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay, "expected a confirm dialog before close")
	fynetest.RequireTextContains(t, overlay, "Discard wizard progress?")

	// When confirming
	fynetest.TapButton(t, overlay, "Yes")

	// Then the project closes and the launcher shows.
	fynetest.RequireTextContains(t, deps.root(), "Open Project")
}

func TestOpenRecentDuringDirtyWizardConfirmsFirst(t *testing.T) {
	// Given a dirty wizard and a remembered project
	recentPath := filepath.Join(t.TempDir(), "remembered.deckcheck")
	createCSVProject(t, recentPath, "Remembered row")
	controller, deps := newConfiguredTestApp(t, testAppConfig{recentPaths: []string{recentPath}})
	controller.ActivateWizardView()
	fynetest.TypeEntry(t, deps.root(), "My Classification Project", "Half-typed")

	// When choosing the recent project from the menu
	openRecent := fileMenuItem(t, deps.window, "Open Recent")
	childMenuItem(t, openRecent, "remembered.deckcheck").Action()

	// Then the wizard confirms before being replaced; declining keeps
	// the typed input on screen.
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay, "expected a confirm dialog before open")
	fynetest.TapButton(t, overlay, "No")
	fynetest.RequireEntryValue(t, deps.root(), "My Classification Project", "Half-typed")
}

func TestRequestCloseWithoutProjectClosesWindow(t *testing.T) {
	// Given the launcher with no project open
	controller, deps := newTestApp(t)
	closed := false
	deps.window.SetOnClosed(func() {
		closed = true
	})
	controller.ActivateWelcomeView()

	// When
	controller.RequestClose()

	// Then the window closes for real.
	require.True(t, closed)
}

func TestFileMenuNewProjectOpensWizard(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()

	// When
	fileMenuItem(t, deps.window, "New Project").Action()

	// Then
	fynetest.RequireTextContains(t, deps.root(), "Create new project")
}

func TestFileMenuCloseProjectStartsDisabled(t *testing.T) {
	// Given no project open
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()

	// Then the destructive-looking item cannot misfire.
	require.True(t, fileMenuItem(t, deps.window, "Close Project").Disabled)
}

func TestFileMenuCloseProjectClosesProjectAndReturnsToWelcome(t *testing.T) {
	// Given an open project, loaded through the same path production
	// uses so the menu has refreshed with the project set
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "close-me.deckcheck")
	createCSVProject(t, projectPath, "Close row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()

	item := fileMenuItem(t, deps.window, "Close Project")
	require.False(t, item.Disabled)

	// When
	item.Action()

	// Then the launcher is shown again.
	fynetest.RequireTextContains(t, deps.root(), "New Project")
	fynetest.RequireTextContains(t, deps.root(), "Open Project")
}

func TestFileMenuExportStartsDisabled(t *testing.T) {
	// Given the launcher, which has nothing to export
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()

	// Then
	require.True(t, fileMenuItem(t, deps.window, "Export CSV…").Disabled)
}

func TestFileMenuExportEnabledWithOpenProject(t *testing.T) {
	// Given an open project showing in the classifier
	controller, deps := newTestApp(t)
	projectPath := filepath.Join(t.TempDir(), "exportable.deckcheck")
	createCSVProject(t, projectPath, "Export row")
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()

	// Then
	require.False(t, fileMenuItem(t, deps.window, "Export CSV…").Disabled)
}

func TestOpenRecentClearMenuEmptiesRecents(t *testing.T) {
	// Given a recents store with an entry whose file exists (the store
	// prunes missing paths)
	projectPath := filepath.Join(t.TempDir(), "remembered.deckcheck")
	require.NoError(t, os.WriteFile(projectPath, []byte("stub"), 0o644))
	_, deps := newConfiguredTestApp(t, testAppConfig{recentPaths: []string{projectPath}})

	openRecent := fileMenuItem(t, deps.window, "Open Recent")
	clear := childMenuItem(t, openRecent, "Clear Menu")

	// When
	clear.Action()

	// Then the store is empty.
	require.Empty(t, deps.app.Preferences().StringList(recent.DefaultPreferencesKey))
}

func TestHelpMenuAboutShowsDialog(t *testing.T) {
	// Given
	_, deps := newTestApp(t)

	// When
	menuItem(t, deps.window, "Help", "About DeckCheck").Action()

	// Then the about dialog is on the overlay stack.
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	fynetest.RequireTextContains(t, overlay, "DeckCheck")
	fynetest.RequireTextContains(t, overlay, "Local-first dataset classification.")
}

func TestFileMenuOpenProjectOpensSelectedProject(t *testing.T) {
	// Given a real project file and a picker that returns it
	projectPath := filepath.Join(t.TempDir(), "menu.deckcheck")
	createCSVProject(t, projectPath, "Menu row")

	projectPicker := rootmocks.NewBrowsePicker(t)
	projectPicker.EXPECT().
		Open(mock.Anything, mock.Anything).
		Run(func(_ browse.OpenOptions, onSelected func(string)) {
			onSelected(projectPath)
		}).
		Return()

	controller, deps := newTestAppWithProjectPicker(t, projectPicker)
	controller.ActivateWelcomeView()

	// When
	fileMenuItem(t, deps.window, "Open Project…").Action()
	controller.WaitForPendingOperations()

	// Then the project opens into the classifier.
	fynetest.RequireTextContains(t, deps.root(), "Export CSV")
	fynetest.RequireTextContains(t, deps.root(), "Menu row")
}

func TestRunShowsWelcomeView(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)

	// When the app runs (the test driver's event loop returns immediately)
	controller.Run()

	// Then the launcher is showing.
	fynetest.RequireTextContains(t, deps.root(), "New Project")
	fynetest.RequireTextContains(t, deps.root(), "Open Project")
}

func TestActivateClassifierViewWrapsUncodedActivationFailure(t *testing.T) {
	// Given a project whose record count probe fails without a support code
	controller, _ := newTestApp(t)
	brokenProject := mocks.NewAppProject(t)
	brokenProject.EXPECT().Questions(mock.Anything).Return(defaultQuestions(), nil).Once()
	brokenProject.EXPECT().RecordCount(mock.Anything).Return(0, errors.New("boom")).Once()

	// When
	err := controller.ActivateClassifierView(brokenProject)

	// Then the failure is coded as a classifier-initialisation error.
	require.ErrorIs(t, err, usererror.ErrInitializeClassifier)
}

func TestActivateClassifierViewKeepsCodedActivationFailure(t *testing.T) {
	// Given a project whose first-unclassified scan fails with its own code
	controller, _ := newTestApp(t)
	brokenProject := mocks.NewAppProject(t)
	brokenProject.EXPECT().Questions(mock.Anything).Return(defaultQuestions(), nil).Once()
	brokenProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	brokenProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, false, errors.New("boom")).Once()

	// When
	err := controller.ActivateClassifierView(brokenProject)

	// Then the precise inner code wins over the catch-all wrap.
	require.ErrorIs(t, err, usererror.ErrFindFirstUnclassifiedRecord)
	require.NotErrorIs(t, err, usererror.ErrInitializeClassifier)
}

func TestOpenProjectShowsErrorWhenActivationFails(t *testing.T) {
	// Given a project file whose record data can no longer be decoded
	projectPath := filepath.Join(t.TempDir(), "damaged.deckcheck")
	createCSVProject(t, projectPath, "Damaged row")
	corruptRecordData(t, projectPath)

	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()
	existingRoot := deps.root()

	// When
	controller.OpenProject(projectPath)
	controller.WaitForPendingOperations()

	// Then the launcher stays and the failure is reported in a dialog.
	require.Same(t, existingRoot, deps.root())
	require.NotNil(t, deps.window.Canvas().Overlays().Top())
}

func TestShowErrorIgnoresNilErrors(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)

	// When
	controller.ShowError(nil)

	// Then no dialog is shown.
	require.Nil(t, deps.window.Canvas().Overlays().Top())
}

func TestShowInformationDisplaysDialog(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)

	// When
	controller.ShowInformation("Export Complete", "Exported 2 rows")

	// Then the dialog carries the message.
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	fynetest.RequireTextContains(t, overlay, "Exported 2 rows")
}

func TestShowConfirmDeliversUserChoice(t *testing.T) {
	// Given a confirm dialog awaiting a decision
	controller, deps := newTestApp(t)
	var confirmed *bool
	controller.ShowConfirm("Discard changes?", "Unsaved input will be lost", func(choice bool) {
		confirmed = &choice
	})
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)

	// When the user confirms
	fynetest.TapButton(t, overlay, "Yes")

	// Then the response callback receives the choice.
	require.NotNil(t, confirmed)
	require.True(t, *confirmed)
}

func TestTypedKeysAreIgnoredWithoutKeyAwareView(t *testing.T) {
	// Given the launcher, which does not consume raw key events
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()

	// When / Then typing a navigation key is a no-op.
	require.NotPanics(t, func() {
		deps.window.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyRight})
	})
}

func TestTypedKeysRouteToClassifierView(t *testing.T) {
	// Given a two-record project open in the classifier
	controller, deps := newTestApp(t)
	currentProject := mocks.NewAppProject(t)
	currentProject.EXPECT().Questions(mock.Anything).Return(defaultQuestions(), nil).Once()
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().
		Record(mock.Anything, 0).
		Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"text": "first"}}, nil).
		Once()
	currentProject.EXPECT().
		Record(mock.Anything, 1).
		Return(&project.Record{ID: 2, Index: 1, Data: map[string]string{"text": "second"}}, nil).
		Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 2, nil).Maybe()
	require.NoError(t, controller.ActivateClassifierView(currentProject))

	// When
	deps.window.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyRight})

	// Then the classifier advanced to the next record.
	fynetest.RequireTextContains(t, deps.root(), "second")
}

func TestHelpMenuShowsKeyboardShortcuts(t *testing.T) {
	// Given
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()

	// When
	menuItem(t, deps.window, "Help", "Keyboard Shortcuts").Action()

	// Then the shortcuts reference dialog is on screen.
	overlay := deps.window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	fynetest.RequireTextContains(t, overlay, "Previous record")
	fynetest.RequireTextContains(t, overlay, "Cmd/Ctrl+E")
}

func TestFileMenuOpenRecentOpensRememberedProject(t *testing.T) {
	// Given a project remembered in the recents store
	projectPath := filepath.Join(t.TempDir(), "recent.deckcheck")
	createCSVProject(t, projectPath, "Recent row")
	controller, deps := newConfiguredTestApp(t, testAppConfig{recentPaths: []string{projectPath}})

	recentMenu := fileMenuItem(t, deps.window, "Open Recent")
	require.NotNil(t, recentMenu.ChildMenu)
	// One remembered entry, then the separator and Clear Menu.
	require.Len(t, recentMenu.ChildMenu.Items, 3)
	require.Equal(t, "recent.deckcheck", recentMenu.ChildMenu.Items[0].Label)

	// When choosing the recent entry
	recentMenu.ChildMenu.Items[0].Action()
	controller.WaitForPendingOperations()

	// Then the project opens into the classifier.
	fynetest.RequireTextContains(t, deps.root(), "Recent row")
}

func TestFileMenuExportIsNoopWithoutExportableView(t *testing.T) {
	// Given the launcher, which cannot export
	controller, deps := newTestApp(t)
	controller.ActivateWelcomeView()
	existingRoot := deps.root()

	// When
	fileMenuItem(t, deps.window, "Export CSV…").Action()

	// Then nothing changes and no dialog appears.
	require.Same(t, existingRoot, deps.root())
	require.Nil(t, deps.window.Canvas().Overlays().Top())
}

func TestFileMenuExportDelegatesToClassifierView(t *testing.T) {
	// Given an open project and a picker primed for the export dialog
	picker := rootmocks.NewBrowsePicker(t)
	picker.EXPECT().
		Save(mock.Anything, mock.Anything).
		Run(func(options browse.SaveOptions, _ func(string)) {
			require.Equal(t, "Export CSV", options.Title)
		}).
		Return().
		Once()

	controller, deps := newConfiguredTestApp(t, testAppConfig{picker: picker})
	currentProject := expectProjectForClassifier(t, "exportable")
	require.NoError(t, controller.ActivateClassifierView(currentProject))
	fynetest.RequireTextContains(t, deps.root(), "exportable")

	// When
	fileMenuItem(t, deps.window, "Export CSV…").Action()

	// Then the classifier asked the picker for a destination (asserted by
	// the mock's Once expectation on Save).
}

func childMenuItem(t *testing.T, parent *fyne.MenuItem, label string) *fyne.MenuItem {
	t.Helper()

	require.NotNil(t, parent.ChildMenu, "menu item %q has no submenu", parent.Label)
	for _, item := range parent.ChildMenu.Items {
		if item.Label == label {
			return item
		}
	}

	t.Fatalf("submenu item %q not found under %q", label, parent.Label)
	return nil
}

func fileMenuItem(t *testing.T, window fyne.Window, label string) *fyne.MenuItem {
	t.Helper()

	return menuItem(t, window, "File", label)
}

func menuItem(t *testing.T, window fyne.Window, menuLabel, itemLabel string) *fyne.MenuItem {
	t.Helper()

	for _, menu := range window.MainMenu().Items {
		if menu.Label != menuLabel {
			continue
		}
		for _, item := range menu.Items {
			if item.Label == itemLabel {
				return item
			}
		}
	}

	t.Fatalf("menu item %q > %q not found", menuLabel, itemLabel)
	return nil
}

// corruptRecordData rewrites every imported row's data blob to a JSON
// value DuckDB accepts but the record decoder cannot unmarshal into a
// column map, so opening the file succeeds and the first record load
// fails during classifier activation.
func corruptRecordData(t *testing.T, projectPath string) {
	t.Helper()

	conn, err := sql.Open("duckdb", projectPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	_, err = conn.Exec("UPDATE dataset_rows SET original_data = '123'::JSON")
	require.NoError(t, err)
}

type testDeps struct {
	app     fyne.App
	window  fyne.Window
	content *fyne.Container
}

func (d testDeps) root() fyne.CanvasObject {
	if len(d.content.Objects) == 0 {
		return nil
	}

	return d.content.Objects[0]
}

func newTestApp(t *testing.T) (*deckapp.App, testDeps) {
	t.Helper()

	return newConfiguredTestApp(t, testAppConfig{})
}

func newTestAppWithProjectPicker(t *testing.T, projectPicker browse.Picker) (*deckapp.App, testDeps) {
	t.Helper()

	return newConfiguredTestApp(t, testAppConfig{projectPicker: projectPicker})
}

// testAppConfig parameterises newConfiguredTestApp. The zero value
// matches newTestApp: no pickers and an empty recents store.
type testAppConfig struct {
	picker        browse.Picker
	projectPicker browse.Picker
	recentPaths   []string
}

func newConfiguredTestApp(t *testing.T, cfg testAppConfig) (*deckapp.App, testDeps) {
	t.Helper()

	fyneApp := test.NewApp()
	// The default test theme defines no bold-monospace font, which the
	// keyboard-shortcuts dialog needs; use the production theme like
	// dependencies.New does.
	fyneApp.Settings().SetTheme(theme.New())
	if len(cfg.recentPaths) > 0 {
		// Seed the store before the controller builds its main menu so
		// the File > Open Recent submenu is populated from the start.
		fyneApp.Preferences().SetStringList(recent.DefaultPreferencesKey, cfg.recentPaths)
	}
	window := fyneApp.NewWindow("DeckCheck")
	content := container.NewStack()
	window.SetContent(content)

	controller := deckapp.New(deckapp.Config{
		App:           fyneApp,
		Window:        window,
		Content:       content,
		Picker:        cfg.picker,
		ProjectPicker: cfg.projectPicker,
		Recents:       recent.New(fyneApp.Preferences()),
	})

	// Release any project the test leaves open: Windows cannot remove
	// a still-open database file during TempDir cleanup.
	t.Cleanup(func() {
		controller.WaitForPendingOperations()
		controller.ForceCloseProject()
	})

	return controller, testDeps{
		app:     fyneApp,
		window:  window,
		content: content,
	}
}

func expectProjectForClassifier(t *testing.T, value string) *mocks.AppProject {
	t.Helper()

	currentProject := mocks.NewAppProject(t)
	currentProject.EXPECT().Questions(mock.Anything).Return(defaultQuestions(), nil).Once()
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().
		Record(mock.Anything, 0).
		Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"text": value}}, nil).
		Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	return currentProject
}

func defaultQuestions() []project.Question {
	return []project.Question{{
		ID:      1,
		Text:    "Valid?",
		Answers: []project.Answer{{ID: 10, Text: "Yes"}, {ID: 11, Text: "No"}},
	}}
}

func createCSVProject(t *testing.T, projectPath string, rowText string) {
	t.Helper()

	csvPath := filepath.Join(filepath.Dir(projectPath), filepath.Base(projectPath)+".csv")
	require.NoError(t, os.WriteFile(csvPath, []byte("text\n"+rowText+"\n"), 0o644))

	currentProject, err := projectfile.Create(context.Background(), projectPath, projectfile.CreateOptions{
		Name:        "Test project",
		DatasetType: dataset.TypeCSV,
		Questions: []project.QuestionDef{
			{Text: "Useful?", Answers: []string{"Yes", "No"}},
		},
		Source: dataset.NewCSV(csvPath),
	})
	require.NoError(t, err)
	require.NoError(t, currentProject.Close())
}
