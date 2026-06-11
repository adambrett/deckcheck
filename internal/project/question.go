package project

import "strings"

// Question is one classification question with its possible Answers. The
// IDs are stable across an open project and used to reference questions
// and answers in persisted classifications.
type Question struct {
	ID      int
	Text    string
	Answers []Answer
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
	Text    string
	Answers []string
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
