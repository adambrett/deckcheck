package picker

import (
	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/browse"
)

// CSVFilters returns the standard file filter for CSV pickers.
func CSVFilters() browse.FileFilters {
	return browse.FileFilters{
		{Name: lang.L("CSV Files"), Patterns: []string{"*.csv"}, CaseFold: true},
	}
}
