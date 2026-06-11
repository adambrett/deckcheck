package app

import "github.com/adambrett/deckcheck/internal/projectfile"

// WaitForPendingOperations blocks until every background operation
// started by the App has delivered its result back to the Fyne
// goroutine. Test-only: the export_test mechanism keeps it out of the
// production API surface.
func (a *App) WaitForPendingOperations() {
	a.life.Wait()
}

// ForceCloseProject synchronously closes any open project file,
// bypassing confirmation guards. Test-only: Windows cannot delete a
// still-open database file during TempDir cleanup, so fixtures
// release the project before the test ends. Mock projects are
// dropped without Close so their expectations stay untouched.
func (a *App) ForceCloseProject() {
	a.closeActiveView()

	p := a.project
	a.project = nil
	if file, ok := p.(*projectfile.Project); ok {
		_ = file.Close()
	}
}
