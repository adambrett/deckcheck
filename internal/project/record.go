package project

// Record is a single dataset row read back from a project, with any
// classifications already recorded against it. Data is the per-column
// string content as imported.
type Record struct {
	ID    int
	Index int

	Data      map[string]string
	ImagePath string

	// Answers maps question id to the chosen answer id, for questions
	// that have an answer recorded against this record.
	Answers map[int]int

	// GridAnnotations maps image-grid question id to the stored cell
	// selection. An empty string is meaningful: it records that no grid
	// cells were selected.
	GridAnnotations map[int]string
}

// HasImage reports whether the record has a resolved image path.
func (r *Record) HasImage() bool {
	return r.ImagePath != ""
}

// ClassifiedRecord is one dataset row paired with the human-readable
// answers recorded against it. Keys in Answers are question IDs; values
// are the matching answer text. Questions with no answer are absent.
type ClassifiedRecord struct {
	Data            map[string]string
	ImagePath       string
	Answers         map[int]string
	GridAnnotations map[int]string
}
