package wizard

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	fyneTheme "fyne.io/fyne/v2/theme"
)

func (v *View) updateUI() {
	v.titleLabel.SetText(v.steps[v.currentStep].Title())
	v.stepLabel.SetText(fmt.Sprintf(lang.L("Step %d of %d"), v.currentStep+1, len(v.steps)))
	v.progressBar.SetValue(float64(v.currentStep + 1))

	v.stepContent.Objects = []fyne.CanvasObject{v.steps[v.currentStep].Container()}
	v.stepContent.Refresh()

	if v.currentStep == 0 {
		v.prevBtn.Disable()
	} else {
		v.prevBtn.Enable()
	}

	if v.currentStep == len(v.steps)-1 {
		v.nextBtn.SetText(lang.L("Finish"))
		v.nextBtn.SetIcon(fyneTheme.ConfirmIcon())
	} else {
		v.nextBtn.SetText(lang.L("Next"))
		v.nextBtn.SetIcon(nil)
	}
}

func (v *View) previous() {
	if v.currentStep > 0 {
		v.currentStep--
		v.updateUI()
	}
}

// cancelRequested is the Cancel button handler. If the user has typed
// or chosen anything yet, ask the host UI to confirm via Handlers.
// Confirm; otherwise discard immediately. If no Confirm callback is
// wired, fall through to Cancel; tests that do not exercise the
// confirm path do not need to inject a noop.
func (v *View) cancelRequested() {
	if v.handlers.Cancel == nil {
		return
	}

	v.ConfirmClose(func(proceed bool) {
		if proceed {
			v.handlers.Cancel()
		}
	})
}

// ConfirmClose implements [views.CloseGuard]: any teardown that would
// discard typed input (the Cancel button, Cmd/Ctrl+W, the window
// close button) funnels through the same confirmation. done is
// called exactly once; immediately with true when the wizard is
// pristine or no Confirm handler is wired.
func (v *View) ConfirmClose(done func(proceed bool)) {
	if !v.hasInput() || v.handlers.Confirm == nil {
		done(true)
		return
	}

	v.handlers.Confirm(
		lang.L("Discard wizard progress?"),
		lang.L("You will lose anything you have typed or chosen in this wizard."),
		done,
	)
}

func (v *View) hasInput() bool {
	for _, step := range v.steps {
		if step.HasInput() {
			return true
		}
	}
	return false
}

func (v *View) next() {
	if !v.steps[v.currentStep].Validate() {
		return
	}

	if v.currentStep < len(v.steps)-1 {
		v.currentStep++
		v.updateUI()
		return
	}

	v.finish()
}

func (v *View) finish() {
	if v.handlers.Complete != nil {
		v.handlers.Complete(v.result())
	}
}

// result assembles the final Result from the typed steps. It cannot
// fail: every step always has data, and Validate gated the path here.
func (v *View) result() Result {
	projectData := v.projectStep.Data()
	datasetData := v.datasetStep.Data()
	questionsData := v.questionsStep.Data()

	return Result{
		ProjectName: projectData.Name,
		DBPath:      projectData.DBPath,
		DatasetPath: datasetData.Path,
		DatasetType: datasetData.Type,
		ImageColumn: datasetData.ImageColumn,
		Questions:   questionsData.Questions,
	}
}
