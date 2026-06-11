package project

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	deckproject "github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/form"
)

// Data holds the data from the project step.
type Data struct {
	Name   string
	DBPath string
}

// Handlers bundles the step's outward callbacks. Browse may be nil;
// the step guards before invoking.
type Handlers struct {
	Browse func()
}

// Step is the first wizard step for project configuration.
type Step struct {
	container *fyne.Container

	handlers Handlers

	nameEntry *widget.Entry
	pathEntry *widget.Entry
	pathBtn   *widget.Button

	nameField *form.Field
	pathField *form.Field
}

// New creates a new project configuration step. Handlers.Browse is
// invoked when the user asks to choose the save location; the owner
// shows its file dialog and feeds the result back through
// [Step.SetDBPath].
func New(handlers Handlers) *Step {
	s := &Step{handlers: handlers}

	nameLabel := widget.NewLabel(lang.L("Project name"))
	s.nameEntry = widget.NewEntry()
	s.nameEntry.SetPlaceHolder(lang.L("My Classification Project"))
	s.nameEntry.OnChanged = func(text string) {
		// Cap project names so the filename SuggestedFilename derives
		// from this string stays clear of filesystem path limits.
		if runes := []rune(text); len(runes) > deckproject.MaxNameLength {
			s.nameEntry.SetText(string(runes[:deckproject.MaxNameLength]))
			return
		}
		if strings.TrimSpace(text) != "" {
			s.nameField.SetError("")
		}
	}

	pathLabel := widget.NewLabel(lang.L("Save location"))
	s.pathEntry = widget.NewEntry()
	s.pathEntry.SetPlaceHolder(fmt.Sprintf(lang.L("/path/to/project%s"), deckproject.FileExtension))
	s.pathEntry.OnChanged = func(text string) {
		if strings.TrimSpace(text) != "" {
			s.pathField.SetError("")
		}
	}

	s.pathBtn = widget.NewButton(lang.L("Choose file…"), s.browse)

	pathRow := container.NewBorder(nil, nil, nil, s.pathBtn, s.pathEntry)
	s.nameField = form.NewField(
		nameLabel,
		s.nameEntry,
		form.WithHelpText(fmt.Sprintf(lang.L("Max %d characters."), deckproject.MaxNameLength)),
	)
	s.pathField = form.NewField(
		pathLabel,
		pathRow,
		form.WithHelpText(fmt.Sprintf(lang.L("Choose a %s file the project will save to."), deckproject.FileExtension)),
	)

	fields := form.Groups(
		s.nameField.Container(),
		s.pathField.Container(),
	)
	s.container = container.NewPadded(fields)

	return s
}

// Title returns the step title.
func (s *Step) Title() string {
	return lang.L("Create new project")
}

// Container returns the step's UI content.
func (s *Step) Container() fyne.CanvasObject {
	return s.container
}

// Validate validates the step data.
func (s *Step) Validate() bool {
	s.clearValidation()

	valid := true
	if strings.TrimSpace(s.nameEntry.Text) == "" {
		valid = false
		s.nameField.SetError(lang.L("Enter a project name."))
	}
	if strings.TrimSpace(s.pathEntry.Text) == "" {
		valid = false
		s.pathField.SetError(lang.L("Choose where to save the project database."))
	}

	return valid
}

// Data returns the collected data.
func (s *Step) Data() Data {
	return Data{
		Name:   s.nameEntry.Text,
		DBPath: s.pathEntry.Text,
	}
}

// HasInput reports whether the user has typed anything yet.
func (s *Step) HasInput() bool {
	return strings.TrimSpace(s.nameEntry.Text) != "" || strings.TrimSpace(s.pathEntry.Text) != ""
}

// Reset clears the step data.
func (s *Step) Reset() {
	s.nameEntry.SetText("")
	s.pathEntry.SetText("")
	s.clearValidation()
}

// SetName sets the project name as if the user had typed it.
func (s *Step) SetName(name string) {
	s.nameEntry.SetText(name)
}

// SetDBPath sets the database path.
func (s *Step) SetDBPath(path string) {
	s.pathEntry.SetText(path)
	if strings.TrimSpace(path) != "" {
		s.pathField.SetError("")
	}
}

// SuggestedFilename derives a safe on-disk filename from the current
// project name.
func (s *Step) SuggestedFilename() string {
	return deckproject.DefaultFilename(s.nameEntry.Text)
}

func (s *Step) browse() {
	if s.handlers.Browse != nil {
		s.handlers.Browse()
	}
}

func (s *Step) clearValidation() {
	s.nameField.SetError("")
	s.pathField.SetError("")
}
