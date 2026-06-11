package app

import (
	"fyne.io/fyne/v2"

	"github.com/adambrett/deckcheck/internal/ui/views"
)

// keyHandler is satisfied by views that consume raw key events.
type keyHandler interface {
	HandleKey(*fyne.KeyEvent)
}

func (a *App) handleKeyEvent(key *fyne.KeyEvent) {
	view, ok := a.active.(keyHandler)
	if !ok {
		return
	}

	view.HandleKey(key)
}

// RequestClose applies the same close behaviour as the window close intercept.
func (a *App) RequestClose() {
	a.handleClose()
}

// handleClose is the window close intercept. Views holding unsaved
// input (the wizard) get to confirm first; then, with a project open,
// the close downgrades to "close project, back to the launcher", and
// with nothing open it quits for real.
func (a *App) handleClose() {
	a.navigate(a.performClose)
}

// navigate is the single choke point for tearing down the active
// view: if it holds unsaved input (views.CloseGuard) it confirms with
// the user first, then to runs. Every action that replaces or closes
// the active view must come through here so no path can silently
// destroy typed input.
//
// While a confirmation is showing, further navigations are dropped:
// Fyne dispatches canvas shortcuts even with a dialog open, so
// without the guard a second Cmd+W could stack a second confirm and
// double-run the close.
func (a *App) navigate(to func()) {
	if a.confirming {
		return
	}

	if guard, ok := a.active.(views.CloseGuard); ok {
		a.confirming = true
		guard.ConfirmClose(func(proceed bool) {
			a.confirming = false
			if proceed {
				to()
			}
		})
		return
	}

	to()
}

func (a *App) performClose() {
	if a.project != nil {
		a.closeProjectToWelcome()
		return
	}

	a.quit()
}

func (a *App) closeProjectToWelcome() {
	outgoing := a.active
	a.closeActiveView()
	a.releaseProject(outgoing)
	a.ActivateWelcomeView()
}

// releaseProject closes the open project, first letting the outgoing
// view's background writes drain (off the Fyne goroutine) so a
// classification the user already saw confirmed cannot be cut off by
// the database closing underneath it.
func (a *App) releaseProject(outgoing views.View) {
	p := a.project
	a.project = nil
	if p == nil {
		return
	}

	release := func() { _ = p.Close() }
	if quiescer, ok := outgoing.(views.Quiescer); ok {
		quiescer.Quiesce(release)
		return
	}
	release()
}

func (a *App) quit() {
	a.closeActiveView()

	// The window is going away for good: cancel the app-scoped
	// context so any direct App-launched IO (project open or create
	// in flight) is interrupted rather than racing the window
	// teardown, and drop any background completion that would
	// otherwise land on the dead event loop.
	a.life.Close()
	a.window.SetCloseIntercept(nil)
	a.window.Close()
}

func (a *App) closeActiveView() {
	if closer, ok := a.active.(views.Closer); ok {
		closer.Close()
	}
	a.active = nil
}
