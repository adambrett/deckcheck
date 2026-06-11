package wizard

// WaitForPendingOperations blocks until every background operation
// started by the wizard has delivered its result back to the Fyne
// goroutine. Test-only: the export_test mechanism keeps it out of the
// production API surface.
func (v *View) WaitForPendingOperations() {
	v.life.Wait()
}
