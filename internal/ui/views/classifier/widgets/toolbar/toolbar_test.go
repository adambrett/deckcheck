//go:build integration

package toolbar_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/toolbar"
)

func TestToolbarStartsWithControlsDisabled(t *testing.T) {
	// Given
	test.NewApp()

	// When
	bar := toolbar.New(toolbar.Handlers{})

	// Then every action is disabled and no progress shows.
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Previous").Disabled())
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Skip").Disabled())
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Export CSV").Disabled())
	require.Empty(t, firstLabel(t, bar.Container()).Text)
}

func TestToolbarEnableTogglesControlsAndProgress(t *testing.T) {
	// Given
	test.NewApp()
	bar := toolbar.New(toolbar.Handlers{})

	// When
	bar.EnablePrevious(true)
	bar.EnableSkip(true)
	bar.EnableExport(true)
	bar.SetProgress(3, 10)

	// Then
	require.False(t, fynetest.ButtonWithText(t, bar.Container(), "Previous").Disabled())
	require.False(t, fynetest.ButtonWithText(t, bar.Container(), "Skip").Disabled())
	require.False(t, fynetest.ButtonWithText(t, bar.Container(), "Export CSV").Disabled())
	require.Equal(t, "3 / 10 classified", firstLabel(t, bar.Container()).Text)

	// When disabling again
	bar.EnablePrevious(false)
	bar.EnableSkip(false)
	bar.EnableExport(false)
	bar.SetProgress(0, 0)

	// Then
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Previous").Disabled())
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Skip").Disabled())
	require.True(t, fynetest.ButtonWithText(t, bar.Container(), "Export CSV").Disabled())
	require.Empty(t, firstLabel(t, bar.Container()).Text)
}

func TestToolbarRoutesTapsToHandlers(t *testing.T) {
	// Given an enabled toolbar with capturing handlers
	test.NewApp()

	previousCalled := false
	skipCalled := false
	exportCalled := false
	unclassifiedOnly := false
	bar := toolbar.New(toolbar.Handlers{
		Previous: func() {
			previousCalled = true
		},
		Skip: func() {
			skipCalled = true
		},
		Export: func() {
			exportCalled = true
		},
		UnclassifiedOnlyChanged: func(enabled bool) {
			unclassifiedOnly = enabled
		},
	})
	bar.EnablePrevious(true)
	bar.EnableSkip(true)
	bar.EnableExport(true)

	// When
	test.Tap(fynetest.ButtonWithText(t, bar.Container(), "Previous"))
	test.Tap(fynetest.ButtonWithText(t, bar.Container(), "Skip"))
	test.Tap(fynetest.ButtonWithText(t, bar.Container(), "Export CSV"))
	fynetest.CheckWithText(t, bar.Container(), "Unclassified only").SetChecked(true)

	// Then every handler fired.
	require.True(t, previousCalled)
	require.True(t, skipCalled)
	require.True(t, exportCalled)
	require.True(t, unclassifiedOnly)
}

func firstLabel(t *testing.T, root fyne.CanvasObject) *widget.Label {
	t.Helper()

	var found *widget.Label
	fynetest.Walk(root, func(obj fyne.CanvasObject) {
		label, ok := obj.(*widget.Label)
		if ok && found == nil {
			found = label
		}
	})
	require.NotNil(t, found)

	return found
}
