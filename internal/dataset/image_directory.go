package dataset

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"sort"
)

// FilenameColumn is the synthetic column name an [ImageDirectory] source
// gives each record. Consumers that special-case it (for example to avoid
// rendering the filename twice next to the image) must use this constant
// rather than re-spelling the literal.
const FilenameColumn = "filename"

// ImageDirectory is a dataset source backed by a folder of image files. Each
// supported image becomes a RawRecord with {FilenameColumn: <name>} and the
// full path set as the image path.
type ImageDirectory struct {
	path string
}

// NewImageDirectory creates a source that yields the supported images in path.
func NewImageDirectory(path string) *ImageDirectory {
	return &ImageDirectory{path: path}
}

// Records iterates each supported image in alphabetical order. The iterator
// yields [ErrNoImages] (with a zero RawRecord) when the directory contains
// no supported images.
func (d *ImageDirectory) Records(ctx context.Context) iter.Seq2[RawRecord, error] {
	return func(yield func(RawRecord, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(RawRecord{}, err)
			return
		}
		entries, err := os.ReadDir(d.path)
		if err != nil {
			yield(RawRecord{}, fmt.Errorf("read directory: %w", err))
			return
		}

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !IsImageFile(entry.Name()) {
				continue
			}
			names = append(names, entry.Name())
		}
		sort.Strings(names)

		if len(names) == 0 {
			yield(RawRecord{}, ErrNoImages)
			return
		}

		columns := []string{FilenameColumn}
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				yield(RawRecord{}, err)
				return
			}
			rec := RawRecord{
				Columns:   columns,
				Data:      map[string]string{FilenameColumn: name},
				ImagePath: filepath.Join(d.path, name),
			}
			if !yield(rec, nil) {
				return
			}
		}
	}
}
