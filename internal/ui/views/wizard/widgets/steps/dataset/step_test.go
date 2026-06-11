//go:build integration

package dataset_test

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fynetest"
	datasetstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/dataset"
)

func TestStepValidationDataAndReset(t *testing.T) {
	// Given
	test.NewApp()
	step := datasetstep.New(datasetstep.Handlers{})

	require.Equal(t, "Select dataset", step.Title())
	require.NotNil(t, step.Container())
	require.Equal(t, dataset.TypeCSV, step.SelectedType())
	require.False(t, step.Validate())

	// When
	step.SetPath("/tmp/data.csv")

	// Then
	require.True(t, step.Validate())
	require.Equal(t, "/tmp/data.csv", step.Data().Path)

	// When switching to a CSV-with-images source
	fynetest.SelectRadio(t, step.Container(), "CSV with Image References")
	step.SetCSVHeaders([]string{"front", "image"})

	// Then the image column is required before the step validates.
	require.False(t, step.Validate())

	fynetest.SelectOption(t, step.Container(), "image")

	data := step.Data()
	require.True(t, step.Validate())
	require.Equal(t, dataset.TypeCSVWithImage, data.Type)
	require.Equal(t, "image", data.ImageColumn)

	// When switching to an image folder
	fynetest.SelectRadio(t, step.Container(), "Image Folder")

	// Then the image column no longer applies.
	data = step.Data()
	require.Equal(t, dataset.TypeImages, data.Type)
	require.Empty(t, data.ImageColumn)

	// When
	step.Reset()

	// Then
	data = step.Data()
	require.Equal(t, dataset.TypeCSV, data.Type)
	require.Empty(t, data.Path)
}

func TestStepClearsStaleImageColumn(t *testing.T) {
	// Given
	test.NewApp()
	step := datasetstep.New(datasetstep.Handlers{})
	fynetest.SelectRadio(t, step.Container(), "CSV with Image References")
	step.SetCSVHeaders([]string{"image"})
	fynetest.SelectOption(t, step.Container(), "image")

	// When the path changes, the old headers no longer apply.
	step.SetPath("/tmp/other.csv")

	// Then
	require.Empty(t, step.Data().ImageColumn)

	// When new headers arrive that don't include the old selection
	step.SetCSVHeaders([]string{"thumbnail"})
	fynetest.SelectOption(t, step.Container(), "thumbnail")
	step.SetCSVHeaders([]string{"preview"})

	// Then the stale selection is cleared rather than silently kept.
	require.Empty(t, step.Data().ImageColumn)
}

func TestStepBrowseButtonReportsToOwner(t *testing.T) {
	// Given
	test.NewApp()
	browsed := false
	step := datasetstep.New(datasetstep.Handlers{Browse: func() { browsed = true }})

	// When
	fynetest.TapButton(t, step.Container(), "Choose file…")

	// Then
	require.True(t, browsed)
}
