package answers

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

// shortcutPillTextSize is the pill's text size; it doubles as the
// marker tests use to tell pill text apart from label fragments.
const shortcutPillTextSize = 22

type answerOption struct {
	widget.BaseWidget

	shortcut string
	text     string

	selected bool
	hovered  bool

	onTapped func()
}

func (o *answerOption) Tapped(_ *fyne.PointEvent) {
	if o.onTapped != nil {
		o.onTapped()
	}
}

func (o *answerOption) TappedSecondary(_ *fyne.PointEvent) {}

func (o *answerOption) MouseIn(_ *desktop.MouseEvent) {
	o.hovered = true
	o.Refresh()
}

func (o *answerOption) MouseMoved(_ *desktop.MouseEvent) {}

func (o *answerOption) MouseOut() {
	o.hovered = false
	o.Refresh()
}

func (o *answerOption) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Gray700)
	bg.CornerRadius = 8
	bg.StrokeWidth = 2

	shortcutBg := canvas.NewRectangle(theme.Gray900)
	shortcutBg.CornerRadius = 8
	shortcutBg.StrokeWidth = 2

	shortcutText := canvas.NewText(o.shortcut, theme.Yellow400)
	shortcutText.Alignment = fyne.TextAlignCenter
	shortcutText.TextSize = shortcutPillTextSize
	shortcutText.TextStyle = fyne.TextStyle{Bold: true}

	label := widget.NewLabel(o.text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Wrapping = fyne.TextWrapWord
	// The card has a fixed height; answers too long to wrap inside it
	// ellipsise instead of clipping mid-glyph.
	label.Truncation = fyne.TextTruncateEllipsis

	r := &answerOptionRenderer{
		option:       o,
		bg:           bg,
		shortcutBg:   shortcutBg,
		shortcutText: shortcutText,
		label:        label,
		objects:      []fyne.CanvasObject{bg, shortcutBg, shortcutText, label},
	}
	r.Refresh()
	return r
}

type answerOptionRenderer struct {
	option *answerOption

	bg           *canvas.Rectangle
	shortcutBg   *canvas.Rectangle
	shortcutText *canvas.Text
	label        *widget.Label

	objects []fyne.CanvasObject
}

func (r *answerOptionRenderer) Layout(size fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)

	pillSize := fyne.NewSize(84, 44)
	if r.option.shortcut == "" {
		// No keyboard shortcut: collapse the pill and keep the label
		// column aligned with the pilled rows above.
		pillSize = fyne.NewSize(84, 0)
	}
	pillPos := fyne.NewPos(20, (size.Height-pillSize.Height)/2)
	r.shortcutBg.Move(pillPos)
	r.shortcutBg.Resize(pillSize)

	shortcutSize := r.shortcutText.MinSize()
	r.shortcutText.Move(fyne.NewPos(
		pillPos.X+(pillSize.Width-shortcutSize.Width)/2,
		pillPos.Y+(pillSize.Height-shortcutSize.Height)/2,
	))
	r.shortcutText.Resize(shortcutSize)

	// The label column starts one pill-margin after the pill, so the
	// row reflows if the pill dimensions ever change.
	labelX := pillPos.X + pillSize.Width + 20
	labelWidth := max(size.Width-labelX-36, 80)
	labelHeight := r.label.MinSize().Height
	if labelHeight > size.Height-24 {
		labelHeight = size.Height - 24
	}
	r.label.Move(fyne.NewPos(labelX, (size.Height-labelHeight)/2))
	r.label.Resize(fyne.NewSize(labelWidth, labelHeight))
}

func (r *answerOptionRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 96)
}

func (r *answerOptionRenderer) Refresh() {
	r.bg.FillColor = theme.Gray700
	r.bg.StrokeColor = theme.Gray600

	if r.option.hovered {
		r.bg.FillColor = theme.Gray600
		r.bg.StrokeColor = theme.Gray400
	}

	if r.option.selected {
		r.bg.StrokeColor = theme.Yellow500
		r.shortcutBg.StrokeColor = theme.Gray400
	} else {
		r.shortcutBg.StrokeColor = theme.Gray600
	}
	r.shortcutText.Text = r.option.shortcut
	if r.option.shortcut == "" {
		r.shortcutBg.Hide()
		r.shortcutText.Hide()
	} else {
		r.shortcutBg.Show()
		r.shortcutText.Show()
	}
	r.label.SetText(r.option.text)

	canvas.Refresh(r.bg)
	canvas.Refresh(r.shortcutBg)
	canvas.Refresh(r.shortcutText)
	r.label.Refresh()
}

func (r *answerOptionRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *answerOptionRenderer) Destroy() {}

func newAnswerOption(shortcut, text string, selected bool, onTapped func()) *answerOption {
	o := &answerOption{
		shortcut: shortcut,
		text:     text,
		selected: selected,
		onTapped: onTapped,
	}

	o.ExtendBaseWidget(o)

	return o
}
