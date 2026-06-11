//go:build integration

package project_test

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/fynetest"
	projectstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/project"
)

func TestStepValidationDataAndReset(t *testing.T) {
	// Given
	test.NewApp()

	// When
	step := projectstep.New(projectstep.Handlers{})

	// Then
	require.Equal(t, "Create new project", step.Title())
	require.NotNil(t, step.Container())
	require.False(t, step.Validate())

	// When
	step.SetName("Review/One")
	step.SetDBPath("/tmp/review.deckcheck")

	// Then
	require.True(t, step.Validate())
	require.Equal(t, "Review-One.deckcheck", step.SuggestedFilename())

	data := step.Data()
	require.Equal(t, "Review/One", data.Name)
	require.Equal(t, "/tmp/review.deckcheck", data.DBPath)

	// When
	step.Reset()

	// Then
	data = step.Data()
	require.Empty(t, data.Name)
	require.Empty(t, data.DBPath)
}

func TestStepBrowseButtonReportsToOwner(t *testing.T) {
	// Given
	test.NewApp()
	browsed := false
	step := projectstep.New(projectstep.Handlers{Browse: func() { browsed = true }})

	// When
	fynetest.TapButton(t, step.Container(), "Choose file…")

	// Then
	require.True(t, browsed)
}
