package projectfile

import "errors"

var (
	// ErrInvalidClassification is returned when a row, question, and answer
	// do not belong to the same project/question relationship.
	ErrInvalidClassification = errors.New("invalid classification")

	// ErrInvalidProjectOptions is returned when a project create request
	// would persist invalid project metadata.
	ErrInvalidProjectOptions = errors.New("invalid project options")

	// ErrInvalidQuestion is returned when a question has no text, too few
	// answers, blank answers, or duplicate answer labels.
	ErrInvalidQuestion = errors.New("invalid question")

	// ErrInvalidProjectFile is returned when a .deckcheck file does not
	// contain exactly one project row.
	ErrInvalidProjectFile = errors.New("invalid project file")

	// ErrProjectClosed is returned when callers try to use a project after
	// Close has released its database connection.
	ErrProjectClosed = errors.New("project closed")

	// ErrRecordNotFound is returned by Record when no dataset row exists
	// at the requested index.
	ErrRecordNotFound = errors.New("record not found")
)
