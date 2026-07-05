package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder for image.DecodeConfig
	_ "image/jpeg" // register JPEG decoder for image.DecodeConfig
	_ "image/png"  // register PNG decoder for image.DecodeConfig
	"io"
	"iter"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"  // register BMP decoder for image.DecodeConfig
	_ "golang.org/x/image/webp" // register WebP decoder for image.DecodeConfig

	"github.com/adambrett/deckcheck/internal/project"
)

// Project is the slice of an open project the exporter needs: the
// question set, the original column order, and a stream of classified
// records. *projectfile.Project satisfies it in production.
type Project interface {
	Questions(context.Context) ([]project.Question, error)
	DataColumns() []string
	ClassifiedRecords(context.Context, []project.Question) iter.Seq2[project.ClassifiedRecord, error]
}

// Option configures optional CSV behaviour.
type Option func(*options)

type options struct {
	createFile func(path string) (io.WriteCloser, error)
}

func defaultOptions() options {
	return options{createFile: defaultCreateFile}
}

// WithCreateFile replaces how the exporter opens its output for writing.
// It exists so tests can capture output and inject write failures without
// touching the filesystem.
func WithCreateFile(createFile func(path string) (io.WriteCloser, error)) Option {
	return func(options *options) {
		options.createFile = createFile
	}
}

func defaultCreateFile(path string) (io.WriteCloser, error) {
	return os.Create(path) //nolint:gosec // path is chosen by the user via the export file dialog
}

// CSV writes a project's classified data to a CSV file.
type CSV struct {
	project    Project
	createFile func(path string) (io.WriteCloser, error)
}

// New returns an exporter that pulls data from the given project.
func New(p Project, opts ...Option) *CSV {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &CSV{
		project:    p,
		createFile: options.createFile,
	}
}

// Write serialises every record of the project to path. The first
// column block is the original data in source order, followed by the
// classification columns. Image-grid questions also receive an adjacent
// pixel-bounds column. It returns the number of records written.
//
// On failure the partially written file is removed, so an aborted
// export never leaves a truncated CSV at the user's chosen path.
func (c *CSV) Write(ctx context.Context, path string) (count int, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, ctxErr
	}

	questions, err := c.project.Questions(ctx)
	if err != nil {
		return 0, fmt.Errorf("load questions: %w", err)
	}
	originalHeaders := c.project.DataColumns()

	file, err := c.createFile(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		_ = file.Close()
		// Failed exports must not leave a truncated file behind. The
		// remove is best-effort: with an injected createFile the path
		// may never have existed on disk.
		_ = os.Remove(path)
	}()

	writer := csv.NewWriter(file)

	headers := make([]string, 0, len(originalHeaders)+len(questions)*2)
	headers = append(headers, originalHeaders...)
	for _, question := range questions {
		headers = append(headers, question.Text)
		if question.Kind == project.QuestionKindImageGrid {
			headers = append(headers, question.Text+" pixels")
		}
	}

	if err := writer.Write(headers); err != nil {
		return 0, fmt.Errorf("write headers: %w", err)
	}

	for rec, err := range c.project.ClassifiedRecords(ctx, questions) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return count, ctxErr
		}
		if err != nil {
			return count, err
		}

		row := make([]string, 0, len(headers))
		for _, header := range originalHeaders {
			row = append(row, rec.Data[header])
		}
		for _, question := range questions {
			if question.Kind == project.QuestionKindImageGrid {
				selection, ok := rec.GridAnnotations[question.ID]
				row = append(row, selection)
				if !ok {
					row = append(row, "")
					continue
				}
				row = append(row, gridSelectionPixels(question, selection, rec.ImagePath))
				continue
			}
			row = append(row, rec.Answers[question.ID])
		}

		if err := writer.Write(row); err != nil {
			return count, fmt.Errorf("write row: %w", err)
		}
		count++
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return count, fmt.Errorf("flush csv: %w", err)
	}
	if err := file.Close(); err != nil {
		return count, fmt.Errorf("close file: %w", err)
	}
	closed = true

	return count, nil
}

func gridSelectionPixels(question project.Question, selection, imagePath string) string {
	if strings.TrimSpace(selection) == "" {
		return "[]"
	}

	width, height, err := imageDimensions(imagePath)
	if err != nil {
		return ""
	}

	value, err := project.FormatGridSelectionBoundsJSON(
		selection,
		question.GridRows,
		question.GridColumns,
		width,
		height,
	)
	if err != nil {
		return ""
	}
	return value
}

func imageDimensions(path string) (width, height int, err error) {
	file, err := os.Open(path) //nolint:gosec // project image paths are selected by the user during import
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = file.Close() }()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}
