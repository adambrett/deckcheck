package dataset_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
)

func TestImageDirectoryYieldsSupportedImagesInOrder(t *testing.T) {
	// Given a folder with two images and one unsupported file.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "two.png"), []byte("fake"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.jpg"), []byte("fake"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0o644))

	// When
	records, err := collectRecords(t, dataset.NewImageDirectory(dir))

	// Then images yield alphabetically; the text file is skipped.
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "one.jpg", records[0].Data[dataset.FilenameColumn])
	require.Equal(t, filepath.Join(dir, "one.jpg"), records[0].ImagePath)
	require.Equal(t, "two.png", records[1].Data[dataset.FilenameColumn])
	require.Equal(t, []string{dataset.FilenameColumn}, records[0].Columns)
}

func TestImageDirectoryRequiresImages(t *testing.T) {
	// Given an empty folder.
	dir := t.TempDir()

	// When
	records, err := collectRecords(t, dataset.NewImageDirectory(dir))

	// Then
	require.ErrorIs(t, err, dataset.ErrNoImages)
	require.Empty(t, records)
}

func TestImageDirectoryRecordsHonoursCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	var gotErr error
	for _, err := range dataset.NewImageDirectory(t.TempDir()).Records(ctx) {
		gotErr = err
	}

	// Then
	require.ErrorIs(t, gotErr, context.Canceled)
}

func TestImageDirectoryRecordsReturnsReadError(t *testing.T) {
	// Given a path that is a file, not a directory.
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("text"), 0o644))

	// When
	var gotErr error
	for _, err := range dataset.NewImageDirectory(path).Records(context.Background()) {
		gotErr = err
	}

	// Then
	require.Error(t, gotErr)
}
