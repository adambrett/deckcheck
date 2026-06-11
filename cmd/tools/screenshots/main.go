// Command screenshots renders the three DeckCheck screens with the
// software renderer and writes PNGs to docs/screenshots. Run via
// `make screenshots`; no display is required.
package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"

	"github.com/adambrett/go-fyne/pkg/recent"

	"github.com/adambrett/deckcheck/internal/assets"
	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/export"
	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/projectfile"
	"github.com/adambrett/deckcheck/internal/translations"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier"
	"github.com/adambrett/deckcheck/internal/ui/views/welcome"
	"github.com/adambrett/deckcheck/internal/ui/views/wizard"
)

const outDir = "docs/screenshots"

// out is the absolute output directory, resolved up front so the
// capture helpers cannot be broken by a working-directory change.
var out string

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := translations.Register(); err != nil {
		return err
	}

	app := test.NewApp()
	app.Settings().SetTheme(theme.New())

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	var err error
	if out, err = filepath.Abs(outDir); err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}

	work, err := os.MkdirTemp("", "deckcheck-screenshots-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }()

	if err := captureWelcome(app); err != nil {
		return fmt.Errorf("welcome: %w", err)
	}
	if err := captureWizard(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}
	if err := captureClassifier(work); err != nil {
		return fmt.Errorf("classifier: %w", err)
	}

	return nil
}

// captureWelcome renders the launcher with two seeded recent projects.
func captureWelcome(app fyne.App) error {
	// Recents store absolute paths and render them as the card
	// subtitle, so a temp-dir prefix would leak into the screenshot.
	// Stage the stub projects under the real home directory instead
	// and remove them afterwards.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// os.Mkdir (not MkdirAll) so the tool refuses to run if the user
	// already has a real ~/DeckCheck Projects directory, the deferred
	// cleanup below must never touch anything the tool did not create.
	stage := filepath.Join(home, "DeckCheck Projects")
	if err := os.Mkdir(stage, 0o750); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to stage screenshots: %s already exists; move it aside and re-run", stage)
		}
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	paths := []string{
		filepath.Join(stage, "product-feedback.deckcheck"),
		filepath.Join(stage, "photo-triage.deckcheck"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("stub"), 0o600); err != nil {
			return err
		}
	}
	app.Preferences().SetStringList(recent.DefaultPreferencesKey, paths)

	view := welcome.New(welcome.Config{
		Recents:  recent.New(app.Preferences()),
		Icon:     fyne.NewStaticResource("icon.png", assets.Icon),
		Picker:   nil,
		Handlers: welcome.Handlers{},
	})
	if err := view.Activate(); err != nil {
		return err
	}

	return capture(view.Container(), view.Size(), "welcome.png")
}

// captureWizard renders the first step of the new-project wizard.
func captureWizard() error {
	view := wizard.New(wizard.Config{})
	if err := view.Activate(); err != nil {
		return err
	}
	defer view.Close()

	return capture(view.Container(), view.Size(), "wizard.png")
}

// captureClassifier builds a real project file with sample rows and
// renders the classifier mid-session.
func captureClassifier(work string) error {
	csvPath := filepath.Join(work, "feedback.csv")
	csv := "feedback,channel\n" +
		"\"The new export button saved me an hour today. Thank you!\",email\n" +
		"\"App crashes when I open a project from a network drive.\",support\n" +
		"\"Could you add dark mode? My eyes would be grateful.\",twitter\n"
	if err := os.WriteFile(csvPath, []byte(csv), 0o600); err != nil {
		return err
	}

	ctx := context.Background()
	p, err := projectfile.Create(ctx, filepath.Join(work, "feedback.deckcheck"), projectfile.CreateOptions{
		Name:        "Product feedback",
		DatasetType: dataset.TypeCSV,
		Questions: []project.QuestionDef{
			{Text: "What is the sentiment?", Answers: []string{"Positive", "Negative", "Neutral"}},
		},
		Source: dataset.NewCSV(csvPath),
	})
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()

	questions, err := p.Questions(ctx)
	if err != nil {
		return err
	}

	// Classify the first record so the screenshot shows a session in
	// progress (1/3 done, classifier landed on the second record)
	// rather than an untouched deck.
	first, err := p.Record(ctx, 0)
	if err != nil {
		return err
	}
	if err := p.SaveClassification(ctx, first.ID, questions[0].ID, questions[0].Answers[0].ID); err != nil {
		return err
	}

	view := classifier.New(classifier.Config{
		Project:   p,
		Questions: questions,
		Exporter:  export.New(p),
	})
	if err := view.Activate(); err != nil {
		return err
	}
	defer view.Close()

	return capture(view.Container(), view.Size(), "classifier.png")
}

func capture(content fyne.CanvasObject, size fyne.Size, name string) error {
	canvas := software.NewCanvas()
	canvas.SetContent(content)
	canvas.Resize(size)

	return writePNG(canvas.Capture(), filepath.Join(out, name))
}

func writePNG(img image.Image, path string) error {
	file, err := os.Create(path) //nolint:gosec // path is built from the outDir constant
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	fmt.Println("wrote", path)
	return nil
}
