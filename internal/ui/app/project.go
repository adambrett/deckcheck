package app

import (
	"context"

	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/projectfile"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard"
	"github.com/adambrett/deckcheck/internal/usererror"
)

// CreateProject is the wizard's Complete handler. Source validation
// runs synchronously so input errors surface immediately; the project
// creation and dataset import run off the Fyne goroutine behind a
// progress dialog, and the resulting view swap is marshalled back.
func (a *App) CreateProject(result wizard.Result) {
	source, err := dataset.NewSource(result.DatasetType, result.DatasetPath, result.ImageColumn)
	if err != nil {
		a.ShowError(usererror.Wrap("DC05", usererror.ErrCreateProject, err))
		return
	}

	a.runInBackground(lang.L("Creating project…"), func(ctx context.Context) func() {
		p, err := projectfile.Create(ctx, result.DBPath, projectfile.CreateOptions{
			Name:        result.ProjectName,
			DatasetType: result.DatasetType,
			ImageColumn: result.ImageColumn,
			Questions:   result.Questions,
			Source:      source,
		})

		return func() {
			if err != nil {
				a.ShowError(usererror.Wrap("DC06", usererror.ErrCreateProject, err))
				return
			}
			a.installProject(result.DBPath, p)
		}
	})
}

// OpenProject is the welcome view's Open handler. The file load and
// any schema migrations run off the Fyne goroutine behind a progress
// dialog; the resulting view swap is marshalled back.
func (a *App) OpenProject(path string) {
	a.runInBackground(lang.L("Opening project…"), func(ctx context.Context) func() {
		p, err := projectfile.Open(ctx, path)

		return func() {
			if err != nil {
				a.ShowError(usererror.Wrap("DC07", usererror.ErrOpenProject, err))
				return
			}
			a.installProject(path, p)
		}
	})
}

// installProject swaps the freshly opened project into the classifier
// and records it as a recent. On activation failure the project is
// closed again so the file lock is not leaked.
func (a *App) installProject(path string, p *projectfile.Project) {
	if err := a.ActivateClassifierView(p); err != nil {
		_ = p.Close()
		a.ShowError(err)
		return
	}

	a.rememberProject(path)
}

func (a *App) closeProject() {
	if a.project != nil {
		_ = a.project.Close()
	}
	a.project = nil
}

func (a *App) rememberProject(path string) {
	a.recents.Add(recent.Item{Path: path})

	// Recents drive the File > Open Recent submenu, so rebuild after
	// the store mutates. installMainMenu is intentionally not called
	// here because that would re-register the canvas shortcuts.
	a.refreshMainMenu()
}
