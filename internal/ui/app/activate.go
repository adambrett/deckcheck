package app

import (
	"fyne.io/fyne/v2"

	"github.com/adambrett/deckcheck/internal/export"
	"github.com/adambrett/deckcheck/internal/ui/views"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier"
	"github.com/adambrett/deckcheck/internal/ui/views/welcome"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// ActivateWelcomeView shows the launcher screen. Welcome activation
// cannot fail today; any future error routes through the generic
// dialog rather than minting a code for an unreachable path.
func (a *App) ActivateWelcomeView() {
	view := welcome.New(welcome.Config{
		Recents: a.recents,
		Icon:    a.icon,
		Picker:  a.projectPicker,
		Handlers: welcome.Handlers{
			Create: a.ActivateWizardView,
			Open:   a.OpenProject,
		},
	})

	if err := a.activateView(view); err != nil {
		a.ShowError(err)
	}
}

// ActivateWizardView shows the new-project wizard, first letting a
// guarded active view (an earlier dirty wizard) confirm. The open
// project, if any, stays open: cancelling the wizard returns to it.
func (a *App) ActivateWizardView() {
	a.navigate(a.showWizard)
}

// showWizard installs the wizard unconditionally. Wizard activation
// cannot fail today; see ActivateWelcomeView for the error stance.
func (a *App) showWizard() {
	view := wizard.New(wizard.Config{
		ProjectPicker: a.projectPicker,
		DatasetPicker: a.picker,
		Handlers: wizard.Handlers{
			Complete: a.CreateProject,
			Cancel:   a.returnFromWizard,
			Confirm:  a.ShowConfirm,
			Error:    a.ShowError,
		},
	}, wizard.WithParentContext(a.ctx()))

	if err := a.activateView(view); err != nil {
		a.ShowError(err)
	}
}

// returnFromWizard is the wizard's Cancel destination: back to the
// still-open project when there is one, otherwise to the launcher.
func (a *App) returnFromWizard() {
	if a.project == nil {
		a.ActivateWelcomeView()
		return
	}

	if err := a.ActivateClassifierView(a.project); err != nil {
		// The open project can no longer activate (e.g. the database
		// died underneath it); fall back to a clean launcher rather
		// than stranding the user in the dead wizard.
		a.ShowError(err)
		a.closeProjectToWelcome()
	}
}

// ActivateClassifierView opens currentProject in the classifier and
// installs it as the app's open project, closing any previous one.
func (a *App) ActivateClassifierView(currentProject Project) error {
	outgoing := a.active

	questions, err := currentProject.Questions(a.ctx())
	if err != nil {
		return usererror.Wrap("DC03", usererror.ErrLoadQuestions, err)
	}

	view := classifier.New(classifier.Config{
		Picker:    a.picker,
		Project:   currentProject,
		Questions: questions,
		Exporter:  export.New(currentProject),
		Handlers: classifier.Handlers{
			Error:       a.ShowError,
			Information: a.ShowInformation,
		},
	}, classifier.WithParentContext(a.ctx()))

	if err := a.activateView(view); err != nil {
		// The classifier's Activate attaches its own code to specific
		// failure sites (e.g. DC16 for the first-record scan); only
		// code the catch-all when the error arrives uncoded so the
		// inner, more precise site stays searchable.
		if usererror.CodeOf(err) != "" {
			return err
		}
		return usererror.Wrap("DC04", usererror.ErrInitializeClassifier, err)
	}

	previousProject := a.project
	a.project = currentProject

	if previousProject != nil && previousProject != currentProject {
		release := func() { _ = previousProject.Close() }
		if quiescer, ok := outgoing.(views.Quiescer); ok {
			quiescer.Quiesce(release)
		} else {
			release()
		}
	}

	return nil
}

func (a *App) activateView(view views.View) error {
	if err := view.Activate(); err != nil {
		return err
	}

	// Give the outgoing view a chance to cancel pending work
	// (timers, goroutines) before the new view takes over.
	a.closeActiveView()

	a.active = view

	a.setContent(view.Container())
	a.resizeCentered(view.Size())

	// Menu enabled-states (Close Project, Export CSV) track the active
	// view. Deferred because activation can run from a menu item, and
	// rebuilding the main menu while AppKit is still tracking the
	// click corrupts native menu state.
	fyne.Do(a.refreshMainMenu)

	return nil
}

func (a *App) resizeCentered(size fyne.Size) {
	// Each view picks its preferred initial size; the user is free to
	// resize from there. Fyne clamps the window to the content's
	// minimum size, and the classifier's split respects each pane's
	// minimum, so reflow cannot drag the layout into a broken state.
	a.window.SetFixedSize(false)
	a.window.Resize(size)
	a.window.CenterOnScreen()
}

func (a *App) setContent(content fyne.CanvasObject) {
	a.content.Objects = []fyne.CanvasObject{content}
	a.content.Refresh()
}
