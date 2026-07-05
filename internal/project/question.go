package project

import "strings"

// QuestionKind describes how a question is answered.
type QuestionKind string

const (
	// QuestionKindChoice is the existing multiple-choice question type.
	QuestionKindChoice QuestionKind = "choice"
	// QuestionKindImageGrid asks the classifier to select cells from an
	// image overlay grid.
	QuestionKindImageGrid QuestionKind = "image_grid"
)

const (
	DefaultGridRows    = 3
	DefaultGridColumns = 3
	MinGridSize        = 2
	MaxGridSize        = 12
)

// Question is one classification question with its possible Answers. The
// IDs are stable across an open project and used to reference questions
// and answers in persisted classifications.
type Question struct {
	ID          int
	Kind        QuestionKind
	Text        string
	GridRows    int
	GridColumns int
	Answers     []Answer
}

// Answer is one selectable choice for a [Question].
type Answer struct {
	ID   int
	Text string
}

// QuestionDef describes a question and its answers before they are
// persisted. The wizard collects these from the user before project-file
// creation.
type QuestionDef struct {
	Kind        QuestionKind
	Text        string
	GridRows    int
	GridColumns int
	Answers     []string
}

// Normalized returns q with zero-value fields expanded to the defaults
// used by the wizard and project-file creation.
func (q QuestionDef) Normalized() QuestionDef {
	if q.Kind == "" {
		q.Kind = QuestionKindChoice
	}
	if q.Kind == QuestionKindImageGrid {
		if q.GridRows == 0 {
			q.GridRows = DefaultGridRows
		}
		if q.GridColumns == 0 {
			q.GridColumns = DefaultGridColumns
		}
	}
	return q
}

// Valid reports whether the question kind is one DeckCheck understands.
func (k QuestionKind) Valid() bool {
	switch k {
	case QuestionKindChoice, QuestionKindImageGrid:
		return true
	default:
		return false
	}
}

// ParseAnswers splits a comma-separated answer list into trimmed,
// non-empty answers. It is the single policy for turning user-typed
// answer text into a QuestionDef answer set, shared by the wizard and
// any future input surface.
func ParseAnswers(text string) []string {
	parts := strings.Split(text, ",")
	answers := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			answers = append(answers, trimmed)
		}
	}

	return answers
}

// FindDuplicateAnswer returns the first answer that repeats an earlier
// one when compared case-insensitively after trimming, and whether such
// a duplicate exists. Persistence rejects duplicate answers with this
// exact rule; the wizard uses it to surface the conflict before any
// file is created.
func FindDuplicateAnswer(answers []string) (string, bool) {
	seen := make(map[string]struct{}, len(answers))
	for _, answer := range answers {
		key := strings.ToLower(strings.TrimSpace(answer))
		if _, ok := seen[key]; ok {
			return answer, true
		}
		seen[key] = struct{}{}
	}

	return "", false
}
