package record

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type previewImage struct {
	widget.BaseWidget

	image *canvas.Image

	onTapped func()
}

func (p *previewImage) SetMinSize(size fyne.Size) {
	p.image.SetMinSize(size)
	p.Refresh()
}

func (p *previewImage) SetFile(path string) {
	p.image.File = path
	p.image.Resource = nil
	p.image.Image = nil
	p.image.Refresh()
	p.Refresh()
}

func (p *previewImage) Tapped(_ *fyne.PointEvent) {
	if p.onTapped != nil && p.image.File != "" {
		p.onTapped()
	}
}

func (p *previewImage) TappedSecondary(_ *fyne.PointEvent) {}

func (p *previewImage) CreateRenderer() fyne.WidgetRenderer {
	return &previewImageRenderer{
		image:   p.image,
		objects: []fyne.CanvasObject{p.image},
	}
}

type previewImageRenderer struct {
	image *canvas.Image

	objects []fyne.CanvasObject
}

func (r *previewImageRenderer) Layout(size fyne.Size) {
	r.image.Move(fyne.NewPos(0, 0))
	r.image.Resize(size)
}

func (r *previewImageRenderer) MinSize() fyne.Size {
	return r.image.MinSize()
}

func (r *previewImageRenderer) Refresh() {
	// Skip the canvas redraw when there is nothing to render. The
	// panel calls SetFile("") whenever the record has no image and
	// hides the imageBox immediately afterwards, so a refresh here
	// would only paint an empty canvas the user never sees.
	if r.image.File == "" && r.image.Resource == nil && r.image.Image == nil {
		return
	}
	canvas.Refresh(r.image)
}

func (r *previewImageRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *previewImageRenderer) Destroy() {}

func newPreviewImage(onTapped func()) *previewImage {
	p := &previewImage{
		image:    &canvas.Image{FillMode: canvas.ImageFillContain},
		onTapped: onTapped,
	}
	p.ExtendBaseWidget(p)
	return p
}
