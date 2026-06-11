package dataset

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
)

// CSV reads records from a CSV file. When imageColumn is non-empty, the value
// in that column of each row is resolved (relative paths join with the CSV's
// directory) and exposed as RawRecord.ImagePath.
type CSV struct {
	path               string
	imageColumn        string
	requireImageColumn bool
}

// NewCSV creates a CSV source with no image-column resolution.
func NewCSV(path string) *CSV {
	return &CSV{path: path}
}

// NewImageCSV creates a CSV source that resolves the named header to a row's
// image path.
func NewImageCSV(path string, imageColumn string) *CSV {
	return &CSV{
		path:               path,
		imageColumn:        strings.TrimSpace(imageColumn),
		requireImageColumn: true,
	}
}

// LoadHeaders reads just the first row of the CSV and returns the headers.
func (c *CSV) LoadHeaders(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(c.path) //nolint:gosec // path is chosen by the user via the dataset picker
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv headers: %w", err)
	}
	if err := validateHeaders(headers); err != nil {
		return nil, err
	}

	return headers, nil
}

// Records streams each row of the CSV as a RawRecord. The iterator yields
// [ErrEmptyCSV] (with a zero RawRecord) when the file has a header but no
// data rows.
func (c *CSV) Records(ctx context.Context) iter.Seq2[RawRecord, error] {
	return func(yield func(RawRecord, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(RawRecord{}, err)
			return
		}
		file, err := os.Open(c.path) //nolint:gosec // path is chosen by the user via the dataset picker
		if err != nil {
			yield(RawRecord{}, fmt.Errorf("open csv: %w", err))
			return
		}
		defer func() { _ = file.Close() }()

		reader := csv.NewReader(file)
		headers, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				yield(RawRecord{}, ErrEmptyCSV)
				return
			}
			yield(RawRecord{}, fmt.Errorf("read csv headers: %w", err))
			return
		}
		if headerErr := validateHeaders(headers); headerErr != nil {
			yield(RawRecord{}, headerErr)
			return
		}

		csvDir := filepath.Dir(c.path)
		imageColIdx, err := c.imageColumnIndex(headers)
		if err != nil {
			yield(RawRecord{}, err)
			return
		}

		// One copy for the whole iteration; see the RawRecord.Columns
		// contract. Copying per row costs one allocation per record for
		// data that never changes mid-file.
		columns := append([]string(nil), headers...)

		rowIndex := 0
		for {
			if err := ctx.Err(); err != nil {
				yield(RawRecord{}, err)
				return
			}
			row, err := reader.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					if rowIndex == 0 {
						yield(RawRecord{}, ErrEmptyCSV)
					}
					return
				}
				if errors.Is(err, csv.ErrFieldCount) {
					yield(RawRecord{}, fmt.Errorf("%w: data row %d has %d fields, expected %d", ErrInvalidCSVRow, rowIndex+1, len(row), len(headers)))
					return
				}
				yield(RawRecord{}, fmt.Errorf("read csv row %d: %w", rowIndex, err))
				return
			}

			rec := RawRecord{
				Columns: columns,
				Data:    make(map[string]string, len(headers)),
			}
			for j, header := range headers {
				if j < len(row) {
					rec.Data[header] = row[j]
				}
			}
			if imageColIdx >= 0 && imageColIdx < len(row) {
				imagePath := strings.TrimSpace(row[imageColIdx])
				if imagePath != "" {
					rec.ImagePath = resolveImagePath(csvDir, imagePath)
				}
			}

			if !yield(rec, nil) {
				return
			}
			rowIndex++
		}
	}
}

func (c *CSV) imageColumnIndex(headers []string) (int, error) {
	if !c.requireImageColumn {
		return -1, nil
	}
	if c.imageColumn == "" {
		return -1, ErrMissingImageColumn
	}
	for i, header := range headers {
		if header == c.imageColumn {
			return i, nil
		}
	}
	return -1, fmt.Errorf("%w: %s", ErrImageColumnNotFound, c.imageColumn)
}

func resolveImagePath(baseDir string, imagePath string) string {
	if filepath.IsAbs(imagePath) {
		return imagePath
	}
	return filepath.Join(baseDir, imagePath)
}

func validateHeaders(headers []string) error {
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		if strings.TrimSpace(header) == "" {
			return fmt.Errorf("%w: blank column name", ErrInvalidCSVHeader)
		}
		if _, ok := seen[header]; ok {
			return fmt.Errorf("%w: duplicate column %q", ErrInvalidCSVHeader, header)
		}
		seen[header] = struct{}{}
	}

	return nil
}
