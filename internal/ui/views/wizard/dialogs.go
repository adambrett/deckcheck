package wizard

import (
	"context"

	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/picker"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// browseDataset is the dataset step's browse callback. After the user
// picks a path it is written back to the step; for CSV-with-images
// sources the CSV header row is probed off the Fyne goroutine so the
// image-column dropdown can offer real choices without stalling the
// window on a slow disk.
func (v *View) browseDataset() {
	selectedType := v.datasetStep.SelectedType()
	v.selectDatasetPath(selectedType, func(path string) {
		v.datasetStep.SetPath(path)
		if selectedType != dataset.TypeCSVWithImage {
			return
		}

		v.probeCSVHeaders(path)
	})
}

func (v *View) probeCSVHeaders(path string) {
	v.life.Go(func(ctx context.Context) func() {
		headers, err := dataset.NewCSV(path).LoadHeaders(ctx)

		return func() {
			if err != nil {
				v.showError(usererror.Wrap("DC08", usererror.ErrReadCSVHeaders, err))
				return
			}

			v.datasetStep.SetCSVHeaders(headers)
		}
	})
}

func (v *View) selectProjectPath(suggestedFilename string, onSelected func(path string)) {
	v.projectPicker.Save(browse.SaveOptions{
		Filename: suggestedFilename,
	}, onSelected)
}

func (v *View) selectDatasetPath(datasetType dataset.Type, onSelected func(path string)) {
	if datasetType == dataset.TypeImages {
		v.datasetPicker.Open(browse.OpenOptions{
			Title:  lang.L("Select Image Folder"),
			Folder: true,
		}, onSelected)
		return
	}

	v.datasetPicker.Open(browse.OpenOptions{
		Title:   lang.L("Select CSV File"),
		Filters: picker.CSVFilters(),
	}, onSelected)
}
