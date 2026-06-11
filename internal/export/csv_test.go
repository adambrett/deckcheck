package export_test

import (
	"bytes"
	"context"
	"errors"
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
