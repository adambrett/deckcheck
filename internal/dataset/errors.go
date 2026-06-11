package dataset

import "errors"

// Sentinel errors that callers may compare with [errors.Is].
var (
	// ErrEmptyCSV indicates a CSV had a header but no data rows.
	ErrEmptyCSV = errors.New("empty CSV")

	// ErrInvalidCSVHeader indicates a CSV header row cannot be represented
	// safely in a DeckCheck project.
	ErrInvalidCSVHeader = errors.New("invalid CSV header")

	// ErrInvalidCSVRow indicates a CSV data row does not match the header
	// shape and would lose columns if imported.
	ErrInvalidCSVRow = errors.New("invalid CSV row")

	// ErrMissingImageColumn indicates an image-backed CSV source was
	// constructed without naming the column that contains image paths.
	ErrMissingImageColumn = errors.New("missing image column")

	// ErrImageColumnNotFound indicates an image-backed CSV source named a
	// column that does not exist in the CSV header.
	ErrImageColumnNotFound = errors.New("image column not found")

	// ErrNoImages indicates an image directory had no supported images.
	ErrNoImages = errors.New("no images")

	// ErrUnsupportedDatasetType is returned by [NewSource] when the
	// supplied [Type] is not one of the package's documented constants.
	ErrUnsupportedDatasetType = errors.New("unsupported dataset type")
)
