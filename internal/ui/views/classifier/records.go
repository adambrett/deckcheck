package classifier

import (
	"github.com/adambrett/deckcheck/internal/usererror"
)

// LoadRecord loads and displays a specific record.
func (v *View) LoadRecord(index int) error {
	if index < 0 || index >= v.totalRecords {
		return nil
	}
	v.cancelPendingAdvance()

	record, err := v.project.Record(v.ctx(), index)
	if err != nil {
		return err
	}

	v.currentIndex = index
	v.currentRecord = record

	v.recordDisplay.SetRecord(record)
	v.answerPanel.SetRecordState(record.Answers, record.GridAnnotations)
	v.statusLabel.SetText("")

	v.updateNavigation()
	v.updateProgress()

	return nil
}

func (v *View) updateProgress() {
	classified, total, err := v.project.Progress(v.ctx())
	if err != nil {
		v.showError(usererror.Wrap("DC09", usererror.ErrLoadProgress, err))
		return
	}

	v.toolbar.SetProgress(classified, total)
}
