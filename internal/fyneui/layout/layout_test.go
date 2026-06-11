//go:build integration

package layout_test

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fyneui/layout"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

func TestNewStacksToolbarAboveContent(t *testing.T) {
	// Given
	test.NewApp()
	toolbar := canvas.NewRectangle(theme.Gray700)
	toolbar.SetMinSize(fyne.NewSize(100, 50))

	content := canvas.NewRectangle(theme.Gray800)
	content.SetMinSize(fyne.NewSize(200, 200))

	// When
	l := layout.New(toolbar, content)

	// Then
	require.NotNil(t, l)
}

func TestFixedHeightLayout(t *testing.T) {
	// Given
	test.NewApp()
	obj := canvas.NewRectangle(theme.Gray700)
	obj.SetMinSize(fyne.NewSize(120, 40))

	containerSize := fyne.NewSize(300, 90)
	l := layout.NewFixedHeight(64, obj)

	// When
	minSize := l.MinSize()
	l.Resize(containerSize)

	// Then
	require.Equal(t, float32(120), minSize.Width)
	require.Equal(t, float32(64), minSize.Height)
	require.Equal(t, containerSize, obj.Size())
	require.Equal(t, fyne.NewPos(0, 0), obj.Position())
}

func TestFixedHeightLayoutSkipsHiddenChildren(t *testing.T) {
	// Given
	test.NewApp()
	obj := canvas.NewRectangle(theme.Gray700)
	obj.SetMinSize(fyne.NewSize(120, 40))
	obj.Hide()

	l := layout.NewFixedHeight(64, obj)

	// When
	minSize := l.MinSize()
	l.Resize(fyne.NewSize(300, 90))

	// Then
	require.Equal(t, float32(0), minSize.Width)
	require.Equal(t, float32(64), minSize.Height)
	require.NotEqual(t, fyne.NewSize(300, 90), obj.Size())
}

func TestFixedWidthLayout(t *testing.T) {
	// Given
	test.NewApp()
	obj := canvas.NewRectangle(theme.Gray700)
	obj.SetMinSize(fyne.NewSize(40, 30))

	containerSize := fyne.NewSize(400, 120)
	l := layout.NewFixedWidth(200, obj)

	// When
	minSize := l.MinSize()
	l.Resize(containerSize)

	// Then
	require.Equal(t, float32(200), minSize.Width)
	require.Equal(t, float32(200), obj.Size().Width)
	require.Equal(t, containerSize.Height, obj.Size().Height)
	require.Equal(t, fyne.NewPos(0, 0), obj.Position())
}

func TestFixedWidthLayoutProbesWrappedLabelHeight(t *testing.T) {
	// Given
	test.NewApp()
	singleLineHeight := widget.NewLabel("word").MinSize().Height

	label := widget.NewLabel(strings.TrimSpace(strings.Repeat("word ", 60)))
	label.Wrapping = fyne.TextWrapWord

	l := layout.NewFixedWidth(200, label)

	// When
	minSize := l.MinSize()

	// Then
	require.Equal(t, float32(200), minSize.Width)
	require.Greater(t, minSize.Height, singleLineHeight)
}

func TestFixedWidthLayoutSkipsHiddenChildren(t *testing.T) {
	// Given
	test.NewApp()
	obj := canvas.NewRectangle(theme.Gray700)
	obj.SetMinSize(fyne.NewSize(40, 30))
	obj.Hide()

	l := layout.NewFixedWidth(200, obj)

	// When
	minSize := l.MinSize()
	l.Resize(fyne.NewSize(400, 120))

	// Then
	require.Equal(t, float32(200), minSize.Width)
	require.Equal(t, float32(0), minSize.Height)
	require.NotEqual(t, fyne.NewSize(200, 120), obj.Size())
}
