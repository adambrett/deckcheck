package export_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/export"
	"github.com/adambrett/deckcheck/internal/mocks"
	"github.com/adambrett/deckcheck/internal/project"
)

func TestCSVWrite(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	questions := []project.Question{
		{ID: 1, Text: "Sentiment?"},
	}
	writer := &bufferWriteCloser{}

	currentProject.EXPECT().Questions(mock.Anything).Return(questions, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"text", "value"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, questions).
		Return(classifiedRecords(
			project.ClassifiedRecord{
				Data:    map[string]string{"text": "Hello", "value": "100"},
				Answers: map[int]string{1: "Positive"},
			},
			project.ClassifiedRecord{
				Data:    map[string]string{"text": "World", "value": "200"},
				Answers: map[int]string{},
			},
		)).
		Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(path string) (io.WriteCloser, error) {
		require.Equal(t, "export.csv", path)
		return writer, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.NoError(t, err)
	require.Equal(t, 2, rows)
	require.Equal(t, "text,value,Sentiment?\nHello,100,Positive\nWorld,200,\n", writer.String())
	require.True(t, writer.closed)
}

func TestCSVWritePreservesSourceColumnOrder(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	writer := &bufferWriteCloser{}

	currentProject.EXPECT().Questions(mock.Anything).Return(nil, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"z", "a", "m"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, []project.Question(nil)).
		Return(classifiedRecords(project.ClassifiedRecord{
			Data:    map[string]string{"z": "last", "a": "first", "m": "middle"},
			Answers: map[int]string{},
		})).
		Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return writer, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, rows)
	require.Equal(t, "z,a,m\nlast,first,middle\n", writer.String())
}

func TestCSVWriteIncludesGridAnnotations(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	questions := []project.Question{
		{ID: 1, Kind: project.QuestionKindChoice, Text: "Sentiment?"},
		{ID: 2, Kind: project.QuestionKindImageGrid, Text: "Cars?", GridRows: 3, GridColumns: 3},
	}
	writer := &bufferWriteCloser{}
	imagePath := writePNG(t, 100, 100)

	currentProject.EXPECT().Questions(mock.Anything).Return(questions, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"image"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, questions).
		Return(classifiedRecords(project.ClassifiedRecord{
			Data:            map[string]string{"image": "street.jpg"},
			ImagePath:       imagePath,
			Answers:         map[int]string{1: "Positive"},
			GridAnnotations: map[int]string{2: "A1 C2"},
		})).
		Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return writer, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, rows)
	actualRows, err := csv.NewReader(bytes.NewReader(writer.Bytes())).ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"image", "Sentiment?", "Cars?", "Cars? pixels"},
		{
			"street.jpg",
			"Positive",
			"A1 C2",
			`[{"cell":"A1","x":0,"y":0,"width":33,"height":33},{"cell":"C2","x":66,"y":33,"width":34,"height":33}]`,
		},
	}, actualRows)
}

func TestCSVWriteIncludesEmptyGridPixelSelection(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	questions := []project.Question{
		{ID: 2, Kind: project.QuestionKindImageGrid, Text: "Cars?", GridRows: 3, GridColumns: 3},
	}
	writer := &bufferWriteCloser{}

	currentProject.EXPECT().Questions(mock.Anything).Return(questions, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"image"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, questions).
		Return(classifiedRecords(project.ClassifiedRecord{
			Data:            map[string]string{"image": "street.jpg"},
			GridAnnotations: map[int]string{2: ""},
		})).
		Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return writer, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, rows)
	actualRows, err := csv.NewReader(bytes.NewReader(writer.Bytes())).ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"image", "Cars?", "Cars? pixels"},
		{"street.jpg", "", "[]"},
	}, actualRows)
}

func TestCSVWriteReturnsCreateError(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	wantErr := errors.New("create failed")

	currentProject.EXPECT().Questions(mock.Anything).Return(nil, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"text"}).Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return nil, wantErr
	})).Write(context.Background(), "export.csv")

	// Then
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 0, rows)
}

func TestCSVWriteReturnsFlushError(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	wantErr := errors.New("disk is full")

	currentProject.EXPECT().Questions(mock.Anything).Return(nil, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"text"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, []project.Question(nil)).
		Return(classifiedRecords()).
		Once()

	// When
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return failingWriter{err: wantErr}, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 0, rows)
}

func TestCSVWriteRemovesPartialFileOnFailure(t *testing.T) {
	// Given a producer that fails after the first record, writing to a
	// real file via the default createFile.
	currentProject := mocks.NewExportProject(t)
	wantErr := errors.New("scan record failed")
	path := filepath.Join(t.TempDir(), "export.csv")

	currentProject.EXPECT().Questions(mock.Anything).Return(nil, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"text"}).Once()
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, []project.Question(nil)).
		Return(func(yield func(project.ClassifiedRecord, error) bool) {
			if !yield(project.ClassifiedRecord{
				Data: map[string]string{"text": "row1"}, Answers: map[int]string{},
			}, nil) {
				return
			}
			yield(project.ClassifiedRecord{}, wantErr)
		}).
		Once()

	// When
	_, err := export.New(currentProject).Write(context.Background(), path)

	// Then no truncated CSV is left at the user's chosen path.
	require.ErrorIs(t, err, wantErr)
	_, statErr := os.Stat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestCSVWriteClosesIteratorOnProducerError pins WR-09: the
// range-over-func producer in ClassifiedRecords releases its resources
// (the DB rows handle in production) when the consumer exits the loop
// early on a producer-emitted error. We assert the producer's
// deferred cleanup fires before Write returns.
func TestCSVWriteClosesIteratorOnProducerError(t *testing.T) {
	// Given
	currentProject := mocks.NewExportProject(t)
	wantErr := errors.New("scan record failed")

	currentProject.EXPECT().Questions(mock.Anything).Return(nil, nil).Once()
	currentProject.EXPECT().DataColumns().Return([]string{"text"}).Once()

	cleaned := false
	currentProject.EXPECT().
		ClassifiedRecords(mock.Anything, []project.Question(nil)).
		Return(func(yield func(project.ClassifiedRecord, error) bool) {
			defer func() { cleaned = true }()
			if !yield(project.ClassifiedRecord{
				Data: map[string]string{"text": "row1"}, Answers: map[int]string{},
			}, nil) {
				return
			}
			yield(project.ClassifiedRecord{}, wantErr)
		}).
		Once()

	// When
	writer := &bufferWriteCloser{}
	rows, err := export.New(currentProject, export.WithCreateFile(func(string) (io.WriteCloser, error) {
		return writer, nil
	})).Write(context.Background(), "export.csv")

	// Then
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, rows)
	require.True(t, cleaned, "iterator cleanup did not run after producer error")
}

func classifiedRecords(records ...project.ClassifiedRecord) iter.Seq2[project.ClassifiedRecord, error] {
	return func(yield func(project.ClassifiedRecord, error) bool) {
		for _, record := range records {
			if !yield(record, nil) {
				return
			}
		}
	}
}

type bufferWriteCloser struct {
	bytes.Buffer
	closed bool
}

func (w *bufferWriteCloser) Close() error {
	w.closed = true
	return nil
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func (w failingWriter) Close() error {
	return nil
}

func writePNG(t *testing.T, width, height int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "image.png")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	require.NoError(t, png.Encode(file, img))

	return path
}
