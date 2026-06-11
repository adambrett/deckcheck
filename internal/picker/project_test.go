package picker_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/picker"
	"github.com/adambrett/deckcheck/internal/project"
)

func TestProjectPickerAppliesOpenConfig(t *testing.T) {
	// Given
	var closed bool

	backing := &fynetest.StubPicker{OpenPath: "/tmp/project.deckcheck"}
	projectPicker, err := picker.NewProject(backing)
	require.NoError(t, err)

	// When
	var selected string
	projectPicker.Open(browse.OpenOptions{
		Title: "Open Item",
		OnClosed: func() {
			closed = true
		},
	}, func(path string) {
		selected = path
	})

	// Then
	require.Len(t, backing.OpenCalls, 1)
	got := backing.OpenCalls[0]
	require.Equal(t, "Open DeckCheck Project", got.Title)
	require.Equal(t, projectFilters(), got.Filters)
	require.Equal(t, "/tmp/project.deckcheck", selected)
	require.True(t, closed)
}

func TestProjectPickerAppliesSaveConfig(t *testing.T) {
	// Given
	var closed bool

	backing := &fynetest.StubPicker{SavePath: "/tmp/project.deckcheck"}
	projectPicker, err := picker.NewProject(backing)
	require.NoError(t, err)

	// When
	var selected string
	projectPicker.Save(browse.SaveOptions{
		Filename: "My Project.deckcheck",
		OnClosed: func() {
			closed = true
		},
	}, func(path string) {
		selected = path
	})

	// Then
	require.Len(t, backing.SaveCalls, 1)
	got := backing.SaveCalls[0]
	require.Equal(t, "Save DeckCheck Project", got.Title)
	require.Equal(t, "My Project.deckcheck", got.Filename)
	require.Equal(t, projectFilters(), got.Filters)
	require.True(t, got.ConfirmOverwrite)
	require.Equal(t, "/tmp/project.deckcheck", selected)
	require.True(t, closed)
}

func TestNewProjectReturnsErrorWhenBasePickerMissing(t *testing.T) {
	// When / Then
	_, err := picker.NewProject(nil)
	require.ErrorIs(t, err, picker.ErrMissingPicker)
}

func projectFilters() browse.FileFilters {
	return browse.FileFilters{
		{
			Name:     "DeckCheck Project",
			Patterns: []string{"*" + project.FileExtension},
			CaseFold: true,
		},
	}
}
