package form

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

// Group stacks a label tightly above its input, the unit every form
// row is built from.
func Group(label, field fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedVBoxLayout(2), label, field)
}

// Groups stacks form rows with consistent vertical breathing room.
func Groups(groups ...fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedVBoxLayout(14), groups...)
}

// Field is a labelled input with an error border, error copy, and
// optional persistent help text.
type Field struct {
	container *fyne.Container
	border    *canvas.Rectangle
	errorText *canvas.Text
	helpText  *canvas.Text
}

// Option configures optional fields on a Field.
type Option func(*options)

type options struct {
	help string
}

func defaultOptions() options {
	return options{}
}

// WithHelpText supplies persistent caption copy shown beneath the
// input. Unlike an Entry placeholder it does not disappear when the
// user types or when a callback (e.g. Browse) fills the field, so it
// is the right home for guidance the user may need to re-read.
func WithHelpText(text string) Option {
	return func(o *options) {
		o.help = text
	}
}

// NewField wraps an input in the form's framed row: label above,
// error border around, and help/error copy below.
func NewField(label, field fyne.CanvasObject, opts ...Option) *Field {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	border := canvas.NewRectangle(nil)
	border.CornerRadius = 8
	border.StrokeColor = theme.ErrorRed
	border.StrokeWidth = 2
	border.Hide()

	errorText := canvas.NewText("", theme.ErrorRed)
	errorText.TextSize = 12
	errorText.Hide()

	fieldFrame := container.NewStack(border, container.NewPadded(field))

	f := &Field{
		border:    border,
		errorText: errorText,
	}

	column := []fyne.CanvasObject{fieldFrame, errorText}
	if options.help != "" {
		help := canvas.NewText(options.help, theme.Gray400)
		help.TextSize = 12
		f.helpText = help
		// Help sits between the input and the (initially hidden) error
		// row so the error can replace the visual weight of the help
		// without re-flowing the layout.
		column = []fyne.CanvasObject{fieldFrame, help, errorText}
	}

	f.container = Group(label, container.New(layout.NewCustomPaddedVBoxLayout(2), column...))
	return f
}

// Container returns the field's renderable row.
func (f *Field) Container() fyne.CanvasObject {
	return f.container
}

// SetError shows message (and the red frame) beneath the input, or
// clears both when message is empty.
func (f *Field) SetError(message string) {
	if message == "" {
		f.border.Hide()
		f.errorText.Hide()
		f.container.Refresh()
		return
	}

	f.errorText.Text = message
	f.border.Show()
	f.errorText.Show()
	f.container.Refresh()
}

// ErrorVisible reports whether the error row is currently showing.
func (f *Field) ErrorVisible() bool {
	return f.errorText.Visible()
}

// ErrorText returns the currently displayed error copy.
func (f *Field) ErrorText() string {
	return f.errorText.Text
}
