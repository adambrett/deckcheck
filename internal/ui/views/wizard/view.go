package wizard

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/go-fyne/pkg/browse"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views"
	"github.com/adambrett/deckcheck/internal/ui/views/lifecycle"
	datasetstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/dataset"
	projectstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/project"
	questionsstep "github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/steps/questions"
)

// Compile-time checks: wizard.View satisfies both views.View and
// views.Closer so its CSV header probe is cancelled when the view is
// replaced.
var (
	_ views.View       = (*View)(nil)
	_ views.Closer     = (*View)(nil)
	_ views.CloseGuard = (*View)(nil)
)

const (
	viewWidth  = 720
	viewHeight = 580
)

// Result contains the data collected from all wizard steps.
type Result struct {
	ProjectName string
	DBPath      string
	DatasetPath string
	DatasetType dataset.Type
	ImageColumn string
	Questions   []project.QuestionDef
}

// Step represents a single step in the wizard.
//
// Title is shown in the header. Container is the visual content
// installed when this step is active. Validate is called on Next /
// Finish; returning false keeps the wizard on this step. Reset
// returns the step to its initial state when the wizard reopens.
// HasInput reports whether the user has typed or chosen anything
// that would be discarded on Cancel. Each step's collected payload
// is read through its concrete, typed Data method, deliberately not
// part of this interface, so the wizard never type-asserts.
type Step interface {
	Title() string
	Container() fyne.CanvasObject
	Validate() bool
	Reset()
	HasInput() bool
}

// Handlers bundles the wizard's outward callbacks. Any field may be
// nil; the wizard guards before invoking. Confirm asks the host UI
// to display a yes/no prompt and call the supplied callback with the
// user's choice. When nil, the wizard skips confirmation and runs
// destructive actions directly.
type Handlers struct {
	Complete func(Result)
	Cancel   func()
	Confirm  func(title, message string, response func(confirmed bool))
	Error    func(error)
}

// Config holds the wizard's dependencies. ProjectPicker backs the
// database save dialog; DatasetPicker backs the source-data open
// dialog.
type Config struct {
	ProjectPicker browse.Picker
	DatasetPicker browse.Picker
	Handlers      Handlers
}

// View manages the multi-step wizard flow.
type View struct {
	projectPicker browse.Picker
	datasetPicker browse.Picker

	// life scopes the IO the wizard issues (currently the CSV header
	// probe for the image-column dropdown, which runs off the Fyne
	// goroutine). Activate rebuilds its context on each activation;
	// Close cancels any probe still in flight when the user backs out.
	life *lifecycle.Lifecycle

	container *fyne.Container

	stepContent *fyne.Container
	titleLabel  *widget.Label
	stepLabel   *widget.Label
	progressBar *widget.ProgressBar
	prevBtn     *widget.Button
	nextBtn     *widget.Button
	cancelBtn   *widget.Button

	projectStep   *projectstep.Step
	datasetStep   *datasetstep.Step
	questionsStep *questionsstep.Step
	steps         []Step
	currentStep   int

	handlers Handlers
}

// Option configures optional wizard behaviour.
type Option func(*options)

type options struct {
	parentCtx context.Context
}

func defaultOptions() options {
	return options{}
}

// WithParentContext supplies the long-lived context the wizard's
// per-activation context is derived from. Cancelling parent cancels
// every in-flight wizard call.
func WithParentContext(parent context.Context) Option {
	return func(options *options) {
		options.parentCtx = parent
	}
}

// New constructs a fresh wizard with three steps wired from cfg.
// Production wires every field; tests may leave callbacks nil when
// they don't drive that code path.
func New(cfg Config, opts ...Option) *View {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	v := &View{
		projectPicker: cfg.ProjectPicker,
		datasetPicker: cfg.DatasetPicker,
		handlers:      cfg.Handlers,
		life:          lifecycle.New(options.parentCtx),
	}

	// Each step reports browse taps back to the wizard, which owns the
	// pickers; results return to the step through its typed setters.
	v.projectStep = projectstep.New(projectstep.Handlers{Browse: func() {
		v.selectProjectPath(v.projectStep.SuggestedFilename(), v.projectStep.SetDBPath)
	}})
	v.datasetStep = datasetstep.New(datasetstep.Handlers{Browse: v.browseDataset})
	v.questionsStep = questionsstep.New()
	v.steps = []Step{v.projectStep, v.datasetStep, v.questionsStep}

	v.titleLabel = widget.NewLabel("")
	v.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	v.titleLabel.Alignment = fyne.TextAlignLeading

	v.stepLabel = widget.NewLabel("")
	v.stepLabel.Alignment = fyne.TextAlignTrailing

	v.progressBar = widget.NewProgressBar()
	v.progressBar.Min = 0
	v.progressBar.Max = float64(len(v.steps))
	v.progressBar.TextFormatter = func() string { return "" }

	v.prevBtn = widget.NewButton(lang.L("Previous"), v.previous)

	v.nextBtn = widget.NewButton(lang.L("Next"), v.next)
	v.nextBtn.Importance = widget.HighImportance
	v.nextBtn.IconPlacement = widget.ButtonIconTrailingText

	v.cancelBtn = widget.NewButton(lang.L("Cancel"), v.cancelRequested)

	// Step content container
	v.stepContent = container.NewStack(v.steps[0].Container())

	// Build layout
	header := container.NewVBox(
		container.NewBorder(nil, nil, v.titleLabel, v.stepLabel),
		v.progressBar,
		widget.NewSeparator(),
	)

	buttons := container.NewHBox(
		v.cancelBtn,
		layout.NewSpacer(),
		v.prevBtn,
		v.nextBtn,
	)

	form := container.NewBorder(
		header,
		container.NewPadded(buttons),
		nil,
		nil,
		container.NewPadded(v.stepContent),
	)
	v.container = container.NewStack(
		canvas.NewRectangle(theme.Gray950),
		container.NewPadded(form),
	)

	// updateUI owns all step-dependent chrome (title, step counter,
	// progress, button states); running it here keeps New and
	// navigation from carrying duplicate copies of that logic.
	v.updateUI()

	return v
}
