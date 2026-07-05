package classifier

import (
	"context"

	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/deckcheck/internal/project"
	recordpanel "github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/panels/record"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// SelectAnswerByIndex selects the answer at the given index for the first unanswered question.
func (v *View) SelectAnswerByIndex(index int) {
	v.answerPanel.SelectAnswerByIndex(index)
}

// onAnswerSelected persists a selection change. The panel has already
// updated its highlight optimistically; the write runs through
// GoSerial so rapid keystrokes keep their order, and a failed write
// rolls the panel back from the database. Write contexts are detached
// from view cancellation (a confirmed selection must commit even if
// the view closes mid-write), and each apply step re-checks that its
// record is still on screen before touching the UI.
func (v *View) onAnswerSelected(questionID, answerID int, selected bool) {
	if v.currentRecord == nil {
		// Nothing to persist against; undo the panel's optimistic
		// highlight so the screen never shows a selection that was
		// not saved.
		v.answerPanel.SetSelections(nil)
		return
	}

	v.cancelPendingAdvance()
	recordID := v.currentRecord.ID
	recordIndex := v.currentIndex

	if !selected {
		v.life.GoSerial(func(ctx context.Context) func() {
			err := v.project.DeleteClassification(context.WithoutCancel(ctx), recordID, questionID)

			return func() {
				if v.currentRecord == nil || v.currentRecord.ID != recordID {
					return
				}
				if err != nil {
					v.statusLabel.SetText(lang.L("Failed to delete"))
					v.showError(usererror.Wrap("DC11", usererror.ErrDeleteClassification, err))
					v.restoreSelections(recordID, recordIndex)
					return
				}

				v.statusLabel.SetText("")
				v.updateProgress()
			}
		})
		return
	}

	v.life.GoSerial(func(ctx context.Context) func() {
		err := v.project.SaveClassification(context.WithoutCancel(ctx), recordID, questionID, answerID)

		return func() {
			if v.currentRecord == nil || v.currentRecord.ID != recordID {
				return
			}
			if err != nil {
				v.statusLabel.SetText(lang.L("Failed to save"))
				v.showError(usererror.Wrap("DC12", usererror.ErrSaveClassification, err))
				v.restoreSelections(recordID, recordIndex)
				return
			}

			if v.answerPanel.AllAnswered() {
				v.statusLabel.SetText(lang.L("✓ Saved"))
				v.updateProgress()
				v.scheduleAdvance()
				return
			}

			// Clear any stale failure message from an earlier write
			// now that this one has succeeded.
			v.statusLabel.SetText("")
		}
	})
}

// onGridSaved persists an image-grid selection. The panel has already
// marked the grid question answered optimistically; failed writes
// restore the panel from the project file, just like answer choices.
func (v *View) onGridSaved(questionID int, value string) {
	if v.currentRecord == nil {
		v.answerPanel.SetRecordState(nil, nil)
		return
	}

	v.cancelPendingAdvance()
	recordID := v.currentRecord.ID
	recordIndex := v.currentIndex

	v.life.GoSerial(func(ctx context.Context) func() {
		err := v.project.SaveGridAnnotation(context.WithoutCancel(ctx), recordID, questionID, value)

		return func() {
			if v.currentRecord == nil || v.currentRecord.ID != recordID {
				return
			}
			if err != nil {
				v.statusLabel.SetText(lang.L("Failed to save"))
				v.showError(usererror.Wrap("DC17", usererror.ErrSaveClassification, err))
				v.restoreSelections(recordID, recordIndex)
				return
			}

			if v.currentRecord.GridAnnotations == nil {
				v.currentRecord.GridAnnotations = make(map[int]string)
			}
			v.currentRecord.GridAnnotations[questionID] = value

			if v.answerPanel.AllAnswered() {
				v.statusLabel.SetText(lang.L("✓ Saved"))
				v.updateProgress()
				v.scheduleAdvance()
				return
			}

			v.statusLabel.SetText("")
		}
	})
}

func (v *View) onActiveQuestionChanged(question *project.Question) {
	if question == nil ||
		question.Kind != project.QuestionKindImageGrid ||
		v.currentRecord == nil ||
		!v.currentRecord.HasImage() {
		v.recordDisplay.ClearGrid()
		return
	}

	v.recordDisplay.SetGrid(recordpanel.GridConfig{
		Rows:      question.GridRows,
		Columns:   question.GridColumns,
		Selection: v.answerPanel.GridSelection(question.ID),
		Changed:   v.onGridSelectionChanged,
	})
}

func (v *View) onGridSelectionChanged(value string) {
	question := v.answerPanel.ActiveQuestion()
	if question == nil || question.Kind != project.QuestionKindImageGrid {
		return
	}

	v.answerPanel.SetGridSelection(question.ID, value)
	v.statusLabel.SetText("")
}

// restoreSelections re-reads the record after a failed write and
// resets the answer panel from it. The database is the single source
// of truth for selections; the view keeps no separate copy to roll
// back from. The re-read goes through GoSerial so it orders after any
// selection writes still queued behind the failed one (a snapshot
// taken earlier would wipe their optimistic highlights), and the
// apply step drops the rollback if the user has moved on.
func (v *View) restoreSelections(recordID, recordIndex int) {
	v.life.GoSerial(func(ctx context.Context) func() {
		record, err := v.project.Record(ctx, recordIndex)

		return func() {
			if err != nil {
				// The status label already explains the write failure;
				// a stale highlight is the lesser problem, so don't
				// stack a second dialog on top of the first.
				return
			}
			if v.currentRecord == nil || v.currentRecord.ID != recordID {
				return
			}

			v.currentRecord = record
			v.answerPanel.SetRecordState(record.Answers, record.GridAnnotations)
		}
	})
}

// scheduleAdvance arms a short-delay auto-advance to the next record.
// The delay lets the "✓ Saved" feedback flash before the view jumps
// on. The timer runs through the lifecycle so Close drops a late
// firing and tests can wait for it; the apply step re-checks the
// record and the all-answered state because a deselection can sneak
// in between the timer firing and the apply running.
func (v *View) scheduleAdvance() {
	v.cancelPendingAdvance()

	recordID := 0
	if v.currentRecord != nil {
		recordID = v.currentRecord.ID
	}
	// A stale cancel func left behind after the timer fires is a
	// harmless no-op (Stop on a fired timer does nothing).
	cancel := v.life.After(autoAdvanceDelay, func() {
		if v.currentRecord == nil || v.currentRecord.ID != recordID || !v.answerPanel.AllAnswered() {
			return
		}
		v.Next()
	})
	v.cancelAdvance.Store(&cancel)
}

func (v *View) cancelPendingAdvance() {
	if cancel := v.cancelAdvance.Swap(nil); cancel != nil {
		(*cancel)()
	}
}
