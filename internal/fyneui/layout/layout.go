package layout

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// New creates the main application layout with content filling the window and a bottom action bar.
func New(toolbar, content fyne.CanvasObject) *fyne.Container {
	return container.NewBorder(nil, toolbar, nil, nil, content)
}

// NewFixedHeight pins an object to a predictable vertical size while allowing
// it to fill the available width.
func NewFixedHeight(height float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(&fixedHeightLayout{height: height}, obj)
}

// NewFixedWidth pins an object to a predictable width while letting
// its height grow with content. Used to constrain widget.Label
// wrapping inside dialogs without locking the dialog's vertical size.
func NewFixedWidth(width float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(&fixedWidthLayout{width: width}, obj)
}

type fixedHeightLayout struct {
	height float32
}

func (l *fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		if size := obj.MinSize(); size.Width > width {
			width = size.Width
		}
	}
	return fyne.NewSize(width, l.height)
}

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		obj.Move(fyne.NewPos(0, 0))
		obj.Resize(size)
	}
}

type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		// Wrapped-label height depends on width, so probe each child's
		// height at the pinned width by resizing before measuring. The
		// resize is idempotent for static dialog content; it is the
		// only way to learn a label's wrapped line count in Fyne.
		obj.Resize(fyne.NewSize(l.width, obj.MinSize().Height))
		if size := obj.MinSize(); size.Height > height {
			height = size.Height
		}
	}

	return fyne.NewSize(l.width, height)
}

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		obj.Move(fyne.NewPos(0, 0))
		obj.Resize(fyne.NewSize(l.width, size.Height))
	}
}
