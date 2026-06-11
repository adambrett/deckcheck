package app

import (
	"context"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// runInBackground executes work off the Fyne goroutine while a modal
// progress dialog blocks further input, then runs the apply step work
// returns back on the Fyne goroutine. work must not touch any widget;
// everything UI-bound belongs in the apply step. This is what keeps
// multi-second project loads and dataset imports from freezing the
// window.
func (a *App) runInBackground(message string, work func(ctx context.Context) func()) {
	progress := dialog.NewCustomWithoutButtons(message, widget.NewProgressBarInfinite(), a.window)
	progress.Show()

	a.life.Go(func(ctx context.Context) func() {
		apply := work(ctx)

		return func() {
			progress.Hide()
			apply()
		}
	})
}
