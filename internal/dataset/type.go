package dataset

// Type identifies the kind of dataset attached to a project. The
// constants below are the only valid values and are also what gets
// persisted in the project row's dataset_type column.
type Type string

const (
	// TypeCSV is a plain CSV: every column is data, no images.
	TypeCSV Type = "csv"

	// TypeImages is a folder of image files; each image becomes one record.
	TypeImages Type = "images"

	// TypeCSVWithImage is a CSV in which one column names an image
	// path (relative paths resolve against the CSV's directory).
	TypeCSVWithImage Type = "csv_with_images"
)

// Valid reports whether t is one of the dataset types this binary
// understands. It is the single source of truth for the valid set, so
// validation logic elsewhere does not re-enumerate the constants.
func (t Type) Valid() bool {
	switch t {
	case TypeCSV, TypeImages, TypeCSVWithImage:
		return true
	}

	return false
}
