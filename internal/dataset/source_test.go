package dataset_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
)

func TestNewSource(t *testing.T) {
	// Given
	tests := []struct {
		name        string
		datasetType dataset.Type
		requireType any
	}{
		{name: "csv", datasetType: dataset.TypeCSV, requireType: &dataset.CSV{}},
		{name: "image csv", datasetType: dataset.TypeCSVWithImage, requireType: &dataset.CSV{}},
		{name: "image directory", datasetType: dataset.TypeImages, requireType: &dataset.ImageDirectory{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			path := "/tmp/source"
			imageColumn := "image"

			// When
			source, err := dataset.NewSource(tt.datasetType, path, imageColumn)

			// Then
			require.NoError(t, err)
			require.IsType(t, tt.requireType, source)
		})
	}
}

func TestNewSourceRejectsUnsupportedType(t *testing.T) {
	// Given
	datasetType := dataset.Type("unknown")

	// When
	source, err := dataset.NewSource(datasetType, "/tmp/source", "")

	// Then
	require.Error(t, err)
	require.True(t, errors.Is(err, dataset.ErrUnsupportedDatasetType))
	require.Nil(t, source)
}

func TestNewSourceRejectsMissingImageColumn(t *testing.T) {
	// Given
	datasetType := dataset.TypeCSVWithImage

	// When
	source, err := dataset.NewSource(datasetType, "/tmp/source", " ")

	// Then
	require.Error(t, err)
	require.True(t, errors.Is(err, dataset.ErrMissingImageColumn))
	require.Nil(t, source)
}
