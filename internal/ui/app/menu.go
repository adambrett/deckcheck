package app

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/go-fyne/pkg/browse"
	"github.com/adambrett/go-fyne/pkg/recent"
)

type exportView interface {
	Export()
}

// command pairs a Cmd/Ctrl shortcut with its action and the
// human-readable row the shortcuts dialog shows. One table drives
// both shortcut wiring and the dialog so the two cannot drift.
type command struct {
	key         fyne.KeyName
	label       string
	description string
	action      func()
}

// shortcutFor returns the platform Cmd/Ctrl shortcut for key.
func shortcutFor(key fyne.KeyName) fyne.Shortcut {
	return &desktop.CustomShortcut{
		KeyName:  key,
		Modifier: fyne.KeyModifierShortcutDefault,
	}
}

func (a *App) commands() []command {
	return []command{
		{key: fyne.KeyN, label: "Cmd/Ctrl+N", description: lang.L("New project"), action: a.ActivateWizardView},
		{key: fyne.KeyO, label: "Cmd/Ctrl+O", description: lang.L("Open project"), action: a.openProjectFromMenu},
		{key: fyne.KeyW, label: "Cmd/Ctrl+W", description: lang.L("Close project (or window)"), action: a.RequestClose},
		{key: fyne.KeyE, label: "Cmd/Ctrl+E", description: lang.L("Export CSV"), action: a.exportActiveView},
		{key: fyne.KeySlash, label: "Cmd/Ctrl+/", description: lang.L("Show this shortcuts list"), action: a.showKeyboardShortcuts},
	}
}

// installMainMenu builds the application menu. Keyboard shortcuts
// hang off the menu items rather than the canvas: Fyne routes canvas
// shortcuts to a focused Entry first (which swallows them), while
// menu-item shortcuts fire regardless of focus.
func (a *App) installMainMenu() {
	a.refreshMainMenu()
}

// refreshMainMenu rebuilds the main menu from the current state
// (notably the recents list) without re-registering the canvas
// shortcuts that were wired by installMainMenu.
func (a *App) refreshMainMenu() {
	newProject := fyne.NewMenuItem(lang.L("New Project"), a.ActivateWizardView)
	newProject.Shortcut = shortcutFor(fyne.KeyN)

	openProject := fyne.NewMenuItem(lang.L("Open Project…"), a.openProjectFromMenu)
	openProject.Shortcut = shortcutFor(fyne.KeyO)

	closeProject := a.closeProjectMenuItem()
	closeProject.Shortcut = shortcutFor(fyne.KeyW)

	exportCSV := a.exportMenuItem()
	exportCSV.Shortcut = shortcutFor(fyne.KeyE)

	shortcutsHelp := fyne.NewMenuItem(lang.L("Keyboard Shortcuts"), a.showKeyboardShortcuts)
	shortcutsHelp.Shortcut = shortcutFor(fyne.KeySlash)

	a.window.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu(lang.L("File"),
			newProject,
			openProject,
			a.openRecentMenuItem(),
			fyne.NewMenuItemSeparator(),
			closeProject,
			fyne.NewMenuItemSeparator(),
			exportCSV,
		),
		fyne.NewMenu(lang.L("Help"),
			shortcutsHelp,
			fyne.NewMenuItem(lang.L("About DeckCheck"), a.showAbout),
		),
	))
}

// exportMenuItem returns File > Export CSV, disabled while the active
// view has nothing to export, matching the affordance of the Close
// Project item rather than silently no-opping.
func (a *App) exportMenuItem() *fyne.MenuItem {
	item := fyne.NewMenuItem(lang.L("Export CSV…"), a.exportActiveView)
	_, exportable := a.active.(exportView)
	item.Disabled = !exportable

	return item
}

// closeProjectMenuItem returns File > Close Project, disabled while no
// project is open so the destructive-looking action cannot misfire
// from the launcher.
func (a *App) closeProjectMenuItem() *fyne.MenuItem {
	item := fyne.NewMenuItem(lang.L("Close Project"), func() {
		a.navigate(a.closeProjectToWelcome)
	})
	item.Disabled = a.project == nil

	return item
}

// openRecentMenuItem returns a File > Open Recent submenu populated
// from the recents store. Once a project is open the launcher screen
// is no longer reachable, so the menu is the only way back to recents
// for macOS-style "File > Open Recent" muscle memory.
func (a *App) openRecentMenuItem() *fyne.MenuItem {
	parent := fyne.NewMenuItem(lang.L("Open Recent"), nil)

	items := a.recents.Items()
	if len(items) == 0 {
		empty := fyne.NewMenuItem(lang.L("No recent projects"), nil)
		empty.Disabled = true
		parent.ChildMenu = fyne.NewMenu("", empty)
		return parent
	}

	children := make([]*fyne.MenuItem, 0, len(items)+2)
	for _, item := range items {
		path := item.Path
		children = append(children, fyne.NewMenuItem(item.DisplayName(), func() {
			a.navigate(func() { a.OpenProject(path) })
		}))
	}
	children = append(children,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(lang.L("Clear Menu"), a.clearRecentProjects),
	)
	parent.ChildMenu = fyne.NewMenu("", children...)

	return parent
}

// clearRecentProjects empties the recents store. The menu rebuild is
// deferred to the next event-loop tick: rebuilding the main menu while
// AppKit is still tracking the click corrupts native menu state.
func (a *App) clearRecentProjects() {
	a.recents.Replace(recent.Items{})
	fyne.Do(a.refreshMainMenu)
}

func (a *App) openProjectFromMenu() {
	a.navigate(a.showOpenProjectPicker)
}

// showOpenProjectPicker shows the open dialog. No options: the
// project picker wrapper owns the title and the .deckcheck filter.
func (a *App) showOpenProjectPicker() {
	a.projectPicker.Open(browse.OpenOptions{}, a.OpenProject)
}

func (a *App) exportActiveView() {
	view, ok := a.active.(exportView)
	if !ok {
		return
	}

	view.Export()
}

// repoURL is shown in the About dialog as the project home.
const repoURL = "https://github.com/adambrett/deckcheck"

// showAbout opens a small dialog with the app identity: icon, name,
// version (when the build carries one), and a link to the repository.
func (a *App) showAbout() {
	name := widget.NewLabelWithStyle("DeckCheck", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	tagline := widget.NewLabelWithStyle(
		lang.L("Local-first dataset classification."), fyne.TextAlignCenter, fyne.TextStyle{},
	)

	rows := []fyne.CanvasObject{name, tagline}

	if version := a.app.Metadata().Version; version != "" {
		rows = append(rows, widget.NewLabelWithStyle("v"+version, fyne.TextAlignCenter, fyne.TextStyle{}))
	}

	if home, err := url.Parse(repoURL); err == nil {
		link := widget.NewHyperlink("github.com/adambrett/deckcheck", home)
		link.Alignment = fyne.TextAlignCenter
		rows = append(rows, link)
	}

	content := container.NewVBox(rows...)
	if a.icon != nil {
		logo := canvas.NewImageFromResource(a.icon)
		logo.FillMode = canvas.ImageFillContain
		logo.SetMinSize(fyne.NewSize(64, 64))
		content = container.NewVBox(append([]fyne.CanvasObject{logo}, rows...)...)
	}

	dialog.NewCustom(lang.L("About DeckCheck"), lang.L("Close"), container.NewPadded(content), a.window).Show()
}

// showKeyboardShortcuts opens a modal listing every keyboard shortcut
// the app responds to. Shortcuts are otherwise only discoverable via
// the answer cards' inline pills, so this is the canonical reference.
func (a *App) showKeyboardShortcuts() {
	type row struct {
		key, action string
	}

	rows := []row{
		{"1 - 9", lang.L("Select the matching answer for the active question")},
		{"Left", lang.L("Previous record")},
		{"Right", lang.L("Skip to the next record")},
	}
	for _, cmd := range a.commands() {
		rows = append(rows, row{cmd.label, cmd.description})
	}

	lines := make([]fyne.CanvasObject, 0, len(rows))
	for _, r := range rows {
		key := widget.NewLabelWithStyle(r.key, fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
		action := widget.NewLabel(r.action)
		lines = append(lines, container.NewBorder(nil, nil, key, nil, action))
	}

	content := container.NewPadded(container.NewVBox(lines...))
	dialog.NewCustom(lang.L("Keyboard Shortcuts"), lang.L("Close"), content, a.window).Show()
}
