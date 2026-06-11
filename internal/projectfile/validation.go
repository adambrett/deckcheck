package projectfile

import (
	"fmt"
	"strings"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/project"
)

// normalizeCreateOptions validates opts and returns a normalised copy.
// The Questions slice is replaced rather than rewritten in place so the
// caller's slice is never mutated through the shared backing array.
func normalizeCreateOptions(opts CreateOptions) (CreateOptions, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		return opts, fmt.Errorf("%w: project name is required", ErrInvalidProjectOptions)
	}

	if err := validateDatasetOptions(opts.DatasetType, opts.ImageColumn); err != nil {
		return opts, err
	}
	opts.ImageColumn = strings.TrimSpace(opts.ImageColumn)

	questions := make([]project.QuestionDef, len(opts.Questions))
	for i, q := range opts.Questions {
		text, answers, err := normalizeQuestion(q.Text, q.Answers)
		if err != nil {
			return opts, fmt.Errorf("question %d: %w", i+1, err)
		}
		questions[i] = project.QuestionDef{Text: text, Answers: answers}
	}
	opts.Questions = questions

	return opts, nil
}

func validateDatasetOptions(datasetType dataset.Type, imageColumn string) error {
	if !datasetType.Valid() {
		return fmt.Errorf("%w: unsupported dataset type %q", ErrInvalidProjectOptions, datasetType)
	}

	imageColumn = strings.TrimSpace(imageColumn)
	if datasetType == dataset.TypeCSVWithImage {
		if imageColumn == "" {
			return fmt.Errorf("%w: image column is required for CSV image projects", ErrInvalidProjectOptions)
		}
		return nil
	}
	if imageColumn != "" {
		return fmt.Errorf("%w: image column is only valid for CSV image projects", ErrInvalidProjectOptions)
	}

	return nil
}

func normalizeQuestion(text string, answers []string) (string, []string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, fmt.Errorf("%w: question text is required", ErrInvalidQuestion)
	}

	out := make([]string, 0, len(answers))
	for _, answer := range answers {
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", nil, fmt.Errorf("%w: answer text is required", ErrInvalidQuestion)
		}
		out = append(out, answer)
	}

	if duplicate, found := project.FindDuplicateAnswer(out); found {
		return "", nil, fmt.Errorf("%w: duplicate answer %q", ErrInvalidQuestion, duplicate)
	}
	if len(out) < 2 {
		return "", nil, fmt.Errorf("%w: at least two answers are required", ErrInvalidQuestion)
	}

	return text, out, nil
}
