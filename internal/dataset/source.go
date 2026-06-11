package dataset

import (
	"context"
	"fmt"
	"iter"
	"strings"
)

// RawRecord is one record produced by a Source: the per-column data
// and an optional resolved image path. Persistence belongs to the
// consumer.
//
// Data values are strings because every Source the project ships with
// reads from text inputs (CSV cells or filenames); a richer encoding
// would not round-trip safely through the .deckcheck file. Type-aware
// rendering is the export layer's problem.
type RawRecord struct {
	// Columns preserves the original source-column order. Map iteration is
	// deliberately not used for export ordering. The slice may be shared
	// between every record yielded by one iteration; callers must not
	// mutate it.
	Columns []string

	Data      map[string]string
	ImagePath string
}

// Source is a stream of RawRecords. Records returns a single-pass
// iterator; if reading fails the iterator yields a non-nil error and
// stops.
//
// If a Source has no records at all, the first yield carries a
// sentinel error (e.g. [ErrEmptyCSV] or [ErrNoImages]) so callers can
// react before opening a transaction.
type Source interface {
	Records(context.Context) iter.Seq2[RawRecord, error]
}

// NewSource constructs a Source for the given dataset type. The
// imageColumn is only used for [TypeCSVWithImage].
func NewSource(datasetType Type, path string, imageColumn string) (Source, error) {
	switch datasetType {
	case TypeCSV:
		return NewCSV(path), nil
	case TypeCSVWithImage:
		imageColumn = strings.TrimSpace(imageColumn)
		if imageColumn == "" {
			return nil, ErrMissingImageColumn
		}
		return NewImageCSV(path, imageColumn), nil
	case TypeImages:
		return NewImageDirectory(path), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDatasetType, datasetType)
	}
}
