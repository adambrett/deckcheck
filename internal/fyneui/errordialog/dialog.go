package errordialog

import (
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/layout"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// dialogWidth pins the dialog body wrap width so multi-line resolution
// copy reads as paragraphs rather than narrow columns.
const dialogWidth = float32(520)

// Show renders message over window: severity pill and copyable code
// at the top, then description, impact, and resolution as readable
// paragraphs. Body text stays as labels so it reads naturally; only
// the support code is wired for one-click copy.
func Show(window fyne.Window, clipboard fyne.Clipboard, message usererror.Message) {
	body := container.NewVBox(buildHeader(clipboard, message))
	if message.Description != "" {
		body.Add(wrappedLabel(lang.L(message.Description)))
	}
	if message.Impact != "" {
		body.Add(wrappedLabel(lang.L(message.Impact)))
	}
	if message.Resolution != "" {
		body.Add(wrappedLabel(lang.L(message.Resolution)))
	}
	if debugDiagnosticsEnabled() && message.Technical != "" {
		body.Add(buildTechnicalDetails(message.Technical))
	}

	sized := layout.NewFixedWidth(dialogWidth, body)
	dialog.NewCustom(lang.L(message.Summary), lang.L("Close"), container.NewPadded(sized), window).Show()
}

// buildHeader returns the severity pill, code, and copy-icon row
// that anchors the dialog. The code renders monospace so it reads as
// a stable identifier.
func buildHeader(clipboard fyne.Clipboard, message usererror.Message) fyne.CanvasObject {
	pill := severityPill(message.Severity)
	if message.Code == "" {
		return pill
	}

	codeLabel := widget.NewLabelWithStyle(message.Code, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

	copyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		clipboard.SetContent(message.Code)
	})
	copyButton.Importance = widget.LowImportance

	return container.NewHBox(pill, codeLabel, copyButton)
}

// severityPill renders the severity badge as a small filled rounded
// rectangle with bold white text. Every severity renders with error
// emphasis today; a future warning tier picks its color here.
func severityPill(severity usererror.Severity) fyne.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameError))
	bg.CornerRadius = 4

	label := canvas.NewText(severityLabel(severity), color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.TextSize = theme.CaptionTextSize()
	label.Alignment = fyne.TextAlignCenter

	return container.NewStack(bg, container.NewPadded(label))
}

// severityLabel localises the severity badge text. Each case uses a
// literal lang.L key so the translations catalog gate can see it; a
// dynamic lang.L(string(severity)) would be invisible to the gate.
func severityLabel(severity usererror.Severity) string {
	switch severity {
	case usererror.SeverityError:
		return lang.L("Error")
	default:
		return string(severity)
	}
}

// wrappedLabel returns a word-wrapped Label suitable for a paragraph
// of body copy.
func wrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignLeading
	label.Wrapping = fyne.TextWrapWord

	return label
}

// buildTechnicalDetails wraps the raw error chain in an Accordion so
// it stays out of the way by default. The technical block lives in a
// disabled MultiLineEntry because it is the one section that must be
// fully copyable (developer use only, gated behind DECKCHECK_DEBUG).
func buildTechnicalDetails(technical string) fyne.CanvasObject {
	entry := widget.NewMultiLineEntry()
	entry.SetText(technical)
	entry.Wrapping = fyne.TextWrapWord
	entry.Disable()

	return widget.NewAccordion(
		widget.NewAccordionItem(lang.L("Show technical details"), entry),
	)
}

// debugDiagnosticsEnabled gates the raw error chain shown in the
// dialog. Off for end users; toggle by running with DECKCHECK_DEBUG=1
// to surface the underlying error verbatim.
func debugDiagnosticsEnabled() bool {
	switch os.Getenv("DECKCHECK_DEBUG") {
	case "1", "true", "TRUE", "yes":
		return true
	}

	return false
}
