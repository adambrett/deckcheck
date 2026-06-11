//go:build integration

package errordialog_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/fyneui/errordialog"
	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestShowRendersMessageSectionsInOverlay(t *testing.T) {
	// Given
	t.Setenv("DECKCHECK_DEBUG", "")
	fyneApp := test.NewApp()
	window := fyneApp.NewWindow("test")
	window.Resize(fyne.NewSize(800, 600))
	message := usererror.Message{
		Severity:    usererror.SeverityError,
		Code:        "DC07",
		Summary:     "Could not open project",
		Description: "The file is not a DeckCheck project.",
		Impact:      "The file was not changed.",
		Resolution:  "Pick a different file and try again.",
	}

	// When
	errordialog.Show(window, fyneApp.Clipboard(), message)

	// Then
	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)

	texts := collectTexts(overlay)
	require.Contains(t, texts, string(usererror.SeverityError))
	require.Contains(t, texts, "DC07")
	require.Contains(t, texts, "Could not open project")
	require.Contains(t, texts, "The file is not a DeckCheck project.")
	require.Contains(t, texts, "The file was not changed.")
	require.Contains(t, texts, "Pick a different file and try again.")
}

func TestShowCopyButtonCopiesCodeToClipboard(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	window := fyneApp.NewWindow("test")
	window.Resize(fyne.NewSize(800, 600))
	message := usererror.Message{
		Severity: usererror.SeverityError,
		Code:     "DC11",
		Summary:  "Could not export csv",
	}
	errordialog.Show(window, fyneApp.Clipboard(), message)

	copyButton := findCopyButton(window.Canvas().Overlays().Top())
	require.NotNil(t, copyButton)

	// When
	test.Tap(copyButton)

	// Then
	require.Equal(t, "DC11", fyneApp.Clipboard().Content())
}

func TestShowOmitsCopyButtonWithoutCode(t *testing.T) {
	// Given
	fyneApp := test.NewApp()
	window := fyneApp.NewWindow("test")
	window.Resize(fyne.NewSize(800, 600))
	message := usererror.Message{
		Severity:    usererror.SeverityError,
		Summary:     "Could not open project",
		Description: "The file is not a DeckCheck project.",
	}

	// When
	errordialog.Show(window, fyneApp.Clipboard(), message)

	// Then
	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	require.Nil(t, findCopyButton(overlay))
}

func TestShowShowsTechnicalDetailsAccordionWithDebugEnv(t *testing.T) {
	// Given
	t.Setenv("DECKCHECK_DEBUG", "1")
	fyneApp := test.NewApp()
	window := fyneApp.NewWindow("test")
	window.Resize(fyne.NewSize(800, 600))
	message := usererror.Message{
		Severity:  usererror.SeverityError,
		Summary:   "Could not open project",
		Technical: "open project: file missing",
	}

	// When
	errordialog.Show(window, fyneApp.Clipboard(), message)

	// Then
	accordion := findAccordion(window.Canvas().Overlays().Top())
	require.NotNil(t, accordion)
	require.Len(t, accordion.Items, 1)
	require.Equal(t, "Show technical details", accordion.Items[0].Title)
}

func TestShowHidesTechnicalDetailsWithoutDebugEnv(t *testing.T) {
	// Given
	t.Setenv("DECKCHECK_DEBUG", "")
	fyneApp := test.NewApp()
	window := fyneApp.NewWindow("test")
	window.Resize(fyne.NewSize(800, 600))
	message := usererror.Message{
		Severity:  usererror.SeverityError,
		Summary:   "Could not open project",
		Technical: "open project: file missing",
	}

	// When
	errordialog.Show(window, fyneApp.Clipboard(), message)

	// Then
	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	require.Nil(t, findAccordion(overlay))
}

// collectTexts gathers the visible text of every label and canvas text
// node under obj.
func collectTexts(obj fyne.CanvasObject) []string {
	var texts []string
	fynetest.Walk(obj, func(node fyne.CanvasObject) {
		switch typed := node.(type) {
		case *widget.Label:
			texts = append(texts, typed.Text)
		case *canvas.Text:
			texts = append(texts, typed.Text)
		}
	})

	return texts
}

// findCopyButton returns the icon-only copy button under obj, or nil
// when the dialog renders without a copy affordance.
func findCopyButton(obj fyne.CanvasObject) *widget.Button {
	var found *widget.Button
	fynetest.Walk(obj, func(node fyne.CanvasObject) {
		button, ok := node.(*widget.Button)
		if !ok || button.Text != "" || button.Icon == nil {
			return
		}
		if button.Icon.Name() == theme.ContentCopyIcon().Name() {
			found = button
		}
	})

	return found
}

// findAccordion returns the first accordion under obj, or nil when the
// technical-details section is absent.
func findAccordion(obj fyne.CanvasObject) *widget.Accordion {
	var found *widget.Accordion
	fynetest.Walk(obj, func(node fyne.CanvasObject) {
		if accordion, ok := node.(*widget.Accordion); ok && found == nil {
			found = accordion
		}
	})

	return found
}
