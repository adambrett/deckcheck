//go:build integration

package record

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
)

func TestImageGridOverlayMapsTapsToContainedImageCells(t *testing.T) {
	// Given a wide image contained in a square widget, leaving top and
	// bottom letterboxing that should not receive grid clicks.
	test.NewApp()
	overlay := newImageGridOverlay()
	overlay.Resize(fyne.NewSize(200, 200))
	overlay.SetImageDimensions(100, 50)

	var selections []string
	overlay.SetConfig(GridConfig{
		Rows:    2,
		Columns: 2,
		Changed: func(value string) {
			selections = append(selections, value)
		},
	})

	// When tapping outside the contained image
	overlay.Tapped(&fyne.PointEvent{Position: fyne.NewPos(20, 20)})

	// Then it is ignored.
	require.Empty(t, selections)

	// When tapping the top-right image cell
	overlay.Tapped(&fyne.PointEvent{Position: fyne.NewPos(150, 75)})

	// Then
	require.Equal(t, []string{"B1"}, selections)
}

func TestImageGridOverlayDragsRectangularSelection(t *testing.T) {
	// Given
	test.NewApp()
	overlay := newImageGridOverlay()
	overlay.Resize(fyne.NewSize(200, 200))
	overlay.SetImageDimensions(200, 200)

	var selections []string
	overlay.SetConfig(GridConfig{
		Rows:    2,
		Columns: 2,
		Changed: func(value string) {
			selections = append(selections, value)
		},
	})

	// When dragging from A1 across to B1, then down to B2.
	overlay.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(150, 50)},
		Dragged:    fyne.Delta{DX: 100, DY: 0},
	})
	overlay.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(150, 150)},
		Dragged:    fyne.Delta{DX: 0, DY: 100},
	})
	overlay.DragEnd()

	// Then the drag fills the full rectangle from the anchor to the
	// current cell, not just the direct movement path.
	require.Equal(t, []string{"A1 B1", "A1 B1 A2 B2"}, selections)
}
