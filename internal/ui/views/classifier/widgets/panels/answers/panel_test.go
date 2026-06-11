//go:build integration

package answers_test

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/panels/answers"
)

func TestPanelSelectAnswerByIndex(t *testing.T) {
	// Given
	test.NewApp()

	var changed []int
	panel := answers.New([]project.Question{
		{ID: 1, Text: "First?", Answers: []project.Answer{{ID: 10, Text: "Yes"}, {ID: 11, Text: "No"}}},
		{ID: 2, Text: "Second?", Answers: []project.Answer{{ID: 20, Text: "A"}, {ID: 21, Text: "B"}}},
	}, answers.Handlers{
		Changed: func(questionID, answerID int, selected bool) {
			if selected {
				changed = append(changed, questionID, answerID)
			}
		},
	})

	// When keying answers: No for Q1 (display advances to Q2), A for
	// Q2, then B for Q2; keys always target the question on screen,
	// which stays the last one once everything is answered. An
	// out-of-range index is ignored.
	panel.SelectAnswerByIndex(1)
	panel.SelectAnswerByIndex(0)
	panel.SelectAnswerByIndex(1)
	panel.SelectAnswerByIndex(20)

	// Then every selection reached the Changed handler in order.
	require.Equal(t, []int{1, 11, 2, 20, 2, 21}, changed)
	require.True(t, panel.AllAnswered())

	// When the owner resets the selections (e.g. a new record loads)
	panel.SetSelections(nil)

	// Then
	require.False(t, panel.AllAnswered())
}

func TestPanelSetSelectionsCopiesInput(t *testing.T) {
	// Given
	test.NewApp()

	panel := answers.New([]project.Question{
		{ID: 1, Text: "Q1", Answers: []project.Answer{{ID: 1, Text: "A1"}, {ID: 2, Text: "A2"}}},
	}, answers.Handlers{})
	selections := map[int]int{1: 2}

	// When the caller mutates its map after handing it over
	panel.SetSelections(selections)
	delete(selections, 1)

	// Then the panel's own state is unaffected.
	require.True(t, panel.AllAnswered())
}

func TestPanelHandlesNoQuestions(t *testing.T) {
	// Given
	test.NewApp()

	// When
	panel := answers.New(nil, answers.Handlers{})

	// Then
	require.NotNil(t, panel.Container())
	require.True(t, panel.AllAnswered())
}

func TestPanelTapTogglesAnswerSelection(t *testing.T) {
	// Given a panel with a single two-answer question
	test.NewApp()

	type change struct {
		questionID, answerID int
		selected             bool
	}
	var changes []change
	panel := answers.New([]project.Question{
		{ID: 1, Text: "Useful?", Answers: []project.Answer{{ID: 10, Text: "Yes"}, {ID: 11, Text: "No"}}},
	}, answers.Handlers{
		Changed: func(questionID, answerID int, selected bool) {
			changes = append(changes, change{questionID, answerID, selected})
		},
	})

	// When tapping the first answer card with the mouse
	test.Tap(answerOptionAt(t, panel.Container(), 0))

	// Then the selection reaches the Changed handler and the rebuilt
	// card shows the selected stroke.
	require.Equal(t, []change{{1, 10, true}}, changes)
	require.True(t, panel.AllAnswered())
	require.Equal(t, theme.Yellow500, optionBackground(t, panel.Container(), 0).StrokeColor)

	// When tapping the same answer again
	test.Tap(answerOptionAt(t, panel.Container(), 0))

	// Then the selection toggles off.
	require.Equal(t, []change{{1, 10, true}, {1, 10, false}}, changes)
	require.False(t, panel.AllAnswered())
	require.NotEqual(t, theme.Yellow500, optionBackground(t, panel.Container(), 0).StrokeColor)
}

func TestPanelHoverHighlightsOption(t *testing.T) {
	// Given a panel with one question on screen
	test.NewApp()

	panel := answers.New([]project.Question{
		{ID: 1, Text: "Useful?", Answers: []project.Answer{{ID: 10, Text: "Yes"}, {ID: 11, Text: "No"}}},
	}, answers.Handlers{})

	option := answerOptionAt(t, panel.Container(), 0)
	hoverable, ok := option.(desktop.Hoverable)
	require.True(t, ok, "answer option must be hoverable")

	// When the pointer enters the card
	hoverable.MouseIn(&desktop.MouseEvent{})

	// Then the background lifts to the hover fill...
	require.Equal(t, theme.Gray600, optionBackground(t, panel.Container(), 0).FillColor)

	// ...and when the pointer leaves it drops back to the resting fill.
	hoverable.MouseOut()
	require.Equal(t, theme.Gray700, optionBackground(t, panel.Container(), 0).FillColor)
}

func TestPanelManyAnswersScrollAndCapShortcutPills(t *testing.T) {
	// Given a question with more answers than there are number keys
	test.NewApp()

	answersList := make([]project.Answer, 12)
	for i := range answersList {
		answersList[i] = project.Answer{ID: 100 + i, Text: fmt.Sprintf("Answer %d", i+1)}
	}
	panel := answers.New([]project.Question{
		{ID: 1, Text: "Which?", Answers: answersList},
	}, answers.Handlers{})

	// Then every answer is present and tappable (the scroll bars are
	// tappable too, hence at-least)...
	var tappables int
	fynetest.Walk(panel.Container(), func(obj fyne.CanvasObject) {
		if _, ok := obj.(fyne.Tappable); ok {
			tappables++
		}
	})
	require.GreaterOrEqual(t, tappables, 12)
	fynetest.RequireLabel(t, panel.Container(), "Answer 12")

	// ...the options live in a scroll container so the window's
	// minimum height stays bounded...
	var scrolls int
	fynetest.Walk(panel.Container(), func(obj fyne.CanvasObject) {
		if _, ok := obj.(*container.Scroll); ok {
			scrolls++
		}
	})
	require.NotZero(t, scrolls, "options must be wrapped in a scroll container")

	// ...and no pill advertises a number key that does not exist.
	// Pills are the only canvas.Text rendered at the shortcut size,
	// which distinguishes them from label fragments.
	var pills []string
	fynetest.Walk(panel.Container(), func(obj fyne.CanvasObject) {
		if text, ok := obj.(*canvas.Text); ok && text.Visible() && text.TextSize == answers.ShortcutPillTextSize {
			pills = append(pills, text.Text)
		}
	})
	require.Equal(t, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, pills)
}

// answerOptionAt returns the index-th tappable answer card under root.
func answerOptionAt(t *testing.T, root fyne.CanvasObject, index int) fyne.Tappable {
	t.Helper()

	var found []fyne.Tappable
	fynetest.Walk(root, func(obj fyne.CanvasObject) {
		if tappable, ok := obj.(fyne.Tappable); ok {
			found = append(found, tappable)
		}
	})
	require.Greater(t, len(found), index, "tappable answer card %d not found", index)

	return found[index]
}

// optionBackground returns the background rectangle of the index-th
// answer card, exposing the fill/stroke the renderer paints for the
// hover and selected states.
func optionBackground(t *testing.T, root fyne.CanvasObject, index int) *canvas.Rectangle {
	t.Helper()

	option, ok := answerOptionAt(t, root, index).(fyne.Widget)
	require.True(t, ok, "answer card must be a widget")

	for _, obj := range test.WidgetRenderer(option).Objects() {
		if rect, ok := obj.(*canvas.Rectangle); ok {
			return rect
		}
	}

	t.Fatal("answer card renderer has no background rectangle")
	return nil
}
