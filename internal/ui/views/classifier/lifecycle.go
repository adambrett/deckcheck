package classifier

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/deckcheck/internal/usererror"
)

// Container returns the view's content.
func (v *View) Container() fyne.CanvasObject {
	return v.container
}

// Size returns the preferred classifier window size.
func (v *View) Size() fyne.Size {
	return fyne.NewSize(viewWidth, viewHeight)
}

// Activate loads initial data and displays the first record. It also
// establishes the view-scoped context that every subsequent UI call
// site uses for domain operations; cancelling that context (via
// [View.Close]) interrupts any in-flight DuckDB query so closing the
// window mid-load does not leak work.
func (v *View) Activate() error {
	v.life.Activate()

	count, err := v.project.RecordCount(v.ctx())
	if err != nil {
		return err
	}
	v.totalRecords = count

	if v.totalRecords == 0 {
		v.toolbar.EnableExport(false)
		v.statusLabel.SetText(lang.L("No records to classify"))
		v.updateNavigation()
		return nil
	}

	startIndex, found, err := v.project.NextUnclassified(v.ctx(), 0)
	if err != nil {
		return usererror.Wrap("DC16", usererror.ErrFindFirstUnclassifiedRecord, err)
	}
	allClassified := !found
	if !found {
		startIndex = 0
	}

	if err := v.LoadRecord(startIndex); err != nil {
		return err
	}

	if allClassified {
		// LoadRecord clears the status label; restore the
		// "all done" hint after it has run.
		v.statusLabel.SetText(lang.L("All records classified"))
	}

	v.toolbar.EnableExport(true)
	return nil
}

// Close releases any background work owned by the view: the
// auto-advance timer, open preview windows, and the per-view context
// that scopes every database/IO call. Cancelling the context
// interrupts any in-flight domain operation so closing the window
// mid-query does not strand a DuckDB call. Close is safe to call
// multiple times; like the lifecycle it delegates to, it must run on
// the Fyne goroutine.
func (v *View) Close() {
	v.life.Close()
	v.cancelPendingAdvance()
	v.recordDisplay.Close()
}

// Quiesce implements [views.Quiescer]: then runs on a fresh goroutine
// once every background operation (including detached classification
// writes) has drained.
func (v *View) Quiesce(then func()) {
	go func() {
		v.life.Wait()
		then()
	}()
}

// ctx returns the current activation's context for domain calls.
func (v *View) ctx() context.Context {
	return v.life.Context()
}
