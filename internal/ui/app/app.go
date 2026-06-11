package app

import (
	"context"

	"fyne.io/fyne/v2"

	"github.com/adambrett/go-fyne/pkg/browse"
	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/export"
	"github.com/adambrett/deckcheck/internal/ui/views"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier"
	"github.com/adambrett/deckcheck/internal/ui/views/lifecycle"
)

// Project is the App's grip on an open project: everything the
// classifier view consumes, the slice the CSV exporter streams from,
// and Close for the single-open-project lifecycle the App enforces.
// *projectfile.Project satisfies it in production.
type Project interface {
	classifier.Project
	export.Project
	Close() error
}

// App is the DeckCheck window controller. Exactly one instance exists
// per Fyne window for the lifetime of the program.
type App struct {
	app    fyne.App
	window fyne.Window

	content *fyne.Container

	project Project

	// life carries the application-scoped context cancelled when the
	// window closes. Operations launched directly by the App (project
	// open or create that runs before the classifier view exists) use
	// it so a window-close request mid-load is honoured; it also drops
	// late runInBackground completions once the window is gone.
	life *lifecycle.Lifecycle

	icon          fyne.Resource
	picker        browse.Picker
	projectPicker browse.Picker
	recents       *recent.Recent

	active views.View

	// confirming is set while a CloseGuard confirmation dialog is on
	// screen; navigate drops re-entrant calls until it resolves.
	confirming bool
}

// Config holds the runtime dependencies App needs from the outside world.
type Config struct {
	App           fyne.App
	Window        fyne.Window
	Content       *fyne.Container
	Icon          fyne.Resource
	Picker        browse.Picker
	ProjectPicker browse.Picker
	Recents       *recent.Recent
}

// New constructs the DeckCheck window controller from its dependencies.
func New(cfg Config) *App {
	a := &App{
		app:           cfg.App,
		window:        cfg.Window,
		content:       cfg.Content,
		life:          lifecycle.New(context.Background()),
		icon:          cfg.Icon,
		picker:        cfg.Picker,
		projectPicker: cfg.ProjectPicker,
		recents:       cfg.Recents,
	}
	a.life.Activate()

	a.window.SetCloseIntercept(a.handleClose)
	a.window.Canvas().SetOnTypedKey(a.handleKeyEvent)
	a.installMainMenu()

	// macOS Cmd+Q / dock Quit stops the driver without ever hitting
	// the window close intercept; release the project file on the way
	// out so the database closes cleanly.
	a.app.Lifecycle().SetOnStopped(a.closeProject)

	return a
}

// Run shows the welcome view and enters the Fyne event loop.
func (a *App) Run() {
	a.ActivateWelcomeView()
	a.window.Show()
	a.app.Run()
}

// ctx returns the application-scoped context for domain calls.
func (a *App) ctx() context.Context {
	return a.life.Context()
}
