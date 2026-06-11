package questions

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/form"
)

// Data holds the data from the questions step.
type Data struct {
	Questions []project.QuestionDef
}

// questionEntry represents a single question input row.
type questionEntry struct {
	container *fyne.Container

	label        *widget.Label
	textEntry    *widget.Entry
	answersEntry *widget.Entry
	removeBtn    *widget.Button

	textField    *form.Field
	answersField *form.Field
}

// Step is the wizard step for defining classification questions.
type Step struct {
	container    *fyne.Container
	questionsBox *fyne.Container
	addBtn       *widget.Button
	scrollable   *container.Scroll

	questions []*questionEntry
}

// New creates a new questions definition step.
func New() *Step {
	s := &Step{
		questions: make([]*questionEntry, 0),
	}

	s.questionsBox = container.NewVBox()
	s.addQuestion()
	s.addBtn = widget.NewButtonWithIcon(lang.L("Add Question"), theme.ContentAddIcon(), func() {
		s.addQuestion()
	})

	s.scrollable = container.NewVScroll(s.questionsBox)
	s.scrollable.SetMinSize(fyne.NewSize(0, 300))

	form := container.NewVBox(
		s.scrollable,
		s.addBtn,
	)

	s.container = container.NewPadded(form)
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
	for _, q := range s.questions {
		if strings.TrimSpace(q.textEntry.Text) == "" {
			valid = false
			q.textField.SetError(lang.L("Enter the question text."))
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
	}

	return valid
}

// Data returns the collected data.
func (s *Step) Data() Data {
	questions := make([]project.QuestionDef, len(s.questions))

	for i, q := range s.questions {
		questions[i] = project.QuestionDef{
			Text:    strings.TrimSpace(q.textEntry.Text),
			Answers: project.ParseAnswers(q.answersEntry.Text),
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

	s.addQuestion()
	s.clearValidation()
}

func (s *Step) addQuestion() {
	idx := len(s.questions)

	q := &questionEntry{}

	q.label = widget.NewLabel(questionLabel(idx))
	q.label.TextStyle = fyne.TextStyle{Bold: true}

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

	q.removeBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		s.removeQuestion(q)
	})

	if len(s.questions) == 0 {
		q.removeBtn.Disable()
	}

	labelRow := container.NewBorder(nil, nil, nil, q.removeBtn, q.label)
	q.textField = form.NewField(labelRow, q.textEntry)
	q.answersField = form.NewField(
		answersLabel,
		q.answersEntry,
		form.WithHelpText(lang.L("Separate answers with commas. Answers cannot themselves contain commas.")),
	)

	q.container = container.NewVBox(
		q.textField.Container(),
		q.answersField.Container(),
		widget.NewSeparator(),
	)

	s.questions = append(s.questions, q)
	s.questionsBox.Add(q.container)
	s.updateRemoveButtons()
}

func (s *Step) removeQuestion(q *questionEntry) {
	for i, question := range s.questions {
		if question == q {
			s.questions = append(s.questions[:i], s.questions[i+1:]...)
			s.questionsBox.Remove(q.container)
			break
		}
	}
	s.updateRemoveButtons()
	s.updateQuestionLabels()
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
		q.label.SetText(questionLabel(i))
	}
}

// questionLabel renders the numbered heading for the question at
// index; the single localised source for both initial construction
// and post-removal renumbering.
func questionLabel(index int) string {
	return fmt.Sprintf(lang.L("Question %d"), index+1)
}

func (s *Step) clearValidation() {
	for _, q := range s.questions {
		q.textField.SetError("")
		q.answersField.SetError("")
	}
}
