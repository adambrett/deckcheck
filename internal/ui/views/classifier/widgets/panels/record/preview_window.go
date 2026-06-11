package record

import (
	"image"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

func (p *Panel) showImagePreview(path string) {
	app := fyne.CurrentApp()
	if app == nil || path == "" {
		return
	}

	size := previewWindowSize(path)
	var w fyne.Window
	img := newPreviewImage(func() {
		if w != nil {
			w.Close()
		}
	})
	img.SetFile(path)
	img.SetMinSize(size)

	w = app.NewWindow(filepath.Base(path))
	p.previews[w] = struct{}{}
	w.SetOnClosed(func() { delete(p.previews, w) })

	hint := widget.NewLabel(lang.L("Click image or press Esc to close"))
	hint.Alignment = fyne.TextAlignCenter

	w.SetContent(container.NewStack(
		canvas.NewRectangle(theme.Gray950),
		container.NewBorder(nil, hint, nil, nil, img),
	))
	w.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	w.Resize(size)
	w.CenterOnScreen()
	w.Show()
}

func previewWindowSize(path string) fyne.Size {
	width, height := imageDimensions(path)
	if width <= 0 || height <= 0 {
		return fyne.NewSize(900, 700)
	}

	size := fyne.NewSize(float32(width), float32(height))
	maxSize := fyne.NewSize(1400, 1000)
	if size.Width <= maxSize.Width && size.Height <= maxSize.Height {
		return size
	}

	scale := min(maxSize.Width/size.Width, maxSize.Height/size.Height)
	return fyne.NewSize(size.Width*scale, size.Height*scale)
}

func imageDimensions(path string) (int, int) {
	file, err := os.Open(path) //nolint:gosec // path comes from a user-selected image in the open project
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}

	return config.Width, config.Height
}
