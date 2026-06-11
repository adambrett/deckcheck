package answers

import (
	"fmt"
	"maps"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
)

// longQuestionThreshold is the rune count above which the question
// renders at the smaller (sub-heading) text size instead of the headline
// size. It is a rough heuristic only; the underlying widget word-wraps
// natively to the panel's actual pixel width regardless of this value.
const longQuestionThreshold = 55

// Panel displays answer buttons for all questions.
type Panel struct {
	container *fyne.Container

	questionBox *fyne.Container
	optionsBox  *fyne.Container

	questions  []project.Question
	selections map[int]int // questionID -> answerID

	handlers Handlers
}

// Handlers bundles the answer panel's outward callbacks. Changed may
// be nil; the panel guards before invoking.
type Handlers struct {
	Changed func(questionID, answerID int, selected bool)
}

// New creates a new answer panel.
func New(questions []project.Question, handlers Handlers) *Panel {
	p := &Panel{
		questions:  questions,
		selections: make(map[int]int),
		handlers:   handlers,
	}

	p.questionBox = container.NewVBox()
	p.optionsBox = container.NewVBox()

	// The options scroll: a question with many answers must not push
	// the window's minimum height past the screen.
	p.container = container.NewPadded(
		container.NewBorder(
			p.questionBox,
			nil,
			nil,
			nil,
			container.NewPadded(container.NewVScroll(p.optionsBox)),
		),
	)

	p.refreshActiveQuestion()

	return p
}

// Container returns the panel's container.
func (p *Panel) Container() fyne.CanvasObject {
	return p.container
}

// SetSelections sets the current selections.
func (p *Panel) SetSelections(selections map[int]int) {
	p.selections = maps.Clone(selections)
	if p.selections == nil {
		p.selections = make(map[int]int)
	}

	p.refreshActiveQuestion()
}

// AllAnswered returns true if all questions have been answered.
func (p *Panel) AllAnswered() bool {
	for _, q := range p.questions {
		if _, ok := p.selections[q.ID]; !ok {
			return false
		}
	}

	return true
}

// SelectAnswerByIndex selects the answer at the given index for the
// question currently on screen, the same question refreshActive-
// Question renders, so a keypress can never change an answer the user
// cannot see.
func (p *Panel) SelectAnswerByIndex(index int) {
	if len(p.questions) == 0 {
		return
	}

	q := p.questions[p.activeQuestionIndex()]
	if index >= 0 && index < len(q.Answers) {
		p.selectAnswer(q.ID, q.Answers[index].ID)
	}
}

func (p *Panel) refreshActiveQuestion() {
	p.questionBox.Objects = nil
	p.optionsBox.Objects = nil

	if len(p.questions) == 0 {
		p.questionBox.Add(newKicker(lang.L("No questions")))
		p.questionBox.Refresh()
		p.optionsBox.Refresh()
		return
	}

	index := p.activeQuestionIndex()
	question := p.questions[index]
	p.questionBox.Add(newKicker(fmt.Sprintf(lang.L("Question %d of %d"), index+1, len(p.questions))))
	p.questionBox.Add(newQuestionText(question.Text))

	selectedAnswerID, hasSelection := p.selections[question.ID]
	for i, answer := range question.Answers {
		// Number-key shortcuts stop at 9; later answers render without
		// a pill rather than advertising keys that do not exist.
		shortcut := ""
		if i < 9 {
			shortcut = strconv.Itoa(i + 1)
		}
		answerID := answer.ID
		option := newAnswerOption(shortcut, answer.Text, hasSelection && selectedAnswerID == answerID, func() {
			p.selectAnswer(question.ID, answerID)
		})
		p.optionsBox.Add(option)
	}

	p.questionBox.Refresh()
	p.optionsBox.Refresh()
}

func (p *Panel) selectAnswer(questionID, answerID int) {
	selected := true
	if currentAnswerID, ok := p.selections[questionID]; ok && currentAnswerID == answerID {
		delete(p.selections, questionID)
		selected = false
	} else {
		p.selections[questionID] = answerID
	}

	p.refreshActiveQuestion()

	if p.handlers.Changed != nil {
		p.handlers.Changed(questionID, answerID, selected)
	}
}

func (p *Panel) activeQuestionIndex() int {
	for i, q := range p.questions {
		if _, answered := p.selections[q.ID]; !answered {
			return i
		}
	}
	return len(p.questions) - 1
}

func newKicker(text string) *canvas.Text {
	kicker := canvas.NewText(text, theme.Yellow400)
	kicker.TextSize = 14
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	return kicker
}

// newQuestionText returns a widget.RichText that renders the question
// at heading size for short prompts and sub-heading size for long ones,
// and word-wraps to its allocated width via Fyne's native wrapping -
// not a hand-rolled rune-count wrap that breaks on non-Latin glyphs or
// non-default font metrics.
func newQuestionText(text string) *widget.RichText {
	sizeName := fyneTheme.SizeNameHeadingText
	if len([]rune(text)) > longQuestionThreshold {
		sizeName = fyneTheme.SizeNameSubHeadingText
	}

	rt := widget.NewRichText(&widget.TextSegment{
		Text: text,
		Style: widget.RichTextStyle{
			SizeName:  sizeName,
			ColorName: fyneTheme.ColorNameForeground,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}
