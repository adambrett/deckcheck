package questions

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	fyneLayout "fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	deckLayout "github.com/adambrett/deckcheck/internal/fyneui/layout"
	deckTheme "github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/form"
)

const (
	questionTypeWidth = 200
	gridSelectWidth   = 84
)

// Data holds the data from the questions step.
type Data struct {
	Questions []project.QuestionDef
}

// questionEntry represents a single question input row.
type questionEntry struct {
	container *fyne.Container

	label        *widget.Label
	kind         project.QuestionKind
	kindLabel    fyne.CanvasObject
	kindSelect   *widget.Select
	kindControls *fyne.Container
	textEntry    *widget.Entry
	answersEntry *widget.Entry
	rowsSelect   *widget.Select
	colsSelect   *widget.Select
	removeBtn    *widget.Button

	textField    *form.Field
	answersField *form.Field
	gridField    *form.Field
}

// Step is the wizard step for defining classification questions.
type Step struct {
	container    *fyne.Container
	questionsBox *fyne.Container
	addBtn       *widget.Button

	questions             []*questionEntry
	currentIndex          int
	allowImageAnnotations bool
}

// New creates a new questions definition step.
func New() *Step {
	s := &Step{
		questions:             make([]*questionEntry, 0),
		allowImageAnnotations: true,
	}

	s.questionsBox = container.NewStack()
	s.addQuestion()
	s.addBtn = widget.NewButtonWithIcon(lang.L("Add Question"), fyneTheme.ContentAddIcon(), func() {
		s.addQuestion()
	})

	addRow := container.NewHBox(
		fyneLayout.NewSpacer(),
		s.addBtn,
	)

	s.container = container.NewPadded(container.NewBorder(
		nil,
		addRow,
		nil,
		nil,
		s.questionsBox,
	))
	return s
}

// Title returns the step title.
func (s *Step) Title() string {
	return lang.L("Define questions")
}

// Container returns the step's UI content.
func (s *Step) Container() fyne.CanvasObject {
	return s.container
}

// Validate validates the step data.
func (s *Step) Validate() bool {
	s.clearValidation()

	if len(s.questions) == 0 {
		return false
	}

	valid := true
	firstInvalid := -1
	for i, q := range s.questions {
		if !s.validateQuestion(q) {
			valid = false
			if firstInvalid == -1 {
				firstInvalid = i
			}
		}
	}

	if !valid && firstInvalid >= 0 {
		s.currentIndex = firstInvalid
		s.showCurrentQuestion()
	}

	return valid
}

// Data returns the collected data.
func (s *Step) Data() Data {
	questions := make([]project.QuestionDef, len(s.questions))

	for i, q := range s.questions {
		rows, cols := q.gridSize()
		answers := project.ParseAnswers(q.answersEntry.Text)
		if q.kind == project.QuestionKindChoice {
			rows = 0
			cols = 0
		} else {
			answers = nil
		}
		questions[i] = project.QuestionDef{
			Kind:        q.kind,
			Text:        strings.TrimSpace(q.textEntry.Text),
			GridRows:    rows,
			GridColumns: cols,
			Answers:     answers,
		}
	}

	return Data{
		Questions: questions,
	}
}

// HasInput reports whether the user has typed any question text or
// added an extra question row beyond the initial empty entry.
func (s *Step) HasInput() bool {
	if len(s.questions) > 1 {
		return true
	}
	for _, q := range s.questions {
		if q.kind != project.QuestionKindChoice {
			return true
		}
		if strings.TrimSpace(q.textEntry.Text) != "" {
			return true
		}
		if strings.TrimSpace(q.answersEntry.Text) != "" {
			return true
		}
	}
	return false
}

// Reset clears the step data.
func (s *Step) Reset() {
	s.questions = s.questions[:0]
	s.questionsBox.Objects = s.questionsBox.Objects[:0]
	s.currentIndex = 0

	s.addQuestion()
	s.clearValidation()
}

// Back moves to the previous question on this step. It returns false
// when the wizard should leave the questions step instead.
func (s *Step) Back() bool {
	if s.currentIndex <= 0 {
		return false
	}

	s.currentIndex--
	s.showCurrentQuestion()
	return true
}

// Advance moves to the next question on this step. handled reports
// whether the step consumed the navigation, and valid reports whether
// the wizard may continue.
func (s *Step) Advance() (handled, valid bool) {
	if !s.HasNext() {
		return false, true
	}
	if !s.validateCurrentQuestion() {
		return true, false
	}

	s.currentIndex++
	s.showCurrentQuestion()
	return true, true
}

// HasNext reports whether there is another question after the one
// currently being edited.
func (s *Step) HasNext() bool {
	return s.currentIndex < len(s.questions)-1
}

// SetImageAnnotationsEnabled controls whether image-annotation
// questions are available for the dataset currently selected in the
// wizard. Plain CSV projects only support multiple-choice questions.
func (s *Step) SetImageAnnotationsEnabled(enabled bool) {
	s.allowImageAnnotations = enabled
	for _, q := range s.questions {
		s.configureQuestionKinds(q)
	}
}

func (s *Step) addQuestion() {
	q := &questionEntry{}
	q.kind = project.QuestionKindChoice

	q.label = widget.NewLabel("")
	q.label.TextStyle = fyne.TextStyle{Bold: true}

	q.kindSelect = widget.NewSelect(questionKindOptions(s.allowImageAnnotations), func(selected string) {
		q.kind = questionKindForLabel(selected)
		if q.answersField != nil {
			q.refreshKindFields()
			s.showCurrentQuestion()
			s.container.Refresh()
		}
	})
	q.kindSelect.SetSelected(questionKindLabel(project.QuestionKindChoice))

	q.textEntry = widget.NewEntry()
	q.textEntry.SetPlaceHolder(lang.L("e.g., What is the sentiment?"))
	q.textEntry.OnChanged = func(text string) {
		if strings.TrimSpace(text) != "" {
			q.textField.SetError("")
		}
	}

	answersLabel := widget.NewLabel(lang.L("Possible answers, comma-separated"))
	q.answersEntry = widget.NewEntry()
	q.answersEntry.SetPlaceHolder(lang.L("e.g., Positive, Negative, Neutral"))
	q.answersEntry.OnChanged = func(text string) {
		answers := project.ParseAnswers(text)
		if _, duplicate := project.FindDuplicateAnswer(answers); len(answers) >= 2 && !duplicate {
			q.answersField.SetError("")
		}
	}

	q.rowsSelect = widget.NewSelect(gridSizeOptions(), func(string) {
		if q.gridField != nil {
			q.gridField.SetError("")
		}
	})
	q.rowsSelect.SetSelected(strconv.Itoa(project.DefaultGridRows))
	q.colsSelect = widget.NewSelect(gridSizeOptions(), func(string) {
		if q.gridField != nil {
			q.gridField.SetError("")
		}
	})
	q.colsSelect.SetSelected(strconv.Itoa(project.DefaultGridColumns))

	q.removeBtn = widget.NewButtonWithIcon("", fyneTheme.DeleteIcon(), func() {
		s.removeQuestion(q)
	})

	if len(s.questions) == 0 {
		q.removeBtn.Disable()
	}

	q.textField = form.NewField(widget.NewLabel(lang.L("Prompt")), q.textEntry)
	q.answersField = form.NewField(
		answersLabel,
		q.answersEntry,
	)
	q.gridField = form.NewField(
		widget.NewLabel(lang.L("Grid size")),
		container.New(fyneLayout.NewCustomPaddedHBoxLayout(12),
			inlineControl(widget.NewLabel(lang.L("Rows")), q.rowsSelect, gridSelectWidth),
			inlineControl(widget.NewLabel(lang.L("Columns")), q.colsSelect, gridSelectWidth),
		),
	)

	q.kindLabel = inlineLabel(lang.L("Question type"))
	q.kindControls = middleRowControl(q.kindLabel, q.kindSelect, q.removeBtn, questionTypeWidth)
	header := container.NewBorder(nil, nil, q.label, q.kindControls)
	content := container.New(fyneLayout.NewCustomPaddedVBoxLayout(10),
		header,
		q.textField.Container(),
		q.answersField.Container(),
		q.gridField.Container(),
	)
	q.container = questionCard(content)
	q.refreshKindFields()
	s.configureQuestionKinds(q)

	s.questions = append(s.questions, q)
	s.currentIndex = len(s.questions) - 1
	s.updateRemoveButtons()
	s.updateQuestionLabels()
	s.showCurrentQuestion()
}

func (s *Step) removeQuestion(q *questionEntry) {
	for i, question := range s.questions {
		if question == q {
			s.questions = append(s.questions[:i], s.questions[i+1:]...)
			if s.currentIndex >= len(s.questions) {
				s.currentIndex = len(s.questions) - 1
			}
			break
		}
	}
	s.updateRemoveButtons()
	s.updateQuestionLabels()
	s.showCurrentQuestion()
}

func (s *Step) updateRemoveButtons() {
	canRemove := len(s.questions) > 1
	for _, q := range s.questions {
		if canRemove {
			q.removeBtn.Enable()
		} else {
			q.removeBtn.Disable()
		}
	}
}

func (s *Step) updateQuestionLabels() {
	for i, q := range s.questions {
		q.label.SetText(questionLabel(i, len(s.questions)))
	}
}

func (s *Step) configureQuestionKinds(q *questionEntry) {
	q.kindSelect.Options = questionKindOptions(s.allowImageAnnotations)
	if !s.allowImageAnnotations && q.kind == project.QuestionKindImageGrid {
		q.kindSelect.SetSelected(questionKindLabel(project.QuestionKindChoice))
	} else {
		q.kindSelect.Refresh()
	}

	if s.allowImageAnnotations {
		q.kindControls.Show()
	} else {
		q.kindControls.Hide()
	}

	if q.container != nil {
		q.container.Refresh()
	}
}

// questionLabel renders the numbered heading for the question at
// index; the single localised source for both initial construction
// and post-removal renumbering.
func questionLabel(index, total int) string {
	if total == 1 {
		return fmt.Sprintf(lang.L("Question %d"), index+1)
	}

	return fmt.Sprintf(lang.L("Question %d of %d"), index+1, total)
}

func (s *Step) clearValidation() {
	for _, q := range s.questions {
		q.clearValidation()
	}
}

func (s *Step) validateCurrentQuestion() bool {
	if len(s.questions) == 0 || s.currentIndex < 0 || s.currentIndex >= len(s.questions) {
		return false
	}

	q := s.questions[s.currentIndex]
	q.clearValidation()
	return s.validateQuestion(q)
}

func (s *Step) validateQuestion(q *questionEntry) bool {
	valid := true
	if strings.TrimSpace(q.textEntry.Text) == "" {
		valid = false
		q.textField.SetError(lang.L("Enter the question text."))
	}

	if q.kind == project.QuestionKindImageGrid {
		rows, cols := q.gridSize()
		if !project.ValidGridSize(rows, cols) {
			valid = false
			q.gridField.SetError(fmt.Sprintf(lang.L("Choose a grid size from %d to %d."), project.MinGridSize, project.MaxGridSize))
		}
		return valid
	}

	answers := project.ParseAnswers(q.answersEntry.Text)
	switch _, duplicate := project.FindDuplicateAnswer(answers); {
	case len(answers) < 2:
		valid = false
		q.answersField.SetError(lang.L("Add at least 2 answers separated by commas."))
	case duplicate:
		valid = false
		q.answersField.SetError(lang.L("Each answer must be unique."))
	}

	return valid
}

func (s *Step) showCurrentQuestion() {
	if len(s.questions) == 0 || s.currentIndex < 0 || s.currentIndex >= len(s.questions) {
		s.questionsBox.Objects = nil
		s.questionsBox.Refresh()
		return
	}

	s.questionsBox.Objects = []fyne.CanvasObject{s.questions[s.currentIndex].container}
	s.questionsBox.Refresh()
}

func (q *questionEntry) clearValidation() {
	q.textField.SetError("")
	q.answersField.SetError("")
	q.gridField.SetError("")
}

func (q *questionEntry) refreshKindFields() {
	if q.kind == project.QuestionKindImageGrid {
		q.textEntry.SetPlaceHolder(lang.L("e.g., Click every cell containing a car"))
		q.answersField.Container().Hide()
		q.gridField.Container().Show()
		if q.container != nil {
			q.container.Refresh()
		}
		return
	}

	q.textEntry.SetPlaceHolder(lang.L("e.g., What is the sentiment?"))
	q.answersField.Container().Show()
	q.gridField.Container().Hide()
	if q.container != nil {
		q.container.Refresh()
	}
}

func questionCard(content fyne.CanvasObject) *fyne.Container {
	background := canvas.NewRectangle(deckTheme.Gray900)
	background.CornerRadius = 8

	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = 8
	border.StrokeColor = deckTheme.Gray700
	border.StrokeWidth = 1

	return container.NewStack(
		background,
		border,
		container.NewPadded(content),
	)
}

func inlineControl(label fyne.CanvasObject, control fyne.CanvasObject, width float32) *fyne.Container {
	return container.New(fyneLayout.NewCustomPaddedHBoxLayout(6),
		deckLayout.NewFixedHeight(control.MinSize().Height, container.NewCenter(label)),
		deckLayout.NewFixedWidth(width, control),
	)
}

func middleRowControl(label fyne.CanvasObject, control fyne.CanvasObject, trailing fyne.CanvasObject, width float32) *fyne.Container {
	return container.New(&middleRowLayout{gap: 8, firstYOffset: -3},
		label,
		deckLayout.NewFixedWidth(width, control),
		trailing,
	)
}

func inlineLabel(text string) *canvas.Text {
	label := canvas.NewText(text, color.White)
	label.TextSize = 15
	return label
}

type middleRowLayout struct {
	gap          float32
	firstYOffset float32
}

func (l *middleRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	height := float32(0)
	visible := 0
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		size := obj.MinSize()
		width += size.Width
		if size.Height > height {
			height = size.Height
		}
		visible++
	}
	if visible > 1 {
		width += float32(visible-1) * l.gap
	}
	return fyne.NewSize(width, height)
}

func (l *middleRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		objSize := obj.MinSize()
		y := (size.Height - objSize.Height) / 2
		if i == 0 {
			y += l.firstYOffset
		}
		obj.Move(fyne.NewPos(x, y))
		obj.Resize(objSize)
		x += objSize.Width + l.gap
	}
}

func (q *questionEntry) gridSize() (int, int) {
	rows, _ := strconv.Atoi(q.rowsSelect.Selected)
	cols, _ := strconv.Atoi(q.colsSelect.Selected)
	return rows, cols
}

func questionKindOptions(includeImageAnnotation bool) []string {
	options := []string{
		questionKindLabel(project.QuestionKindChoice),
	}
	if includeImageAnnotation {
		options = append(options, questionKindLabel(project.QuestionKindImageGrid))
	}
	return options
}

func questionKindLabel(kind project.QuestionKind) string {
	switch kind {
	case project.QuestionKindImageGrid:
		return lang.L("Image annotation")
	default:
		return lang.L("Multiple choice")
	}
}

func questionKindForLabel(label string) project.QuestionKind {
	if label == questionKindLabel(project.QuestionKindImageGrid) {
		return project.QuestionKindImageGrid
	}

	return project.QuestionKindChoice
}

func gridSizeOptions() []string {
	options := make([]string, 0, project.MaxGridSize-project.MinGridSize+1)
	for size := project.MinGridSize; size <= project.MaxGridSize; size++ {
		options = append(options, strconv.Itoa(size))
	}
	return options
}
