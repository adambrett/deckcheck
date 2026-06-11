package classifier

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/picker"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// Export starts the CSV export flow: the user picks a destination,
// then the exporter streams the classified records to it.
func (v *View) Export() {
	v.filePicker.Save(browse.SaveOptions{
		Title:            lang.L("Export CSV"),
		Filename:         "export.csv",
		ConfirmOverwrite: true,
		Filters:          picker.CSVFilters(),
	}, v.exportTo)
}

// exportTo streams the project to path off the Fyne goroutine, so a
// large export does not freeze the window; the user keeps classifying
// while it runs. Closing the view cancels the in-flight write through
// the view context.
func (v *View) exportTo(path string) {
	v.statusLabel.SetText(lang.L("Exporting…"))

	v.life.Go(func(ctx context.Context) func() {
		rows, err := v.exporter.Write(ctx, path)

		return func() {
			if err != nil {
				v.statusLabel.SetText(lang.L("Export failed"))
				v.showError(usererror.Wrap("DC10", usererror.ErrExportCSV, err))
				return
			}

			v.statusLabel.SetText("")
			v.showInformation(lang.L("Export Complete"), fmt.Sprintf(lang.L("Exported %d rows to %s"), rows, path))
		}
	})
}
