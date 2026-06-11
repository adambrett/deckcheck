package picker

import (
	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/project"
)

func projectFilters() browse.FileFilters {
	return browse.FileFilters{
		{
			Name:     lang.L("DeckCheck Project"),
			Patterns: []string{"*" + project.FileExtension},
			CaseFold: true,
		},
	}
}

// Project wraps a generic browse.Picker so Open/Save calls automatically use
// the DeckCheck project title, filename filter, and overwrite confirmation.
type Project struct {
	picker browse.Picker
}

// NewProject returns a picker that opens and saves .deckcheck project files.
func NewProject(picker browse.Picker) (Project, error) {
	if picker == nil {
		return Project{}, ErrMissingPicker
	}

	return Project{picker: picker}, nil
}

// Open shows the open dialog with the DeckCheck title and file filter
// applied, delegating everything else to the backing picker.
func (p Project) Open(options browse.OpenOptions, onSelected func(path string)) {
	options.Title = lang.L("Open DeckCheck Project")
	options.Filters = projectFilters()

	p.picker.Open(options, onSelected)
}

// Save shows the save dialog with the DeckCheck title, file filter, and
// overwrite confirmation applied, delegating everything else to the
// backing picker.
func (p Project) Save(options browse.SaveOptions, onSelected func(path string)) {
	options.Title = lang.L("Save DeckCheck Project")
	options.Filters = projectFilters()
	options.ConfirmOverwrite = true

	p.picker.Save(options, onSelected)
}
