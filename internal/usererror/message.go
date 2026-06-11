package usererror

import (
	"errors"
	"io/fs"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/db"
	"github.com/adambrett/deckcheck/internal/projectfile"
)

// Severity classifies how a routed message should be emphasised.
type Severity string

// SeverityError is the severity tag attached to every routed failure
// message.
const SeverityError Severity = "Error"

// Message is the rendered representation of an error ready to drop
// into a dialog. Severity and Code are rendered structurally by the
// dialog; Description, Impact, and Resolution are its body paragraphs.
// Technical carries the raw underlying error chain so a diagnostic
// build can surface it without changing what end users normally see.
type Message struct {
	Severity    Severity
	Code        string
	Summary     string
	Description string
	Impact      string
	Resolution  string
	Technical   string
}

// ForError maps an error to the [Message] the UI should display. nil
// returns a zero Message. Technical is always populated from err so
// diagnostic builds can surface the raw chain even when the routed
// message hides it.
//
// The Code is taken from the outermost [Coder] in the chain
// (typically a [Wrap] at the call site); the user-facing copy is
// selected by the routing table below.
func ForError(err error) Message {
	if err == nil {
		return Message{}
	}

	msg := routedMessage(err)
	msg.Severity = SeverityError
	msg.Code = CodeOf(err)
	msg.Technical = err.Error()

	return msg
}

func routedMessage(err error) Message {
	for _, entry := range routes {
		if entry.matches(err) {
			return entry.msg
		}
	}

	return Message{
		Summary:     "DeckCheck hit a problem",
		Description: "DeckCheck could not complete the action.",
		Impact:      "Any completed work before this message is still saved.",
		Resolution:  "Try the action again. If it keeps failing, contact support and include this code.",
	}
}

// route pairs a matcher predicate with the Message to render when it
// fires. routes is iterated in order, so earlier entries take
// precedence over later ones.
type route struct {
	matches func(error) bool
	msg     Message
}

// isAny returns a matcher that fires when err matches any of the
// supplied sentinels via errors.Is.
func isAny(targets ...error) func(error) bool {
	return func(err error) bool {
		for _, target := range targets {
			if errors.Is(err, target) {
				return true
			}
		}

		return false
	}
}

// isAll returns a matcher that fires when err matches every sentinel
// via errors.Is. Used for two-axis chains such as "ErrOpenProject
// wraps fs.ErrNotExist", where the message depends on both the
// operation and the underlying syscall error.
func isAll(targets ...error) func(error) bool {
	return func(err error) bool {
		for _, target := range targets {
			if !errors.Is(err, target) {
				return false
			}
		}

		return true
	}
}

// routes is the single ordered routing table. Order matters twice
// over: specific underlying conditions (a malformed CSV, a too-new
// schema) must outrank the broad operation-tag entries below them,
// and the two-axis fs.ErrNotExist + ErrOpenProject match must come
// before the bare ErrOpenProject fallback. Anything that matches no
// entry gets the generic copy from routedMessage.
var routes = []route{
	{
		matches: isAny(projectfile.ErrInvalidProjectFile, db.ErrNotDeckCheckProject),
		msg: Message{
			Summary:     "Could not open project",
			Description: "The selected file is not a DeckCheck project.",
			Impact:      "No project was opened. The file was not changed.",
			Resolution:  "Choose a .deckcheck project file created by DeckCheck.",
		},
	},
	{
		matches: isAny(db.ErrSchemaTooNew),
		msg: Message{
			Summary:     "Could not open project",
			Description: "This project was created by a newer version of DeckCheck.",
			Impact:      "No project was opened. The file was not changed.",
			Resolution:  "Update DeckCheck, then open the project again.",
		},
	},
	{
		matches: isAll(fs.ErrNotExist, ErrOpenProject),
		msg: Message{
			Summary:     "Could not open project",
			Description: "The project file could not be found.",
			Impact:      "No project was opened.",
			Resolution:  "Choose an existing .deckcheck project file.",
		},
	},
	{
		matches: isAny(dataset.ErrEmptyCSV),
		msg: Message{
			Summary:     "Could not create project",
			Description: "The selected CSV does not contain any data rows.",
			Impact:      "No project was created.",
			Resolution:  "Choose a CSV with a header row and at least one data row.",
		},
	},
	{
		matches: isAny(dataset.ErrInvalidCSVHeader),
		msg: Message{
			Summary:     "Could not create project",
			Description: "The selected CSV has column names DeckCheck cannot import safely.",
			Impact:      "No project was created.",
			Resolution:  "Make sure every column has a unique, non-empty name, then try again.",
		},
	},
	{
		matches: isAny(dataset.ErrInvalidCSVRow),
		msg: Message{
			Summary:     "Could not create project",
			Description: "One of the CSV rows has a different number of fields than the header row.",
			Impact:      "No project was created.",
			Resolution:  "Fix the CSV so every row matches the header columns, then try again.",
		},
	},
	{
		matches: isAny(dataset.ErrMissingImageColumn),
		msg: Message{
			Summary:     "Could not create project",
			Description: "The image column was not selected for the CSV image project.",
			Impact:      "No project was created.",
			Resolution:  "Select the CSV column that contains image paths.",
		},
	},
	{
		matches: isAny(dataset.ErrImageColumnNotFound),
		msg: Message{
			Summary:     "Could not create project",
			Description: "The selected CSV does not contain the chosen image path column.",
			Impact:      "No project was created.",
			Resolution:  "Select a column that exists in the CSV header row.",
		},
	},
	{
		matches: isAny(dataset.ErrNoImages),
		msg: Message{
			Summary:     "Could not create project",
			Description: "The selected folder does not contain any supported image files.",
			Impact:      "No project was created.",
			Resolution:  "Choose a folder containing PNG, JPEG, GIF, BMP, or WebP images.",
		},
	},
	{
		matches: isAny(projectfile.ErrInvalidQuestion),
		msg: Message{
			Summary:     "Could not create project",
			Description: "One of your questions is not valid. This usually means an answer was repeated, an answer was blank, or the question had fewer than two answers.",
			Impact:      "No project was created.",
			Resolution:  "Go back to the questions step, fix the affected row, and try again.",
		},
	},
	{
		matches: isAny(ErrReadCSVHeaders),
		msg: Message{
			Summary:     "Could not read CSV columns",
			Description: "DeckCheck could not read the header row from the selected CSV.",
			Impact:      "The project wizard stayed open. No project was created.",
			Resolution:  "Choose a readable CSV file with a header row.",
		},
	},
	{
		// The setup-invalid case must come ahead of the broader create
		// entry, otherwise the outer create wrap wins the errors.Is
		// walk and a legitimate setup problem is reported as a generic
		// create failure.
		matches: isAny(dataset.ErrUnsupportedDatasetType, projectfile.ErrInvalidProjectOptions),
		msg: Message{
			Summary:     "Could not create project",
			Description: "DeckCheck produced an invalid project setup.",
			Impact:      "No project was created.",
			Resolution:  "Go back through the wizard and try again. If it keeps failing, contact support and include this code.",
		},
	},
	{
		matches: isAny(ErrCreateProject),
		msg: Message{
			Summary:     "Could not create project",
			Description: "DeckCheck could not finish creating the project.",
			Impact:      "No project was opened. Existing project files were not changed.",
			Resolution:  "Try creating the project again. If it keeps failing, contact support and include this code.",
		},
	},
	{
		matches: isAny(ErrOpenProject),
		msg: Message{
			Summary:     "Could not open project",
			Description: "DeckCheck could not read the selected project file.",
			Impact:      "No project was opened. The file was not changed.",
			Resolution:  "Try opening the project again. If it keeps failing, contact support and include this code.",
		},
	},
	{
		matches: isAny(ErrLoadQuestions),
		msg: Message{
			Summary:     "Could not open project",
			Description: "DeckCheck opened the project file but could not load its questions.",
			Impact:      "The project was not shown. The file was not changed.",
			Resolution:  "Close DeckCheck and try opening the project again.",
		},
	},
	{
		matches: isAny(ErrInitializeClassifier),
		msg: Message{
			Summary:     "Could not open classifier",
			Description: "DeckCheck could not prepare the classifier screen.",
			Impact:      "The project was not shown. The file was not changed.",
			Resolution:  "Close DeckCheck and try opening the project again.",
		},
	},
	{
		matches: isAny(ErrFindFirstUnclassifiedRecord, ErrFindNextUnclassifiedRecord, ErrFindPreviousUnclassifiedRecord),
		msg: Message{
			Summary:     "Could not move through records",
			Description: "DeckCheck could not find the next record to show.",
			Impact:      "Your saved classifications were not changed.",
			Resolution:  "Try moving to another record again.",
		},
	},
	{
		matches: isAny(ErrLoadRecord),
		msg: Message{
			Summary:     "Could not load record",
			Description: "DeckCheck could not load the selected record.",
			Impact:      "Your saved classifications were not changed.",
			Resolution:  "Try moving to another record. If it keeps failing, reopen the project.",
		},
	},
	{
		matches: isAny(ErrLoadProgress),
		msg: Message{
			Summary:     "Could not update progress",
			Description: "DeckCheck could not refresh the classification progress count.",
			Impact:      "Your saved classifications were not changed.",
			Resolution:  "Continue classifying, or reopen the project if the count does not update.",
		},
	},
	{
		matches: isAny(ErrSaveClassification),
		msg: Message{
			Summary:     "Could not save classification",
			Description: "DeckCheck could not save the selected answer.",
			Impact:      "The answer was not saved.",
			Resolution:  "Try selecting the answer again.",
		},
	},
	{
		matches: isAny(ErrDeleteClassification),
		msg: Message{
			Summary:     "Could not remove classification",
			Description: "DeckCheck could not remove the selected answer.",
			Impact:      "The previous saved answer may still be present.",
			Resolution:  "Try clearing the answer again.",
		},
	},
	{
		matches: isAny(ErrExportCSV),
		msg: Message{
			Summary:     "Could not export CSV",
			Description: "DeckCheck could not write the export file.",
			Impact:      "No export file was completed. Your project was not changed.",
			Resolution:  "Choose another save location and try exporting again.",
		},
	},
}
