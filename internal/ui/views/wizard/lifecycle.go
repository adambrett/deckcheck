package wizard

import (
	"fyne.io/fyne/v2"
)

// Activate prepares the wizard for display, establishing a fresh
// per-activation context and resetting all steps.
func (v *View) Activate() error {
	v.life.Activate()
	v.Reset()
	return nil
}

// Close cancels any in-flight wizard IO (currently the CSV header
// probe) so backing out of the wizard does not strand a goroutine
// waiting on a slow disk.
func (v *View) Close() {
	v.life.Close()
}

// Container returns the wizard's content.
func (v *View) Container() fyne.CanvasObject {
	return v.container
}

// Size returns the preferred wizard window size.
func (v *View) Size() fyne.Size {
	return fyne.NewSize(viewWidth, viewHeight)
}

// Reset resets the wizard to the first step.
func (v *View) Reset() {
	v.currentStep = 0
	for _, step := range v.steps {
		step.Reset()
	}
	v.updateUI()
}
