//go:build integration

package classifier_test

import (
	"context"
	"errors"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/fynetest"
	rootmocks "github.com/adambrett/deckcheck/internal/mocks"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/mocks"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier"
	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestViewActivateNavigateAnswerAndExport(t *testing.T) {
	// Given
	test.NewApp()

	questions := usefulQuestion()
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}
	record1 := &project.Record{ID: 2, Index: 1, Data: map[string]string{"name": "second"}}

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil)
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil)
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil)
	currentProject.EXPECT().Record(mock.Anything, 1).Return(record1, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 2, nil).Maybe()
	currentProject.EXPECT().SaveClassification(mock.Anything, 1, 10, 100).Return(nil)

	exporter := mocks.NewClassifierExporter(t)
	exporter.EXPECT().Write(mock.Anything, "/tmp/export.csv").Return(2, nil)

	picker := rootmocks.NewBrowsePicker(t)
	picker.EXPECT().Save(mock.Anything, mock.Anything).Run(func(options browse.SaveOptions, onSelected func(string)) {
		require.Equal(t, "Export CSV", options.Title)
		require.Equal(t, "export.csv", options.Filename)
		require.True(t, options.ConfirmOverwrite)
		onSelected("/tmp/export.csv")
	}).Return()

	var infoTitle string
	view := classifier.New(classifier.Config{
		Picker:    picker,
		Project:   currentProject,
		Questions: questions,
		Exporter:  exporter,
		Handlers: classifier.Handlers{
			Information: func(title, _ string) {
				infoTitle = title
			},
		},
	})

	// When / Then activation lands on the first unclassified record.
	require.NotNil(t, view.Container())
	require.Equal(t, fyne.NewSize(1024, 704), view.Size())
	require.NoError(t, view.Activate())
	fynetest.RequireLabel(t, view.Container(), "first")

	// When answering (waiting for the serialised write and the
	// auto-advance, which lands on the second record) and stepping
	// forward past the end
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()
	view.Next()

	// Then the second record is displayed.
	fynetest.RequireLabel(t, view.Container(), "second")

	// When exporting via the toolbar
	test.Tap(fynetest.ButtonWithText(t, view.Container(), "Export CSV"))
	view.WaitForPendingOperations()

	// Then
	require.Equal(t, "Export Complete", infoTitle)

	view.Close()
}

func TestViewUnclassifiedNavigationMessages(t *testing.T) {
	// Given a single fully-unclassified record with the filter on.
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil)
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil)
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 1).Return(0, false, nil)
	currentProject.EXPECT().PreviousUnclassified(mock.Anything, 0).Return(0, false, nil)

	view := classifier.New(classifier.Config{
		Picker:  nil,
		Project: currentProject,
		Questions: []project.Question{
			{ID: 1, Text: "Q?", Answers: []project.Answer{{ID: 1, Text: "A"}, {ID: 2, Text: "B"}}},
		},
		Exporter: nil,
		Handlers: classifier.Handlers{},
	})
	require.NoError(t, view.Activate())

	// When
	view.SetUnclassifiedOnly(true)

	// Then navigation is unavailable in both directions.
	require.False(t, view.CanGoPrevious())
	require.False(t, view.CanGoNext())

	// When stepping anyway
	view.Next()

	// Then the view stays put and explains why.
	fynetest.RequireLabel(t, view.Container(), "No later unclassified records")

	// When
	view.Previous()

	// Then
	fynetest.RequireLabel(t, view.Container(), "No earlier unclassified records")
}

func TestViewAutoAdvancesAfterAnsweringAllQuestions(t *testing.T) {
	// Given a two-record deck with a single question
	test.NewApp()

	questions := usefulQuestion()
	records := []*project.Record{
		{ID: 1, Index: 0, Data: map[string]string{"name": "first"}},
		{ID: 2, Index: 1, Data: map[string]string{"name": "second"}},
	}

	advanced := make(chan struct{})
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(len(records), nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(records[0], nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 1).RunAndReturn(func(_ context.Context, index int) (*project.Record, error) {
		close(advanced)
		return records[index], nil
	}).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, len(records), nil).Maybe()
	currentProject.EXPECT().SaveClassification(mock.Anything, 1, 10, 100).Return(nil).Once()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())
	defer view.Close()

	// When the last unanswered question is answered by key
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})

	// Then the view advances to the next record on its own after the
	// saved-flash delay. The timer registers with the lifecycle's
	// pending counter, so waiting for quiescence is deterministic.
	view.WaitForPendingOperations()
	select {
	case <-advanced:
	default:
		t.Fatal("auto-advance did not load the next record")
	}
}

func TestViewKeyboardNavigationMovesThroughRecords(t *testing.T) {
	// Given a two-record deck
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}, nil)
	currentProject.EXPECT().Record(mock.Anything, 1).Return(&project.Record{ID: 2, Index: 1, Data: map[string]string{"name": "second"}}, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 2, nil).Maybe()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())

	// When stepping right with the keyboard
	view.HandleKey(&fyne.KeyEvent{Name: fyne.KeyRight})

	// Then the second record shows.
	fynetest.RequireLabel(t, view.Container(), "second")

	// When stepping back left
	view.HandleKey(&fyne.KeyEvent{Name: fyne.KeyLeft})

	// Then the first record shows again.
	fynetest.RequireLabel(t, view.Container(), "first")
}

func TestViewReportsEndOfDataset(t *testing.T) {
	// Given a deck positioned on its final, not-yet-classified record
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "only"}}, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())

	// When stepping past the end
	view.HandleKey(&fyne.KeyEvent{Name: fyne.KeyRight})

	// Then the view stays put and says so.
	fynetest.RequireLabel(t, view.Container(), "End of dataset")
}

func TestViewCelebratesFullyClassifiedDeck(t *testing.T) {
	// Given a deck whose every record is classified
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, false, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0, Answers: map[int]int{10: 100}}, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(1, 1, nil).Maybe()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When
	require.NoError(t, view.Activate())

	// Then activation lands on the first record with the all-done hint.
	fynetest.RequireLabel(t, view.Container(), "All records classified")

	// When stepping past the end anyway
	view.HandleKey(&fyne.KeyEvent{Name: fyne.KeyRight})

	// Then the message upgrades to the export prompt.
	fynetest.RequireLabel(t, view.Container(), "All records classified. Export your results when ready.")
}

func TestViewActivateFailsWhenRecordCountFails(t *testing.T) {
	// Given a project whose record count probe fails
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(0, errors.New("boom")).Once()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: nil,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When
	err := view.Activate()

	// Then the raw error surfaces to the caller.
	require.ErrorContains(t, err, "boom")
}

func TestViewActivateWrapsFirstUnclassifiedScanFailure(t *testing.T) {
	// Given a project whose first-unclassified scan fails
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, false, errors.New("boom")).Once()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: nil,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When
	err := view.Activate()

	// Then the error carries the first-unclassified-scan tag.
	require.ErrorIs(t, err, usererror.ErrFindFirstUnclassifiedRecord)
}

func TestViewActivateFailsWhenFirstRecordLoadFails(t *testing.T) {
	// Given a project whose first record cannot be loaded
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(nil, errors.New("boom")).Once()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: nil,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When
	err := view.Activate()

	// Then
	require.ErrorContains(t, err, "boom")
}

func TestViewActivateWithoutRecordsDisablesExport(t *testing.T) {
	// Given an empty dataset
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(0, nil).Twice()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: nil,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When
	require.NoError(t, view.Activate())

	// Then there is nothing to classify or export.
	fynetest.RequireLabel(t, view.Container(), "No records to classify")
	require.True(t, fynetest.ButtonWithText(t, view.Container(), "Export CSV").Disabled())

	// When activating again (a reused view re-derives its context)
	require.NoError(t, view.Activate())

	// Then the empty state is unchanged.
	fynetest.RequireLabel(t, view.Container(), "No records to classify")
}

func TestViewWithParentContextScopesActivation(t *testing.T) {
	// Given a view rooted at an already-cancelled parent context
	test.NewApp()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().
		RecordCount(mock.Anything).
		RunAndReturn(func(callCtx context.Context) (int, error) {
			return 0, callCtx.Err()
		}).
		Once()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: nil,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	}, classifier.WithParentContext(ctx))

	// When
	err := view.Activate()

	// Then the view-scoped context inherited the cancellation.
	require.ErrorIs(t, err, context.Canceled)
}

func TestViewIgnoresAnswersBeforeARecordIsLoaded(t *testing.T) {
	// Given a view that has not been activated, so no record is current
	test.NewApp()

	currentProject := mocks.NewClassifierProject(t)
	questions := usefulQuestion()
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})

	// When / Then selecting an answer never touches the project (the
	// mock has no expectations, so any call would fail the test).
	require.NotPanics(t, func() {
		view.SelectAnswerByIndex(0)
	})
}

func TestViewSaveFailureRestoresSelectionsFromProject(t *testing.T) {
	// Given a record whose classification cannot be saved
	test.NewApp()

	questions := usefulQuestion()
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	// Loaded once on activation, re-fetched after each failed save to
	// restore selections.
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil).Times(3)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()
	currentProject.EXPECT().SaveClassification(mock.Anything, 1, 10, 100).Return(errors.New("boom")).Twice()

	var capturedErr error
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	})
	require.NoError(t, view.Activate())

	// When answering
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()

	// Then the failure is reported and selections are re-read from the project.
	fynetest.RequireLabel(t, view.Container(), "Failed to save")
	require.ErrorIs(t, capturedErr, usererror.ErrSaveClassification)

	// And the rollback visibly reset the panel: the same key registers
	// as a fresh selection (a second save attempt), not a deselection;
	// a Delete call here would fail the mock.
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()
}

func TestViewDeleteFailureReportsEvenWhenRestoreFails(t *testing.T) {
	// Given an answered record whose classification cannot be deleted
	test.NewApp()

	questions := usefulQuestion()
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil).Once()
	// The post-failure restore fetch also fails; the delete status stays.
	currentProject.EXPECT().Record(mock.Anything, 0).Return(nil, errors.New("gone")).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()
	currentProject.EXPECT().SaveClassification(mock.Anything, 1, 10, 100).Return(nil).Once()
	currentProject.EXPECT().DeleteClassification(mock.Anything, 1, 10).Return(errors.New("boom")).Once()

	var capturedErr error
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	})
	require.NoError(t, view.Activate())
	defer view.Close()

	// When toggling the same answer off again (waiting between the
	// presses so the save delivers before the deselect)
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()

	// Then the delete failure is reported.
	fynetest.RequireLabel(t, view.Container(), "Failed to delete")
	require.ErrorIs(t, capturedErr, usererror.ErrDeleteClassification)
}

func TestViewDropsStaleWriteResultAfterNavigation(t *testing.T) {
	// Given a save on the first record that completes only after the
	// user has navigated to the second, fully-answered record
	test.NewApp()

	questions := usefulQuestion()
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}
	record1 := &project.Record{
		ID: 2, Index: 1,
		Data:    map[string]string{"name": "second"},
		Answers: map[int]int{10: 100},
	}

	release := make(chan struct{})
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(3, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 1).Return(record1, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(1, 3, nil).Maybe()
	currentProject.EXPECT().
		SaveClassification(mock.Anything, 1, 10, 100).
		RunAndReturn(func(context.Context, int, int, int) error {
			<-release
			return nil
		}).
		Once()
	// No Record(_, 2) expectation: a spurious auto-advance off the
	// second record would fail the mock.

	view := classifier.New(classifier.Config{Project: currentProject, Questions: questions})
	require.NoError(t, view.Activate())
	defer view.Close()

	// When answering the first record, moving on mid-write, then
	// letting the write land
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.Next()
	close(release)
	view.WaitForPendingOperations()

	// Then the stale result neither flashes nor advances: the user
	// stays on the record they navigated to.
	fynetest.RequireLabel(t, view.Container(), "second")
	fynetest.RequireNoLabel(t, view.Container(), "✓ Saved")
}

func TestViewRollbackOrdersAfterQueuedWrites(t *testing.T) {
	// Given two questions where the first save fails and the second,
	// already queued behind it, succeeds
	test.NewApp()

	questions := []project.Question{
		{ID: 10, Text: "First?", Answers: []project.Answer{{ID: 100, Text: "Yes"}, {ID: 101, Text: "No"}}},
		{ID: 20, Text: "Second?", Answers: []project.Answer{{ID: 200, Text: "A"}, {ID: 201, Text: "B"}}},
	}
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()
	// The first save blocks until both keypresses have queued, so the
	// second write is already in the serial queue when the failure's
	// rollback is scheduled; the exact interleaving under test.
	release := make(chan struct{})
	currentProject.EXPECT().
		SaveClassification(mock.Anything, 1, 10, 100).
		RunAndReturn(func(context.Context, int, int, int) error {
			<-release
			return errors.New("boom")
		}).
		Once()
	saveSecond := currentProject.EXPECT().SaveClassification(mock.Anything, 1, 20, 200).Return(nil).Once()
	// The rollback re-read must observe the second write's outcome,
	// so it must run after it; a snapshot taken before would wipe
	// the second answer's highlight.
	restoreRead := currentProject.EXPECT().
		Record(mock.Anything, 0).
		Return(&project.Record{ID: 1, Index: 0, Answers: map[int]int{20: 200}}, nil).
		Once()
	restoreRead.NotBefore(saveSecond)

	var capturedErr error
	view := classifier.New(classifier.Config{
		Project:   currentProject,
		Questions: questions,
		Handlers:  classifier.Handlers{Error: func(err error) { capturedErr = err }},
	})
	require.NoError(t, view.Activate())
	defer view.Close()

	// When answering both questions back to back
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	close(release)
	view.WaitForPendingOperations()

	// Then the first failure was reported, and the successful second
	// write cleared the failure status rather than being wiped by an
	// early rollback (the mock enforces the read-after-write order).
	require.ErrorIs(t, capturedErr, usererror.ErrSaveClassification)
	fynetest.RequireNoLabel(t, view.Container(), "Failed to save")
}

func TestViewDeselectingAnswerDeletesClassification(t *testing.T) {
	// Given a record with its single question answered
	test.NewApp()

	questions := usefulQuestion()
	record0 := &project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}

	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(record0, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()
	saveCall := currentProject.EXPECT().SaveClassification(mock.Anything, 1, 10, 100).Return(nil).Once()
	deleteCall := currentProject.EXPECT().DeleteClassification(mock.Anything, 1, 10).Return(nil).Once()
	deleteCall.NotBefore(saveCall)

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())
	defer view.Close()

	// When answering, waiting for the write (and the auto-advance,
	// which stays put on the deck's only record), then toggling the
	// same answer off
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()
	view.HandleKey(&fyne.KeyEvent{Name: fyne.Key1})
	view.WaitForPendingOperations()

	// Then the classification is deleted after the save (NotBefore
	// pins the order) and no saved flash remains.
	fynetest.RequireNoLabel(t, view.Container(), "✓ Saved")
}

func TestViewExportFailureReportsError(t *testing.T) {
	// Given an exporter that fails to write
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	exporter := mocks.NewClassifierExporter(t)
	exporter.EXPECT().Write(mock.Anything, "/tmp/export.csv").Return(0, errors.New("disk full")).Once()

	picker := rootmocks.NewBrowsePicker(t)
	picker.EXPECT().Save(mock.Anything, mock.Anything).Run(func(_ browse.SaveOptions, onSelected func(string)) {
		onSelected("/tmp/export.csv")
	}).Return()

	var capturedErr error
	view := classifier.New(classifier.Config{
		Picker:    picker,
		Project:   currentProject,
		Questions: questions,
		Exporter:  exporter,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	})
	require.NoError(t, view.Activate())

	// When exporting via the toolbar
	test.Tap(fynetest.ButtonWithText(t, view.Container(), "Export CSV"))
	view.WaitForPendingOperations()

	// Then the failure lands in the status label and the error handler.
	fynetest.RequireLabel(t, view.Container(), "Export failed")
	require.ErrorIs(t, capturedErr, usererror.ErrExportCSV)
}

func TestViewDropsExportResultAfterClose(t *testing.T) {
	// Given an export whose view closes while the write is in flight
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	var view *classifier.View
	exporter := mocks.NewClassifierExporter(t)
	exporter.EXPECT().
		Write(mock.Anything, "/tmp/export.csv").
		RunAndReturn(func(context.Context, string) (int, error) {
			view.Close()
			return 0, errors.New("boom")
		}).
		Once()

	picker := rootmocks.NewBrowsePicker(t)
	picker.EXPECT().Save(mock.Anything, mock.Anything).Run(func(_ browse.SaveOptions, onSelected func(string)) {
		onSelected("/tmp/export.csv")
	}).Return()

	errorCalled := false
	view = classifier.New(classifier.Config{
		Picker:    picker,
		Project:   currentProject,
		Questions: questions,
		Exporter:  exporter,
		Handlers: classifier.Handlers{
			Error: func(error) { errorCalled = true },
		},
	})
	require.NoError(t, view.Activate())

	// When exporting and closing mid-write
	test.Tap(fynetest.ButtonWithText(t, view.Container(), "Export CSV"))
	view.WaitForPendingOperations()

	// Then the late completion is dropped silently.
	require.False(t, errorCalled)
	fynetest.RequireLabel(t, view.Container(), "Exporting…")
}

func TestViewUnclassifiedOnlyNavigationMovesBetweenRecords(t *testing.T) {
	// Given two unclassified records with the filter on
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil)
	currentProject.EXPECT().NextUnclassified(mock.Anything, 1).Return(1, true, nil)
	currentProject.EXPECT().NextUnclassified(mock.Anything, 2).Return(0, false, nil)
	currentProject.EXPECT().PreviousUnclassified(mock.Anything, 0).Return(0, false, nil)
	currentProject.EXPECT().PreviousUnclassified(mock.Anything, 1).Return(0, true, nil)
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}, nil)
	currentProject.EXPECT().Record(mock.Anything, 1).Return(&project.Record{ID: 2, Index: 1, Data: map[string]string{"name": "second"}}, nil)
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 2, nil).Maybe()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())
	view.SetUnclassifiedOnly(true)

	// When stepping forward to the next unclassified record
	view.Next()

	// Then the second record shows.
	fynetest.RequireLabel(t, view.Container(), "second")

	// When stepping back
	view.Previous()

	// Then the first record shows again.
	fynetest.RequireLabel(t, view.Container(), "first")
}

func TestViewUnclassifiedOnlyNavigationFailuresSurfaceErrors(t *testing.T) {
	// Given unclassified scans that fail in both directions
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 1).Return(0, false, errors.New("boom"))
	currentProject.EXPECT().PreviousUnclassified(mock.Anything, 0).Return(0, false, errors.New("boom"))
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	var capturedErrs []error
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErrs = append(capturedErrs, err) },
		},
	})
	require.NoError(t, view.Activate())
	view.SetUnclassifiedOnly(true)

	// When stepping forward
	view.Next()

	// Then the scan failure is reported.
	fynetest.RequireLabel(t, view.Container(), "Failed to find next unclassified record")

	// When stepping back
	view.Previous()

	// Then the reverse scan failure is reported too.
	fynetest.RequireLabel(t, view.Container(), "Failed to find previous unclassified record")
	require.Len(t, capturedErrs, 2)
	require.ErrorIs(t, capturedErrs[0], usererror.ErrFindNextUnclassifiedRecord)
	require.ErrorIs(t, capturedErrs[1], usererror.ErrFindPreviousUnclassifiedRecord)
}

func TestViewReportsRecordLoadFailureOnNavigation(t *testing.T) {
	// Given a second record that cannot be loaded
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(2, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0, Data: map[string]string{"name": "first"}}, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 1).Return(nil, errors.New("boom")).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 2, nil).Maybe()

	var capturedErr error
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	})
	require.NoError(t, view.Activate())

	// When stepping to the broken record
	view.Next()

	// Then the load failure is reported and the first record stays up.
	fynetest.RequireLabel(t, view.Container(), "Failed to load record")
	require.ErrorIs(t, capturedErr, usererror.ErrLoadRecord)
	fynetest.RequireLabel(t, view.Container(), "first")
}

func TestViewLoadRecordIgnoresOutOfRangeIndexes(t *testing.T) {
	// Given a single-record view
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 1, nil).Maybe()

	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers:  classifier.Handlers{},
	})
	require.NoError(t, view.Activate())

	// When / Then out-of-range loads are ignored without project calls
	// (the mock would fail on an unexpected Record fetch).
	require.NoError(t, view.LoadRecord(-1))
	require.NoError(t, view.LoadRecord(1))
}

func TestViewReportsProgressLoadFailure(t *testing.T) {
	// Given a progress probe that fails
	test.NewApp()

	questions := usefulQuestion()
	currentProject := mocks.NewClassifierProject(t)
	currentProject.EXPECT().RecordCount(mock.Anything).Return(1, nil).Once()
	currentProject.EXPECT().NextUnclassified(mock.Anything, 0).Return(0, true, nil).Once()
	currentProject.EXPECT().Record(mock.Anything, 0).Return(&project.Record{ID: 1, Index: 0}, nil).Once()
	currentProject.EXPECT().Progress(mock.Anything).Return(0, 0, errors.New("boom")).Once()

	var capturedErr error
	view := classifier.New(classifier.Config{
		Picker:    nil,
		Project:   currentProject,
		Questions: questions,
		Exporter:  nil,
		Handlers: classifier.Handlers{
			Error: func(err error) { capturedErr = err },
		},
	})

	// When
	require.NoError(t, view.Activate())

	// Then the record still loads but the progress failure is reported.
	require.ErrorIs(t, capturedErr, usererror.ErrLoadProgress)
}

// usefulQuestion is the single-question fixture most tests share:
// question 10 "Useful?" answered by 100 Yes / 101 No.
func usefulQuestion() []project.Question {
	return []project.Question{{
		ID:      10,
		Text:    "Useful?",
		Answers: []project.Answer{{ID: 100, Text: "Yes"}, {ID: 101, Text: "No"}},
	}}
}
