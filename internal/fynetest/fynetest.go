package fynetest

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
)

// Walk visits root and every descendant, descending through both plain
// containers and widget renderers so content is reachable no matter
// how it is composed.
func Walk(root fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if root == nil {
		return
	}

	visit(root)

	switch typed := root.(type) {
	case *fyne.Container:
		for _, child := range typed.Objects {
			Walk(child, visit)
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(typed).Objects() {
			Walk(child, visit)
		}
	}
}

// FirstTappable returns the first tappable object under root in
// traversal order.
func FirstTappable(t *testing.T, root fyne.CanvasObject) fyne.Tappable {
	t.Helper()

	var found fyne.Tappable
	Walk(root, func(obj fyne.CanvasObject) {
		if tappable, ok := obj.(fyne.Tappable); ok && found == nil {
			found = tappable
		}
	})
	require.NotNil(t, found, "tappable object not found")

	return found
}

// ButtonWithText returns the first button under root with the given label.
func ButtonWithText(t *testing.T, root fyne.CanvasObject, text string) *widget.Button {
	t.Helper()

	var found *widget.Button
	Walk(root, func(obj fyne.CanvasObject) {
		if button, ok := obj.(*widget.Button); ok && button.Text == text && found == nil {
			found = button
		}
	})
	require.NotNil(t, found, "button %q not found", text)

	return found
}

// TapButton taps the first button under root with the given label.
func TapButton(t *testing.T, root fyne.CanvasObject, text string) {
	t.Helper()

	test.Tap(ButtonWithText(t, root, text))
}

// CheckWithText returns the first checkbox under root with the given label.
func CheckWithText(t *testing.T, root fyne.CanvasObject, text string) *widget.Check {
	t.Helper()

	var found *widget.Check
	Walk(root, func(obj fyne.CanvasObject) {
		if check, ok := obj.(*widget.Check); ok && check.Text == text && found == nil {
			found = check
		}
	})
	require.NotNil(t, found, "checkbox %q not found", text)

	return found
}

// EntryWithPlaceholder returns the first entry under root with the
// given placeholder text.
func EntryWithPlaceholder(t *testing.T, root fyne.CanvasObject, placeholder string) *widget.Entry {
	t.Helper()

	var found *widget.Entry
	Walk(root, func(obj fyne.CanvasObject) {
		if entry, ok := obj.(*widget.Entry); ok && entry.PlaceHolder == placeholder && found == nil {
			found = entry
		}
	})
	require.NotNil(t, found, "entry with placeholder %q not found", placeholder)

	return found
}

// TypeEntry clears the entry identified by placeholder and types text
// into it like a user would.
func TypeEntry(t *testing.T, root fyne.CanvasObject, placeholder, text string) {
	t.Helper()

	entry := EntryWithPlaceholder(t, root, placeholder)
	entry.SetText("")
	test.Type(entry, text)
}

// RequireEntryValue asserts the entry identified by placeholder holds
// exactly want.
func RequireEntryValue(t *testing.T, root fyne.CanvasObject, placeholder, want string) {
	t.Helper()

	require.Equal(t, want, EntryWithPlaceholder(t, root, placeholder).Text)
}

// SelectRadio selects option on the first radio group under root.
func SelectRadio(t *testing.T, root fyne.CanvasObject, option string) {
	t.Helper()

	var found *widget.RadioGroup
	Walk(root, func(obj fyne.CanvasObject) {
		if group, ok := obj.(*widget.RadioGroup); ok && found == nil {
			found = group
		}
	})
	require.NotNil(t, found, "radio group not found")

	found.SetSelected(option)
}

// SelectOption selects option on the first dropdown under root.
func SelectOption(t *testing.T, root fyne.CanvasObject, option string) {
	t.Helper()

	var found *widget.Select
	Walk(root, func(obj fyne.CanvasObject) {
		if selectWidget, ok := obj.(*widget.Select); ok && found == nil {
			found = selectWidget
		}
	})
	require.NotNil(t, found, "select widget not found")

	found.SetSelected(option)
}

// RequireLabel asserts a label with exactly text exists under root.
func RequireLabel(t *testing.T, root fyne.CanvasObject, text string) {
	t.Helper()

	require.True(t, hasLabel(root, text), "label %q not found", text)
}

// RequireNoLabel asserts no label with exactly text exists under root.
func RequireNoLabel(t *testing.T, root fyne.CanvasObject, text string) {
	t.Helper()

	require.False(t, hasLabel(root, text), "label %q unexpectedly found", text)
}

func hasLabel(root fyne.CanvasObject, text string) bool {
	found := false
	Walk(root, func(obj fyne.CanvasObject) {
		if label, ok := obj.(*widget.Label); ok && label.Text == text {
			found = true
		}
	})

	return found
}

// RequireText asserts that visible text equal to text is rendered
// somewhere under root, whether as a widget.Label or a raw canvas.Text
// (the form package renders error copy as canvas.Text).
func RequireText(t *testing.T, root fyne.CanvasObject, text string) {
	t.Helper()

	found := false
	Walk(root, func(obj fyne.CanvasObject) {
		switch typed := obj.(type) {
		case *widget.Label:
			if typed.Text == text && typed.Visible() {
				found = true
			}
		case *canvas.Text:
			if typed.Text == text && typed.Visible() {
				found = true
			}
		}
	})
	require.True(t, found, "text %q not found", text)
}

// RequireTextContains asserts that text appears as a substring of any
// text-bearing widget under root: labels, buttons, entries (including
// placeholders), radio groups, and dropdowns.
func RequireTextContains(t *testing.T, root fyne.CanvasObject, text string) {
	t.Helper()

	require.True(t, HasTextContaining(root, text), "text %q not found", text)
}

// HasTextContaining reports whether text appears as a substring of any
// text-bearing widget under root.
func HasTextContaining(root fyne.CanvasObject, text string) bool {
	found := false
	Walk(root, func(obj fyne.CanvasObject) {
		if strings.Contains(textForObject(obj), text) {
			found = true
		}
	})

	return found
}

func textForObject(obj fyne.CanvasObject) string {
	switch typed := obj.(type) {
	case *canvas.Text:
		return typed.Text
	case *widget.Button:
		return typed.Text
	case *widget.Label:
		return typed.Text
	case *widget.Entry:
		return typed.Text + "\n" + typed.PlaceHolder
	case *widget.RadioGroup:
		return strings.Join(append([]string{typed.Selected}, typed.Options...), "\n")
	case *widget.Select:
		return strings.Join(append([]string{typed.Selected, typed.PlaceHolder}, typed.Options...), "\n")
	default:
		return ""
	}
}
