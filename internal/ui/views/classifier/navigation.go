package classifier

import (
	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/deckcheck/internal/usererror"
)

// SetUnclassifiedOnly configures navigation to jump between unclassified records.
func (v *View) SetUnclassifiedOnly(enabled bool) {
	v.unclassifiedOnly = enabled
	v.updateNavigation()
}

// Next moves to the next record.
func (v *View) Next() {
	if v.unclassifiedOnly {
		index, found, err := v.project.NextUnclassified(v.ctx(), v.currentIndex+1)
		if err != nil {
			v.statusLabel.SetText(lang.L("Failed to find next unclassified record"))
			v.showError(usererror.Wrap("DC13", usererror.ErrFindNextUnclassifiedRecord, err))
			return
		}
		if !found {
			v.reportEndOfDataset(lang.L("No later unclassified records"))
			return
		}

		v.loadRecordOrReport(index)
		return
	}

	if v.currentIndex < v.totalRecords-1 {
		v.loadRecordOrReport(v.currentIndex + 1)
	} else {
		v.reportEndOfDataset(lang.L("End of dataset"))
	}
}

// reportEndOfDataset writes a terminal navigation message to the status
// label, upgrading the wording to a celebratory "All records
// classified. Export your results when ready." when nothing remains
// outstanding. fallback is used when the progress probe fails or when
// some records are still unclassified.
func (v *View) reportEndOfDataset(fallback string) {
	classified, total, err := v.project.Progress(v.ctx())
	if err == nil && total > 0 && classified == total {
		v.statusLabel.SetText(lang.L("All records classified. Export your results when ready."))
		return
	}
	v.statusLabel.SetText(fallback)
}

// Previous moves to the previous record.
func (v *View) Previous() {
	if v.unclassifiedOnly {
		index, found, err := v.project.PreviousUnclassified(v.ctx(), v.currentIndex)
		if err != nil {
			v.statusLabel.SetText(lang.L("Failed to find previous unclassified record"))
			v.showError(usererror.Wrap("DC14", usererror.ErrFindPreviousUnclassifiedRecord, err))
			return
		}
		if !found {
			v.statusLabel.SetText(lang.L("No earlier unclassified records"))
			return
		}

		v.loadRecordOrReport(index)
		return
	}

	if v.currentIndex > 0 {
		v.loadRecordOrReport(v.currentIndex - 1)
	}
}

// Skip skips the current record without classifying.
func (v *View) Skip() {
	v.Next()
}

// CanGoPrevious reports whether navigation backwards is possible. Probe
// errors read as "cannot navigate"; the actual navigation calls surface
// errors to the user, so this stays a silent availability check.
func (v *View) CanGoPrevious() bool {
	if v.unclassifiedOnly {
		_, found, err := v.project.PreviousUnclassified(v.ctx(), v.currentIndex)
		return err == nil && found
	}

	return v.currentIndex > 0
}

// CanGoNext reports whether navigation forwards is possible. Probe
// errors read as "cannot navigate"; the actual navigation calls surface
// errors to the user, so this stays a silent availability check.
func (v *View) CanGoNext() bool {
	if v.unclassifiedOnly {
		_, found, err := v.project.NextUnclassified(v.ctx(), v.currentIndex+1)
		return err == nil && found
	}

	return v.currentIndex < v.totalRecords-1
}

func (v *View) updateNavigation() {
	v.toolbar.EnablePrevious(v.CanGoPrevious())
	v.toolbar.EnableSkip(v.CanGoNext())
}

func (v *View) loadRecordOrReport(index int) {
	if err := v.LoadRecord(index); err != nil {
		v.statusLabel.SetText(lang.L("Failed to load record"))
		v.showError(usererror.Wrap("DC15", usererror.ErrLoadRecord, err))
	}
}
