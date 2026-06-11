package app

import (
	"fyne.io/fyne/v2/dialog"

	"github.com/adambrett/deckcheck/internal/fyneui/errordialog"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// ShowError renders err as a structured error dialog. It is the single
// UI error sink: every view error callback ultimately funnels here.
func (a *App) ShowError(err error) {
	if err == nil {
		return
	}

	errordialog.Show(a.window, a.app.Clipboard(), usererror.ForError(err))
}

// ShowInformation displays a plain informational dialog.
func (a *App) ShowInformation(title, message string) {
	dialog.ShowInformation(title, message, a.window)
}

// ShowConfirm displays a yes/no dialog and invokes response with the
// user's choice. Intended for destructive flows (e.g. wizard cancel
// with unsaved input).
func (a *App) ShowConfirm(title, message string, response func(confirmed bool)) {
	dialog.ShowConfirm(title, message, response, a.window)
}
