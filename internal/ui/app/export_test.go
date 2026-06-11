package app

// WaitForPendingOperations blocks until every background operation
// started by the App has delivered its result back to the Fyne
// goroutine. Test-only: the export_test mechanism keeps it out of the
// production API surface.
func (a *App) WaitForPendingOperations() {
	a.life.Wait()
}
