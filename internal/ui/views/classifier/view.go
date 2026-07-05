package classifier

import (
	"context"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/panels/answers"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/panels/record"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/toolbar"
	"github.com/adambrett/deckcheck/internal/ui/views/lifecycle"
)

// RecordSource provides read access to the project's dataset rows.
type RecordSource interface {
	RecordCount(context.Context) (int, error)
	Record(context.Context, int) (*project.Record, error)
}

// Classifications persists the user's answers and reports progress.
type Classifications interface {
	SaveClassification(ctx context.Context, rowID, questionID, answerID int) error
	SaveGridAnnotation(ctx context.Context, rowID, questionID int, value string) error
	DeleteClassification(ctx context.Context, rowID, questionID int) error
	Progress(context.Context) (classified, total int, err error)
}

// Navigator finds unclassified records in either direction. The bool
// result reports whether one was found; exhaustion is not an error.
type Navigator interface {
	NextUnclassified(ctx context.Context, fromIndex int) (int, bool, error)
	PreviousUnclassified(ctx context.Context, beforeIndex int) (int, bool, error)
}

// Project is the classifier view's complete view of an open project.
// The concrete *projectfile.Project satisfies it in production; tests
// use a generated mock so the view can be exercised without a database.
type Project interface {
	RecordSource
	Classifications
	Navigator
}

// Exporter writes the current project to a selected path.
type Exporter interface {
	Write(context.Context, string) (int, error)
}

// Compile-time checks: classifier.View satisfies both views.View and views.Closer.
var (
	_ views.View     = (*View)(nil)
	_ views.Closer   = (*View)(nil)
	_ views.Quiescer = (*View)(nil)
)

const (
	viewWidth  = 1024
	viewHeight = 704

	// autoAdvanceDelay is how long the "✓ Saved" indicator stays
	// visible before the view jumps to the next record.
	autoAdvanceDelay = 300 * time.Millisecond
)

// View is the main view for classifying records.
type View struct {
	filePicker browse.Picker
	project    Project
	exporter   Exporter
	questions  []project.Question

	container *fyne.Container

	recordDisplay *record.Panel
	answerPanel   *answers.Panel
	toolbar       *toolbar.Toolbar
	statusLabel   *widget.Label

	currentIndex     int
	totalRecords     int
	currentRecord    *project.Record
	unclassifiedOnly bool

	// cancelAdvance disarms the pending auto-advance timer; nil when
	// no advance is scheduled. Atomic because the timer's apply step
	// can run Next -> LoadRecord -> cancelPendingAdvance on the timer
	// goroutine (the Fyne test driver executes DoAndWait inline) while
	// the Fyne goroutine is storing a fresh cancel func.
	cancelAdvance atomic.Pointer[func()]

	// life scopes every database/IO call this view makes so closing
	// the view cancels in-flight work, and drops late background
	// completions (the auto-advance timer, the CSV export) after
	// Close. Its context is populated by Activate and torn down by
	// Close.
	life *lifecycle.Lifecycle

	handlers Handlers
}

// Option configures optional view behaviour.
type Option func(*options)

type options struct {
	parentCtx context.Context
}

func defaultOptions() options {
	return options{}
}

// WithParentContext supplies the context the view derives its
// per-activation context from. Cancelling parent cancels every
// in-flight call the view has issued. If unset, the view roots its
// context at context.Background, which is fine for unit tests but
// leaves window-close cancellation up to View.Close.
func WithParentContext(parent context.Context) Option {
	return func(options *options) {
		options.parentCtx = parent
	}
}

// Handlers bundles the classifier's outward callbacks. Either field
// may be nil; the view guards before invoking.
type Handlers struct {
	Error       func(error)
	Information func(title, message string)
}

// Config holds the classifier view's dependencies. Project must
// already be opened and Questions must list the project's full
// question set (resolved once by the caller). Exporter writes that
// same project's records; the caller wires the two together.
type Config struct {
	Picker    browse.Picker
	Project   Project
	Questions []project.Question
	Exporter  Exporter
	Handlers  Handlers
}

// New constructs the classifier view from its dependencies.
func New(cfg Config, opts ...Option) *View {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	v := &View{
		filePicker: cfg.Picker,
		project:    cfg.Project,
		exporter:   cfg.Exporter,
		questions:  cfg.Questions,
		handlers:   cfg.Handlers,
		life:       lifecycle.New(options.parentCtx),
	}

	v.recordDisplay = record.New()
	v.answerPanel = answers.New(v.questions, answers.Handlers{
		Changed:               v.onAnswerSelected,
		GridSaved:             v.onGridSaved,
		ActiveQuestionChanged: v.onActiveQuestionChanged,
	})
	v.toolbar = toolbar.New(toolbar.Handlers{
		Previous:                v.Previous,
		Skip:                    v.Skip,
		Export:                  v.Export,
		UnclassifiedOnlyChanged: v.SetUnclassifiedOnly,
	})

	// Status label for feedback
	v.statusLabel = widget.NewLabel("")
	v.statusLabel.Alignment = fyne.TextAlignCenter

	v.container = v.buildLayout()

	return v
}
