//go:build integration

package questions_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
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
	entries["e.g., What is the sentiment?"][1].SetText("Second?")
	entries["e.g., Positive, Negative, Neutral"][1].SetText("A, B, ")

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

func TestStepRemoveQuestionRenumbersAndGuardsLastRow(t *testing.T) {
	// Given three filled question rows
	test.NewApp()
	step := questionstep.New()
	fynetest.TapButton(t, step.Container(), "Add Question")
	fynetest.TapButton(t, step.Container(), "Add Question")

	entries := entriesByPlaceholder(step.Container())
	texts := entries["e.g., What is the sentiment?"]
	require.Len(t, texts, 3)
	texts[0].SetText("First?")
	texts[1].SetText("Second?")
	texts[2].SetText("Third?")
	fynetest.RequireLabel(t, step.Container(), "Question 3")

	// When removing the middle question via its icon-only remove button
	removeButtons := iconOnlyButtons(step.Container())
	require.Len(t, removeButtons, 3)
	test.Tap(removeButtons[1])

	// Then the remaining questions close ranks and renumber.
	data := step.Data()
	require.Len(t, data.Questions, 2)
	require.Equal(t, "First?", data.Questions[0].Text)
	require.Equal(t, "Third?", data.Questions[1].Text)
	fynetest.RequireLabel(t, step.Container(), "Question 1")
	fynetest.RequireLabel(t, step.Container(), "Question 2")
	fynetest.RequireNoLabel(t, step.Container(), "Question 3")

	// When removing down to a single question
	removeButtons = iconOnlyButtons(step.Container())
	require.Len(t, removeButtons, 2)
	test.Tap(removeButtons[1])

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
