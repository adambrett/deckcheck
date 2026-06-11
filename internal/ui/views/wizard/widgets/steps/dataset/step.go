package dataset

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	deckdataset "github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/form"
)

// typeOption ties a radio-button label to the dataset.Type it represents.
type typeOption struct {
	Label string
	Type  deckdataset.Type
}

// typeOptions is the single source of truth that maps user-visible labels
// to dataset.Type values. All conversions in this file pivot through it.
// It is a function (not a package var) so the labels localise after the
// translation catalog has loaded, not at package init.
func typeOptions() []typeOption {
	return []typeOption{
		{lang.L("CSV File"), deckdataset.TypeCSV},
		{lang.L("Image Folder"), deckdataset.TypeImages},
		{lang.L("CSV with Image References"), deckdataset.TypeCSVWithImage},
	}
}

func typeLabels() []string {
	options := typeOptions()
	labels := make([]string, len(options))
	for i, opt := range options {
		labels[i] = opt.Label
	}
	return labels
}

func labelForType(t deckdataset.Type) string {
	for _, opt := range typeOptions() {
		if opt.Type == t {
			return opt.Label
		}
	}
	return typeOptions()[0].Label
}

func typeForLabel(label string) deckdataset.Type {
	for _, opt := range typeOptions() {
		if opt.Label == label {
			return opt.Type
		}
	}
	return typeOptions()[0].Type
}

// Data holds the data from the dataset step.
type Data struct {
	Path        string
	Type        deckdataset.Type
	ImageColumn string
}

// Handlers bundles the step's outward callbacks. Browse may be nil;
// the step guards before invoking.
type Handlers struct {
	Browse func()
}

// Step is the wizard step for dataset selection.
type Step struct {
	container *fyne.Container

	handlers Handlers

	typeRadio      *widget.RadioGroup
	pathEntry      *widget.Entry
	pathBtn        *widget.Button
	imageColLabel  *widget.Label
	imageColSelect *widget.Select
	imageColRow    fyne.CanvasObject
	pathField      *form.Field
	imageColField  *form.Field

	csvHeaders []string
}

// New creates a new dataset selection step. Handlers.Browse is invoked
// when the user asks to choose the dataset file or folder; the owner
// shows its picker and feeds the result back through [Step.SetPath].
func New(handlers Handlers) *Step {
	s := &Step{handlers: handlers}

	pathLabel := widget.NewLabel(lang.L("Dataset location"))
	s.pathEntry = widget.NewEntry()
	s.pathEntry.SetPlaceHolder(lang.L("Select file or folder…"))
	s.pathEntry.OnChanged = func(text string) {
		s.clearCSVHeaders()
		if strings.TrimSpace(text) != "" {
			s.pathField.SetError("")
		}
	}

	s.pathBtn = widget.NewButton(browseLabel(deckdataset.TypeCSV), s.browse)

	pathRow := container.NewBorder(nil, nil, nil, s.pathBtn, s.pathEntry)

	// Image column selection (only visible for CSV with images).
	// Must be created before the radio group to avoid nil pointer in onTypeChanged.
	s.imageColLabel = widget.NewLabel(lang.L("Image path column"))
	s.imageColSelect = widget.NewSelect([]string{}, func(selected string) {
		if selected != "" {
			s.imageColField.SetError("")
		}
	})
	s.imageColSelect.PlaceHolder = lang.L("Select column containing image paths")

	s.pathField = form.NewField(
		pathLabel,
		pathRow,
		form.WithHelpText(lang.L("Choose a CSV file or a folder of images, depending on the dataset type above.")),
	)
	s.imageColField = form.NewField(s.imageColLabel, s.imageColSelect)
	s.imageColRow = s.imageColField.Container()
	s.imageColRow.Hide()

	typeLabel := widget.NewLabel(lang.L("Dataset type"))
	s.typeRadio = widget.NewRadioGroup(typeLabels(), s.onTypeChanged)
	s.typeRadio.SetSelected(labelForType(deckdataset.TypeCSV))

	fields := form.Groups(
		form.Group(typeLabel, s.typeRadio),
		s.pathField.Container(),
		s.imageColRow,
	)

	s.container = container.NewPadded(fields)
	return s
}

// Title returns the step title.
func (s *Step) Title() string {
	return lang.L("Select dataset")
}

// Container returns the step's UI content.
func (s *Step) Container() fyne.CanvasObject {
	return s.container
}

// Validate validates the step data.
func (s *Step) Validate() bool {
	s.clearValidation()

	valid := true
	if strings.TrimSpace(s.pathEntry.Text) == "" {
		valid = false
		s.pathField.SetError(lang.L("Choose a dataset file or folder."))
	}

	if s.SelectedType() == deckdataset.TypeCSVWithImage && s.imageColSelect.Selected == "" {
		valid = false
		s.imageColField.SetError(lang.L("Select the column containing image paths."))
	}

	return valid
}

// Data returns the collected data.
func (s *Step) Data() Data {
	imageColumn := ""
	if s.SelectedType() == deckdataset.TypeCSVWithImage {
		imageColumn = s.imageColSelect.Selected
	}

	return Data{
		Path:        s.pathEntry.Text,
		Type:        s.SelectedType(),
		ImageColumn: imageColumn,
	}
}

// HasInput reports whether the user has changed any field from its
// initial state. The default radio selection (TypeCSV) is treated as
// no input; switching to a different option counts as input.
func (s *Step) HasInput() bool {
	if strings.TrimSpace(s.pathEntry.Text) != "" {
		return true
	}
	if s.imageColSelect != nil && s.imageColSelect.Selected != "" {
		return true
	}
	return s.SelectedType() != deckdataset.TypeCSV
}

// Reset clears the step data.
func (s *Step) Reset() {
	s.typeRadio.SetSelected(labelForType(deckdataset.TypeCSV))
	s.pathEntry.SetText("")
	s.imageColSelect.ClearSelected()
	s.csvHeaders = nil
	s.clearValidation()
	s.imageColRow.Hide()
}

// SetPath sets the dataset path.
func (s *Step) SetPath(path string) {
	s.pathEntry.SetText(path)
	if strings.TrimSpace(path) != "" {
		s.pathField.SetError("")
	}
}

// SetCSVHeaders sets the available CSV headers for image column selection.
func (s *Step) SetCSVHeaders(headers []string) {
	s.csvHeaders = headers
	s.imageColSelect.Options = headers
	if !containsHeader(headers, s.imageColSelect.Selected) {
		s.imageColSelect.ClearSelected()
	}
	s.imageColSelect.Refresh()
}

// SelectedType returns the currently selected dataset type.
func (s *Step) SelectedType() deckdataset.Type {
	return typeForLabel(s.typeRadio.Selected)
}

func (s *Step) onTypeChanged(selected string) {
	if s.imageColRow == nil {
		return
	}

	if typeForLabel(selected) == deckdataset.TypeCSVWithImage {
		s.imageColRow.Show()
	} else {
		s.imageColSelect.ClearSelected()
		s.imageColField.SetError("")
		s.imageColRow.Hide()
	}

	if s.pathBtn != nil {
		s.pathBtn.SetText(browseLabel(typeForLabel(selected)))
	}

	if s.container != nil {
		s.container.Refresh()
	}
}

// browseLabel returns the Browse-button label for the given dataset
// type. CSV-based sources expect a file; image folders expect a
// directory. The two-different-labels approach keeps the user from
// guessing whether the picker will land on a file or folder dialog.
func browseLabel(t deckdataset.Type) string {
	if t == deckdataset.TypeImages {
		return lang.L("Choose folder…")
	}
	return lang.L("Choose file…")
}

func (s *Step) browse() {
	if s.handlers.Browse != nil {
		s.handlers.Browse()
	}
}

func (s *Step) clearValidation() {
	s.pathField.SetError("")
	s.imageColField.SetError("")
}

func (s *Step) clearCSVHeaders() {
	s.csvHeaders = nil
	s.imageColSelect.Options = nil
	s.imageColSelect.ClearSelected()
	s.imageColSelect.Refresh()
}

func containsHeader(headers []string, selected string) bool {
	return selected == "" || slices.Contains(headers, selected)
}
