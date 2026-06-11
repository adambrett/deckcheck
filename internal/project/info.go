package project

import (
	"time"

	"github.com/adambrett/deckcheck/internal/dataset"
)

// Info is the metadata for a project. It is populated when the project is
// opened or created; persistence adapters may refresh DataColumns after a
// deferred dataset import.
type Info struct {
	// ID is the surrogate key of the project row.
	ID int

	// Name is the user-chosen project label, displayed in the title bar.
	Name string

	// DatasetType records how the source data was discovered.
	DatasetType dataset.Type

	// ImageColumn is the CSV column whose values resolve to image paths.
	// Empty for non-image datasets.
	ImageColumn string

	// DataColumns is the union of original-data field names across every
	// imported row, in the order they should appear when exporting.
	DataColumns []string

	// CreatedAt is the project creation timestamp.
	CreatedAt time.Time
}
