//go:build integration

package questions_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/project"
	questionstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/questions"
)

func TestStepValidationDataAddAndReset(t *testing.T) {
	// Given
	test.NewApp()

	// When
	step := questionstep.New()

	// Then
	require.Equal(t, "Define questions", step.Title())
	require.NotNil(t, step.Container())
	require.False(t, step.Validate())

	// When
	entries := entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("First?")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("Yes, No")

	// Then
	require.True(t, step.Validate())

	// When
	fynetest.TapButton(t, step.Container(), "Add Question")
	entries = entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("Second?")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("A, B, ")

	data := step.Data()

	// Then
	require.Len(t, data.Questions, 2)
	require.Equal(t, "First?", data.Questions[0].Text)
	require.Equal(t, []string{"A", "B"}, data.Questions[1].Answers)

	// When
	step.Reset()

	// Then
	data = step.Data()
	require.Len(t, data.Questions, 1)
	require.Empty(t, data.Questions[0].Text)
}

func TestStepValidateRejectsDuplicateAnswers(t *testing.T) {
	// Given a question whose answers repeat
	test.NewApp()
	step := questionstep.New()
	entries := entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("Sentiment?")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("Yes, Yes")

	// When / Then validation fails on the duplicate.
	require.False(t, step.Validate())

	// When the duplicate is fixed
	entries["e.g., Positive, Negative, Neutral"][0].SetText("Yes, No")

	// Then
	require.True(t, step.Validate())
}

func TestStepCollectsImageGridQuestion(t *testing.T) {
	// Given
	test.NewApp()
	step := questionstep.New()
	entries := entriesByPlaceholder(step.Container())

	// When
	fynetest.SelectOption(t, step.Container(), "Image annotation")
	entries = entriesByPlaceholder(step.Container())
	entries["e.g., Click every cell containing a car"][0].SetText("Click every cell with a car")

	// Then a grid question does not require answer choices.
	require.True(t, step.Validate())
	data := step.Data()
	require.Equal(t, []project.QuestionDef{{
		Kind:        project.QuestionKindImageGrid,
		Text:        "Click every cell with a car",
		GridRows:    project.DefaultGridRows,
		GridColumns: project.DefaultGridColumns,
	}}, data.Questions)
}

func TestStepHasInputTracksUserEntries(t *testing.T) {
	// Given
	test.NewApp()
	step := questionstep.New()
	entries := entriesByPlaceholder(step.Container())

	// Then a pristine step reports no input.
	require.False(t, step.HasInput())

	// When typing question text
	entries["e.g., What is the sentiment?"][0].SetText("Anything?")

	// Then
	require.True(t, step.HasInput())

	// When clearing it and typing answers instead
	entries["e.g., What is the sentiment?"][0].SetText("")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("Yes, No")

	// Then
	require.True(t, step.HasInput())

	// When clearing everything again
	entries["e.g., Positive, Negative, Neutral"][0].SetText("")

	// Then
	require.False(t, step.HasInput())

	// When adding a second question row
	fynetest.TapButton(t, step.Container(), "Add Question")

	// Then the extra row alone counts as input.
	require.True(t, step.HasInput())
}

func TestStepNavigatesBetweenQuestions(t *testing.T) {
	// Given two valid questions where the second is currently visible
	test.NewApp()
	step := questionstep.New()
	entries := entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("First?")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("Yes, No")
	fynetest.TapButton(t, step.Container(), "Add Question")
	entries = entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("Second?")
	entries["e.g., Positive, Negative, Neutral"][0].SetText("A, B")

	// When moving back to the previous question
	require.True(t, step.Back())

	// Then
	fynetest.RequireLabel(t, step.Container(), "Question 1 of 2")
	require.True(t, step.HasNext())

	// When advancing again
	handled, valid := step.Advance()

	// Then
	require.True(t, handled)
	require.True(t, valid)
	fynetest.RequireLabel(t, step.Container(), "Question 2 of 2")
	require.False(t, step.HasNext())
}

func TestStepRemoveQuestionRenumbersAndGuardsLastRow(t *testing.T) {
	// Given three filled question rows
	test.NewApp()
	step := questionstep.New()
	entries := entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("First?")
	fynetest.TapButton(t, step.Container(), "Add Question")
	entries = entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("Second?")
	fynetest.TapButton(t, step.Container(), "Add Question")
	entries = entriesByPlaceholder(step.Container())
	entries["e.g., What is the sentiment?"][0].SetText("Third?")
	fynetest.RequireLabel(t, step.Container(), "Question 3 of 3")

	require.True(t, step.Back())

	// When removing the middle question via its icon-only remove button
	removeButtons := iconOnlyButtons(step.Container())
	require.Len(t, removeButtons, 1)
	test.Tap(removeButtons[0])

	// Then the remaining questions close ranks and renumber.
	data := step.Data()
	require.Len(t, data.Questions, 2)
	require.Equal(t, "First?", data.Questions[0].Text)
	require.Equal(t, "Third?", data.Questions[1].Text)
	fynetest.RequireLabel(t, step.Container(), "Question 2 of 2")
	fynetest.RequireNoLabel(t, step.Container(), "Question 3 of 3")

	// When removing down to a single question
	removeButtons = iconOnlyButtons(step.Container())
	require.Len(t, removeButtons, 1)
	test.Tap(removeButtons[0])

	// Then the last row's remove button is disabled so one question remains.
	removeButtons = iconOnlyButtons(step.Container())
	require.Len(t, removeButtons, 1)
	require.True(t, removeButtons[0].Disabled())
	data = step.Data()
	require.Len(t, data.Questions, 1)
	require.Equal(t, "First?", data.Questions[0].Text)
}

func iconOnlyButtons(root fyne.CanvasObject) []*widget.Button {
	var buttons []*widget.Button
	fynetest.Walk(root, func(obj fyne.CanvasObject) {
		button, ok := obj.(*widget.Button)
		if ok && button.Text == "" {
			buttons = append(buttons, button)
		}
	})

	return buttons
}

func entriesByPlaceholder(root fyne.CanvasObject) map[string][]*widget.Entry {
	entries := make(map[string][]*widget.Entry)
	fynetest.Walk(root, func(obj fyne.CanvasObject) {
		entry, ok := obj.(*widget.Entry)
		if ok {
			entries[entry.PlaceHolder] = append(entries[entry.PlaceHolder], entry)
		}
	})

	return entries
}
