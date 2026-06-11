package dependencies

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/adambrett/go-fyne/pkg/browse"
	nativebrowse "github.com/adambrett/go-fyne/pkg/browse/native"
	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/assets"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/picker"
	"github.com/adambrett/deckcheck/internal/translations"
)

const (
	appID   = "dev.adbr.deckcheck"
	appName = "DeckCheck"
)

// Dependencies holds the top-level runtime objects built during startup.
type Dependencies struct {
	App           fyne.App
	Window        fyne.Window
	Content       *fyne.Container
	Icon          fyne.Resource
	Picker        browse.Picker
	ProjectPicker browse.Picker
	Recents       *recent.Recent
}

// New wires the Fyne app, main window, root content stack, and the file
// pickers used by the wizard and the classifier views.
func New() (*Dependencies, error) {
	if err := translations.Register(); err != nil {
		return nil, err
	}

	fyneApp := app.NewWithID(appID)
	fyneApp.Settings().SetTheme(theme.New())

	window := fyneApp.NewWindow(appName)

	content := container.NewStack()
	window.SetContent(content)

	icon := fyne.NewStaticResource("icon.png", assets.Icon)
	window.SetIcon(icon)

	filePicker := nativebrowse.New(window)

	projectPicker, err := picker.NewProject(filePicker)
	if err != nil {
		return nil, fmt.Errorf("build project picker: %w", err)
	}

	return &Dependencies{
		App:           fyneApp,
		Window:        window,
		Content:       content,
		Icon:          icon,
		Picker:        filePicker,
		ProjectPicker: projectPicker,
		Recents:       recent.New(fyneApp.Preferences()),
	}, nil
}
