package welcome

import "fyne.io/fyne/v2"

// Activate prepares the view for display.
func (v *View) Activate() error {
	return nil
}

// Container returns the view content.
func (v *View) Container() fyne.CanvasObject {
	return v.launcher.CanvasObject()
}

// Size returns the preferred welcome window size.
func (v *View) Size() fyne.Size {
	return v.launcher.Size()
}
