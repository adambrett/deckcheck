package toolbar

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

// Toolbar contains classifier navigation and export controls.
type Toolbar struct {
	container *fyne.Container

	prevBtn       *widget.Button
	skipBtn       *widget.Button
	exportBtn     *widget.Button
	unclassified  *widget.Check
	progressLabel *widget.Label

	handlers Handlers
}

// Handlers bundles the toolbar's button callbacks. Any field may be
// nil; the toolbar guards before invoking.
type Handlers struct {
	Previous                func()
	Skip                    func()
	Export                  func()
	UnclassifiedOnlyChanged func(enabled bool)
}

// New creates a new toolbar instance.
func New(handlers Handlers) *Toolbar {
	t := &Toolbar{handlers: handlers}

	t.prevBtn = widget.NewButtonWithIcon(lang.L("Previous"), fyneTheme.NavigateBackIcon(), func() {
		if t.handlers.Previous != nil {
			t.handlers.Previous()
		}
	})
	t.prevBtn.Disable()

	t.skipBtn = widget.NewButtonWithIcon(lang.L("Skip"), fyneTheme.NavigateNextIcon(), func() {
		if t.handlers.Skip != nil {
			t.handlers.Skip()
		}
	})
	t.skipBtn.Disable()

	t.exportBtn = widget.NewButtonWithIcon(lang.L("Export CSV"), fyneTheme.DocumentSaveIcon(), func() {
		if t.handlers.Export != nil {
			t.handlers.Export()
		}
	})
	t.exportBtn.Disable()

	t.unclassified = widget.NewCheck(lang.L("Unclassified only"), func(enabled bool) {
		if t.handlers.UnclassifiedOnlyChanged != nil {
			t.handlers.UnclassifiedOnlyChanged(enabled)
		}
	})

	// Progress label
	t.progressLabel = widget.NewLabel("")
	t.progressLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Previous and Skip are secondary navigation; Export CSV is the
	// terminal "I'm done" action and gets primary emphasis so the
	// visual hierarchy reads correctly.
	t.prevBtn.Importance = widget.LowImportance
	t.skipBtn.Importance = widget.LowImportance
	t.exportBtn.Importance = widget.HighImportance

	// Progress is data, not a command, so it sits apart from the
	// command group containing the Export button. The spacer between
	// the unclassified toggle and the progress label lets the count
	// drift right as the window grows while keeping it visually
	// distinct from the export action.
	content := container.NewHBox(
		commandGroup(t.prevBtn, t.skipBtn),
		commandGroup(t.unclassified),
		layout.NewSpacer(),
		t.progressLabel,
		commandGroup(t.exportBtn),
	)

	bg := canvas.NewRectangle(theme.Gray900)
	divider := canvas.NewRectangle(theme.Gray600)

	t.container = container.NewStack(
		bg,
		container.NewBorder(
			divider,
			nil,
			nil,
			nil,
			container.NewPadded(content),
		),
	)

	return t
}

// Container returns the toolbar's container.
func (t *Toolbar) Container() fyne.CanvasObject {
	return t.container
}

// EnablePrevious enables or disables the previous button.
func (t *Toolbar) EnablePrevious(enabled bool) {
	if enabled {
		t.prevBtn.Enable()
	} else {
		t.prevBtn.Disable()
	}
}

// EnableSkip enables or disables the skip button.
func (t *Toolbar) EnableSkip(enabled bool) {
	if enabled {
		t.skipBtn.Enable()
	} else {
		t.skipBtn.Disable()
	}
}

// SetProgress sets the progress label from classified/total counts.
func (t *Toolbar) SetProgress(classified, total int) {
	if total == 0 {
		t.progressLabel.SetText("")
		return
	}
	t.progressLabel.SetText(fmt.Sprintf(lang.L("%d / %d classified"), classified, total))
}

// EnableExport enables or disables the export button.
func (t *Toolbar) EnableExport(enabled bool) {
	if enabled {
		t.exportBtn.Enable()
	} else {
		t.exportBtn.Disable()
	}
}

// commandGroup wraps a set of related toolbar widgets in an HBox so
// they read as a single unit. Earlier revisions prefixed each group
// with a yellow bullet for visual separation, but with three groups
// the eye read three identical dots as either bullet points or status
// indicators (neither role applied), so the bullet was dropped.
func commandGroup(objects ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(objects...)
}
