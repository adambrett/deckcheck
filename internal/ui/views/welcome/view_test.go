//go:build integration

package welcome_test

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/assets"
	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/ui/views/welcome"
)

func TestOpenButtonOpensSelectedProject(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	recents := recent.New(fyneApp.Preferences())

	var opened string
	picker := &fynetest.StubPicker{OpenPath: "project.deckcheck"}
	view := welcome.New(welcome.Config{
		Recents: recents,
		Icon:    testIcon(),
		Picker:  picker,
		Handlers: welcome.Handlers{
			Open: func(path string) {
				opened = path
			},
		},
	})

	// When
	root := layOutWelcomeForTap(view)
	fynetest.TapButton(t, root, "Open Project")

	// Then the selection reached the handler and the launcher supplied
	// a close hook for the dialog lifecycle.
	require.Equal(t, "project.deckcheck", opened)
	require.Len(t, picker.OpenCalls, 1)
	require.NotNil(t, picker.OpenCalls[0].OnClosed)
}

func TestCreateButtonStartsCreateFlow(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	recents := recent.New(fyneApp.Preferences())

	var created bool
	view := welcome.New(welcome.Config{
		Recents: recents,
		Icon:    testIcon(),
		Picker:  &fynetest.StubPicker{},
		Handlers: welcome.Handlers{
			Create: func() {
				created = true
			},
		},
	})

	// When
	root := layOutWelcomeForTap(view)
	fynetest.TapButton(t, root, "New Project")

	// Then
	require.True(t, created)
}

func TestNilHandlersDoNotPanic(t *testing.T) {
	// Given a welcome view with no callbacks wired
	a := test.NewApp()
	view := welcome.New(welcome.Config{
		Recents:  recent.New(a.Preferences()),
		Icon:     testIcon(),
		Picker:   &fynetest.StubPicker{},
		Handlers: welcome.Handlers{},
	})
	require.NoError(t, view.Activate())
	root := layOutWelcomeForTap(view)

	// When / Then tapping either action is a guarded no-op.
	require.NotPanics(t, func() {
		fynetest.TapButton(t, root, "New Project")
		fynetest.TapButton(t, root, "Open Project")
	})
}

func TestWelcomeUsesSharedRecents(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	projectPath := filepath.Join(t.TempDir(), "project.deckcheck")
	require.NoError(t, os.WriteFile(projectPath, []byte("deckcheck"), 0o644))

	recents := recent.New(fyneApp.Preferences())
	recents.Add(recent.Item{Path: projectPath})

	// When
	view := welcome.New(welcome.Config{
		Recents:  recents,
		Icon:     testIcon(),
		Picker:   &fynetest.StubPicker{},
		Handlers: welcome.Handlers{},
	})

	// Then
	require.NotNil(t, view.Container())
	require.Contains(t, fyneApp.Preferences().StringList(recent.DefaultPreferencesKey), projectPath)
}

func TestWelcomeLifecycle(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	view := welcome.New(welcome.Config{
		Recents:  recent.New(fyneApp.Preferences()),
		Icon:     testIcon(),
		Picker:   &fynetest.StubPicker{},
		Handlers: welcome.Handlers{},
	})

	// When
	err := view.Activate()

	// Then
	require.NoError(t, err)
	require.NotNil(t, view.Container())
	require.Equal(t, fyne.NewSize(720, 480), view.Size())
}

func layOutWelcomeForTap(v *welcome.View) fyne.CanvasObject {
	c := test.NewCanvas()
	c.SetContent(v.Container())
	c.Resize(fyne.NewSize(1024, 768))

	return v.Container()
}

func testIcon() fyne.Resource {
	return fyne.NewStaticResource("deckcheck.png", assets.Icon)
}
