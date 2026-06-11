package record

import (
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fyneui/layout"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
)

// Panel shows the current record's data and optional image.
type Panel struct {
	container *fyne.Container

	image      *previewImage
	imageBox   *fyne.Container
	dataList   *fyne.Container
	scrollable *container.Scroll

	// previews tracks the pop-out image windows so Close can tear
	// them down; otherwise they outlive the project and keep the
	// process alive after the main window closes.
	previews map[fyne.Window]struct{}

	placeholder *widget.Label

	imagePath string
}

// New creates a new record display panel.
func New() *Panel {
	p := &Panel{previews: make(map[fyne.Window]struct{})}

	p.image = newPreviewImage(func() {
		p.showImagePreview(p.imagePath)
	})
	p.image.SetMinSize(fyne.NewSize(220, 220))

	p.imageBox = container.NewStack(p.image)
	p.imageBox.Hide()

	p.dataList = container.NewVBox()
	p.scrollable = container.NewVScroll(p.dataList)
	p.scrollable.Hide()

	// Placeholder shown only between view activation and the first
	// LoadRecord call (or when the dataset is empty). The classifier's
	// status label carries the actionable "no records" message; this
	// text just stops the pane from looking blank during the swap.
	p.placeholder = widget.NewLabel(lang.L("Loading record…"))
	p.placeholder.Alignment = fyne.TextAlignCenter

	bg := canvas.NewRectangle(theme.Gray950)

	p.container = container.NewStack(
		bg,
		container.NewBorder(
			nil,
			layout.NewFixedHeight(190, container.NewPadded(p.scrollable)),
			nil,
			nil,
			p.imageBox,
		),
		container.NewCenter(p.placeholder),
	)

	return p
}

// Close closes any pop-out preview windows the panel has opened.
func (p *Panel) Close() {
	for w := range p.previews {
		w.Close()
	}
	p.previews = make(map[fyne.Window]struct{})
}

// Container returns the display's container.
func (p *Panel) Container() fyne.CanvasObject {
	return p.container
}

// SetRecord updates the display with a new record.
func (p *Panel) SetRecord(record *project.Record) {
	if record == nil {
		p.Clear()
		return
	}

	p.placeholder.Hide()
	p.scrollable.Show()

	if record.HasImage() && p.loadImage(record.ImagePath) {
		p.imageBox.Show()
	} else {
		p.imageBox.Hide()
		p.imagePath = ""
		p.image.SetFile("")
	}

	p.displayData(record.Data, record.ImagePath)
	p.container.Refresh()
}

// Clear clears the display.
func (p *Panel) Clear() {
	p.placeholder.Show()
	p.imageBox.Hide()
	p.dataList.Objects = nil
	p.scrollable.Hide()
	p.imagePath = ""
	p.image.SetFile("")
}

func (p *Panel) loadImage(path string) bool {
	// Any stat failure (missing, permission, dead mount) means the
	// image cannot render; fall back to the data table rather than
	// handing Fyne an unreadable path. The stat is a synchronous read
	// on the Fyne goroutine by the same doctrine as the classifier's
	// other reads (see the classifier package doc).
	if _, err := os.Stat(path); err != nil {
		p.imageBox.Hide()
		return false
	}

	if !dataset.IsImageFile(path) {
		p.imageBox.Hide()
		return false
	}

	p.imagePath = path
	p.image.SetFile(path)
	return true
}

func (p *Panel) displayData(data map[string]string, imagePath string) {
	p.dataList.Objects = nil

	if imagePath != "" {
		p.addTableRow(lang.L("Image"), filepath.Base(imagePath))
	}

	if len(data) == 0 && imagePath == "" {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		// Skip the image-folder synthetic column when we already have an
		// image path; the caller already rendered it as the "Image" row.
		if k == dataset.FilenameColumn && imagePath != "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]
		if value == "" {
			value = lang.L("(empty)")
		}
		p.addTableRow(key, value)
	}

	p.dataList.Refresh()
	p.scrollable.Refresh()
}

func (p *Panel) addTableRow(key, value string) {
	keyLabel := widget.NewLabel(key + ":")
	keyLabel.TextStyle = fyne.TextStyle{Bold: true}

	valueLabel := widget.NewLabel(value)
	valueLabel.Wrapping = fyne.TextWrapWord

	row := container.NewBorder(nil, nil, keyLabel, nil, valueLabel)
	p.dataList.Add(row)
	p.dataList.Add(widget.NewSeparator())
}
