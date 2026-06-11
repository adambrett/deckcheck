package classifier

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/adambrett/deckcheck/internal/fyneui/layout"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
)

// recordPaneSplitOffset balances the record display against the
// answer column: slightly past half so wide records stay readable
// without starving the answer cards.
const recordPaneSplitOffset = 0.53

func (v *View) buildLayout() *fyne.Container {
	leftPane := container.NewStack(
		canvas.NewRectangle(theme.Gray950),
		container.NewPadded(v.recordDisplay.Container()),
	)
	rightPane := container.NewStack(
		canvas.NewRectangle(theme.Gray800),
		container.NewPadded(container.NewBorder(nil, v.statusLabel, nil, nil, v.answerPanel.Container())),
	)

	split := container.NewHSplit(leftPane, rightPane)
	split.Offset = recordPaneSplitOffset

	content := container.NewStack(canvas.NewRectangle(theme.Gray950), split)

	return layout.New(v.toolbar.Container(), content)
}
