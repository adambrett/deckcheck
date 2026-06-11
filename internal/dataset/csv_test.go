package dataset_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
)

func TestCSVLoadHeaders(t *testing.T) {
	// Given
	path := writeFile(t, "test.csv", "name,age,email\ndata,data,data\n")

	// When
	headers, err := dataset.NewCSV(path).LoadHeaders(context.Background())

	// Then
	require.NoError(t, err)
	require.Equal(t, []string{"name", "age", "email"}, headers)
}

func TestCSVLoadHeadersHonoursCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	headers, err := dataset.NewCSV(filepath.Join(t.TempDir(), "missing.csv")).LoadHeaders(ctx)

	// Then
	require.Nil(t, headers)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCSVLoadHeadersReturnsOpenError(t *testing.T) {
	// When
	headers, err := dataset.NewCSV(filepath.Join(t.TempDir(), "missing.csv")).LoadHeaders(context.Background())

	// Then
	require.Nil(t, headers)
	require.Error(t, err)
}

func TestCSVRecordsYieldsRowsWithColumns(t *testing.T) {
	// Given
	path := writeFile(t, "test.csv", "name,value\nAlice,100\nBob,200\n")

	// When
	records, err := collectRecords(t, dataset.NewCSV(path))

	// Then
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, []string{"name", "value"}, records[0].Columns)
	require.Equal(t, "Alice", records[0].Data["name"])
	require.Equal(t, "200", records[1].Data["value"])
	require.Empty(t, records[0].ImagePath)
}

func TestImageCSVResolvesRelativeImagePaths(t *testing.T) {
	// Given
	csvDir := t.TempDir()
	path := filepath.Join(csvDir, "test.csv")
	require.NoError(t, os.WriteFile(path, []byte("text,image\nHello,img1.jpg\nWorld,img2.jpg\n"), 0o644))

	// When
	records, err := collectRecords(t, dataset.NewImageCSV(path, "image"))

	// Then relative paths resolve against the CSV's directory.
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, filepath.Join(csvDir, "img1.jpg"), records[0].ImagePath)
	require.Equal(t, filepath.Join(csvDir, "img2.jpg"), records[1].ImagePath)
}

func TestImageCSVAcceptsAbsoluteImagePath(t *testing.T) {
	// Given
	imagePath := filepath.Join(t.TempDir(), "img1.jpg")
	path := writeFile(t, "test.csv", "text,image\nHello,"+imagePath+"\n")

	// When
	records, err := collectRecords(t, dataset.NewImageCSV(path, "image"))

	// Then
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, imagePath, records[0].ImagePath)
}

func TestImageCSVRejectsMissingImageColumn(t *testing.T) {
	// Given
	path := writeFile(t, "test.csv", "text,image\nHello,img1.jpg\n")

	// When
	records, err := collectRecords(t, dataset.NewImageCSV(path, "missing"))

	// Then
	require.ErrorIs(t, err, dataset.ErrImageColumnNotFound)
	require.Empty(t, records)
}

func TestImageCSVLeavesBlankImageCellsEmpty(t *testing.T) {
	// Given
	path := writeFile(t, "test.csv", "text,image\nHello,\n")

	// When
	records, err := collectRecords(t, dataset.NewImageCSV(path, "image"))

	// Then
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Empty(t, records[0].ImagePath)
}

func TestCSVRequiresRows(t *testing.T) {
	// Given a CSV with a header but no data rows.
	path := writeFile(t, "empty.csv", "name,value\n")

	// When
	records, err := collectRecords(t, dataset.NewCSV(path))

	// Then
	require.ErrorIs(t, err, dataset.ErrEmptyCSV)
	require.Empty(t, records)
}

func TestCSVRejectsDuplicateHeaders(t *testing.T) {
	// Given
	path := writeFile(t, "duplicate.csv", "name,name\nAlice,Bob\n")

	// When
	headers, err := dataset.NewCSV(path).LoadHeaders(context.Background())

	// Then
	require.Nil(t, headers)
	require.ErrorIs(t, err, dataset.ErrInvalidCSVHeader)
}

func TestCSVRejectsBlankHeaders(t *testing.T) {
	// Given
	path := writeFile(t, "blank.csv", "name,\nAlice,Bob\n")

	// When
	yielded := 0
	var gotErr error
	for _, err := range dataset.NewCSV(path).Records(context.Background()) {
		yielded++
		gotErr = err
	}

	// Then the iterator yields the error exactly once and stops.
	require.Equal(t, 1, yielded)
	require.ErrorIs(t, gotErr, dataset.ErrInvalidCSVHeader)
}

func TestCSVRejectsRowsWithMissingColumns(t *testing.T) {
	// Given
	path := writeFile(t, "missing-column.csv", "name,value\nAlice\n")

	// When
	records, err := collectRecords(t, dataset.NewCSV(path))

	// Then
	require.ErrorIs(t, err, dataset.ErrInvalidCSVRow)
	require.Empty(t, records)
}

func TestCSVRejectsRowsWithExtraColumns(t *testing.T) {
	// Given
	path := writeFile(t, "extra-column.csv", "name,value\nAlice,100,extra\n")

	// When
	records, err := collectRecords(t, dataset.NewCSV(path))

	// Then
	require.ErrorIs(t, err, dataset.ErrInvalidCSVRow)
	require.Empty(t, records)
}

func TestCSVIncludesEmptyTrailingColumn(t *testing.T) {
	// Given
	path := writeFile(t, "empty-trailing-column.csv", "name,value\nAlice,\n")

	// When
	records, err := collectRecords(t, dataset.NewCSV(path))

	// Then the empty cell is present, not dropped.
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Contains(t, records[0].Data, "value")
	require.Empty(t, records[0].Data["value"])
}

func TestCSVRecordsHonoursCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	var gotErr error
	for _, err := range dataset.NewCSV(filepath.Join(t.TempDir(), "missing.csv")).Records(ctx) {
		gotErr = err
	}

	// Then
	require.ErrorIs(t, gotErr, context.Canceled)
}

func writeFile(t *testing.T, name string, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

// collectRecords drains a source and returns every yielded record plus
// the first error encountered, if any.
func collectRecords(t *testing.T, source dataset.Source) ([]dataset.RawRecord, error) {
	t.Helper()

	var records []dataset.RawRecord
	for rec, err := range source.Records(context.Background()) {
		if err != nil {
			return records, err
		}
		records = append(records, rec)
	}

	return records, nil
}
