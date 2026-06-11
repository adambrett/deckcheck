//go:build integration

package record_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/fynetest"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/ui/views/classifier/widgets/panels/record"
)

func TestPanelRendersRecordData(t *testing.T) {
	// Given
	test.NewApp()
	panel := record.New()

	// When
	panel.SetRecord(&project.Record{
		ID:   1,
		Data: map[string]string{"text": "Plain row", "empty": ""},
	})

	// Then the column names and values are rendered, with empty cells marked.
	fynetest.RequireLabel(t, panel.Container(), "text:")
	fynetest.RequireLabel(t, panel.Container(), "Plain row")
	fynetest.RequireLabel(t, panel.Container(), "(empty)")
}

func TestPanelRendersImageFilename(t *testing.T) {
	// Given
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)

	// When
	panel.SetRecord(&project.Record{
		ID:        1,
		Data:      map[string]string{"text": "Photo row"},
		ImagePath: imagePath,
	})

	// Then the image row shows the filename next to the CSV data.
	fynetest.RequireLabel(t, panel.Container(), "Image:")
	fynetest.RequireLabel(t, panel.Container(), filepath.Base(imagePath))
	fynetest.RequireLabel(t, panel.Container(), "Photo row")
}

func TestPanelSkipsFilenameColumnWhenImageShown(t *testing.T) {
	// Given an image-folder record whose only column repeats the filename.
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)

	// When
	panel.SetRecord(&project.Record{
		ID:        1,
		Data:      map[string]string{dataset.FilenameColumn: filepath.Base(imagePath)},
		ImagePath: imagePath,
	})

	// Then the filename renders once as the Image row, not twice.
	fynetest.RequireLabel(t, panel.Container(), "Image:")
	fynetest.RequireNoLabel(t, panel.Container(), dataset.FilenameColumn+":")
}

func TestPanelClearsAndHandlesNilRecord(t *testing.T) {
	// Given
	test.NewApp()
	panel := record.New()
	panel.SetRecord(&project.Record{ID: 1, Data: map[string]string{"text": "row"}})

	// When
	panel.SetRecord(nil)

	// Then the data rows are gone.
	fynetest.RequireNoLabel(t, panel.Container(), "row")

	// When / Then clearing an already-clear panel is harmless.
	require.NotPanics(t, panel.Clear)
}

func TestPanelToleratesUnreadableImagePaths(t *testing.T) {
	// Given
	test.NewApp()
	panel := record.New()

	textPath := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(textPath, []byte("text"), 0o600))

	// When / Then non-image and missing paths do not panic the panel.
	require.NotPanics(t, func() {
		panel.SetRecord(&project.Record{ID: 1, ImagePath: textPath})
	})
	require.NotPanics(t, func() {
		panel.SetRecord(&project.Record{ID: 2, ImagePath: filepath.Join(t.TempDir(), "missing.png")})
	})
}

func TestPanelOpensPreviewWindowSizedToImage(t *testing.T) {
	// Given a record showing a small image
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})

	// When tapping the preview image
	test.Tap(fynetest.FirstTappable(t, panel.Container()))

	// Then a preview window opens at the image's natural size.
	preview := windowWithTitle(t, filepath.Base(imagePath))
	require.Equal(t, fyne.NewSize(40, 30), preview.Canvas().Size())
}

func TestPanelCloseClosesOpenPreviewWindows(t *testing.T) {
	// Given a panel with an open preview window
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})
	test.Tap(fynetest.FirstTappable(t, panel.Container()))
	require.NotNil(t, findWindow(filepath.Base(imagePath)))

	// When the panel closes (the classifier view is being torn down)
	panel.Close()

	// Then the orphaned preview window is gone.
	require.Nil(t, findWindow(filepath.Base(imagePath)))
}

func TestPanelPreviewScalesOversizedImagesDown(t *testing.T) {
	// Given a record showing an image larger than the preview cap
	test.NewApp()
	panel := record.New()
	imagePath := writePNGSized(t, 2800, 2000)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})

	// When
	test.Tap(fynetest.FirstTappable(t, panel.Container()))

	// Then the preview is scaled down to fit within the size cap.
	preview := windowWithTitle(t, filepath.Base(imagePath))
	require.Equal(t, fyne.NewSize(1400, 1000), preview.Canvas().Size())
}

func TestPanelPreviewFallsBackToDefaultSizeForUndecodableImages(t *testing.T) {
	// Given an image-named file whose contents cannot be decoded
	test.NewApp()
	panel := record.New()
	imagePath := filepath.Join(t.TempDir(), "corrupt.png")
	require.NoError(t, os.WriteFile(imagePath, []byte("not a png"), 0o600))
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})

	// When
	test.Tap(fynetest.FirstTappable(t, panel.Container()))

	// Then the preview still opens, at the default size.
	preview := windowWithTitle(t, "corrupt.png")
	require.Equal(t, fyne.NewSize(900, 700), preview.Canvas().Size())
}

func TestPanelPreviewFallsBackToDefaultSizeWhenImageDisappears(t *testing.T) {
	// Given a record whose image file vanishes after it is displayed
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})
	require.NoError(t, os.Remove(imagePath))

	// When
	test.Tap(fynetest.FirstTappable(t, panel.Container()))

	// Then the preview still opens, at the default size.
	preview := windowWithTitle(t, filepath.Base(imagePath))
	require.Equal(t, fyne.NewSize(900, 700), preview.Canvas().Size())
}

func TestPreviewWindowClosesOnEscapeAndTap(t *testing.T) {
	// Given an open preview window
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	title := filepath.Base(imagePath)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})
	test.Tap(fynetest.FirstTappable(t, panel.Container()))
	preview := windowWithTitle(t, title)

	// When pressing a key other than Escape
	preview.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEnter})

	// Then the window stays open.
	require.NotNil(t, findWindow(title))

	// When pressing Escape
	preview.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})

	// Then the window is gone.
	require.Nil(t, findWindow(title))

	// Given a fresh preview window
	test.Tap(fynetest.FirstTappable(t, panel.Container()))
	preview = windowWithTitle(t, title)

	// When tapping the enlarged image
	test.Tap(fynetest.FirstTappable(t, preview.Canvas().Content()))

	// Then it closes too.
	require.Nil(t, findWindow(title))
}

func TestPanelPreviewIgnoresTapsWithoutImage(t *testing.T) {
	// Given a panel whose image has been cleared
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})
	panel.Clear()
	before := len(fyne.CurrentApp().Driver().AllWindows())

	// When tapping where the image used to be
	test.Tap(fynetest.FirstTappable(t, panel.Container()))

	// Then no preview window opens.
	require.Len(t, fyne.CurrentApp().Driver().AllWindows(), before)
}

func TestPreviewImageWidgetContract(t *testing.T) {
	// Given a panel showing an image
	test.NewApp()
	panel := record.New()
	imagePath := writePNG(t)
	panel.SetRecord(&project.Record{ID: 1, ImagePath: imagePath})
	img := fynetest.FirstTappable(t, panel.Container())

	// When / Then the secondary-tap and renderer hooks are safe no-ops,
	// and the minimum size matches what the panel configured.
	require.NotPanics(t, func() {
		test.TapSecondary(img.(fyne.SecondaryTappable))
	})
	require.Equal(t, fyne.NewSize(220, 220), img.(fyne.CanvasObject).MinSize())
	require.NotPanics(t, func() {
		test.WidgetRenderer(img.(fyne.Widget)).Destroy()
	})
}

func findWindow(title string) fyne.Window {
	for _, window := range fyne.CurrentApp().Driver().AllWindows() {
		if window.Title() == title {
			return window
		}
	}

	return nil
}

func windowWithTitle(t *testing.T, title string) fyne.Window {
	t.Helper()

	window := findWindow(title)
	require.NotNil(t, window, "window %q not found", title)

	return window
}

func writePNG(t *testing.T) string {
	t.Helper()

	return writePNGSized(t, 40, 30)
}

func writePNGSized(t *testing.T, width, height int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "image.png")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	require.NoError(t, png.Encode(file, img))

	return path
}
