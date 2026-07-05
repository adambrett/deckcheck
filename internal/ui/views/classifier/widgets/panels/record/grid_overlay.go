package record

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/project"
)

type imageGridOverlay struct {
	widget.BaseWidget

	rows, columns      int
	imageWidth         int
	imageHeight        int
	selected           map[project.GridCell]struct{}
	dragAnchor         project.GridCell
	dragActive         bool
	onSelectionChanged func(string)
}

func newImageGridOverlay() *imageGridOverlay {
	g := &imageGridOverlay{
		selected: make(map[project.GridCell]struct{}),
	}
	g.ExtendBaseWidget(g)
	return g
}

func (g *imageGridOverlay) SetImageDimensions(width, height int) {
	g.imageWidth = width
	g.imageHeight = height
	g.Refresh()
}

func (g *imageGridOverlay) SetConfig(cfg GridConfig) {
	g.rows = cfg.Rows
	g.columns = cfg.Columns
	g.onSelectionChanged = cfg.Changed
	g.selected = make(map[project.GridCell]struct{})
	g.dragActive = false

	cells, err := project.ParseGridSelection(cfg.Selection, cfg.Rows, cfg.Columns)
	if err == nil {
		for _, cell := range cells {
			g.selected[cell] = struct{}{}
		}
	}

	g.Refresh()
}

func (g *imageGridOverlay) Tapped(event *fyne.PointEvent) {
	cell, ok := g.cellAt(event.Position, false)
	if !ok {
		return
	}

	if _, ok := g.selected[cell]; ok {
		delete(g.selected, cell)
	} else {
		g.selected[cell] = struct{}{}
	}

	g.Refresh()
	g.emitSelection()
}

func (g *imageGridOverlay) TappedSecondary(_ *fyne.PointEvent) {}

func (g *imageGridOverlay) Dragged(event *fyne.DragEvent) {
	if !g.dragActive {
		anchorPos := fyne.NewPos(
			event.Position.X-event.Dragged.DX,
			event.Position.Y-event.Dragged.DY,
		)
		anchor, ok := g.cellAt(anchorPos, false)
		if !ok {
			return
		}
		g.dragAnchor = anchor
		g.dragActive = true
	}

	cell, ok := g.cellAt(event.Position, true)
	if !ok {
		return
	}
	if !g.selectRectangle(g.dragAnchor, cell) {
		return
	}

	g.Refresh()
	g.emitSelection()
}

func (g *imageGridOverlay) DragEnd() {
	g.dragActive = false
}

func (g *imageGridOverlay) CreateRenderer() fyne.WidgetRenderer {
	r := &imageGridOverlayRenderer{
		overlay: g,
		layer:   container.NewWithoutLayout(),
	}
	r.Refresh()
	return r
}

func (g *imageGridOverlay) emitSelection() {
	if g.onSelectionChanged == nil {
		return
	}

	selection, err := project.FormatGridSelection(g.selectedCells(), g.rows, g.columns)
	if err != nil {
		return
	}
	g.onSelectionChanged(selection)
}

func (g *imageGridOverlay) selectedCells() []project.GridCell {
	cells := make([]project.GridCell, 0, len(g.selected))
	for cell := range g.selected {
		cells = append(cells, cell)
	}
	return cells
}

func (g *imageGridOverlay) cellAt(pos fyne.Position, clamp bool) (project.GridCell, bool) {
	if !project.ValidGridSize(g.rows, g.columns) {
		return project.GridCell{}, false
	}

	bounds := g.imageBounds(g.Size())
	if bounds.size.Width <= 0 || bounds.size.Height <= 0 {
		return project.GridCell{}, false
	}

	if clamp {
		pos.X = min(max(pos.X, bounds.pos.X), bounds.pos.X+bounds.size.Width)
		pos.Y = min(max(pos.Y, bounds.pos.Y), bounds.pos.Y+bounds.size.Height)
	}
	if !bounds.contains(pos) {
		return project.GridCell{}, false
	}

	cellWidth := bounds.size.Width / float32(g.columns)
	cellHeight := bounds.size.Height / float32(g.rows)
	column := min(int((pos.X-bounds.pos.X)/cellWidth), g.columns-1)
	row := min(int((pos.Y-bounds.pos.Y)/cellHeight), g.rows-1)

	return project.GridCell{Row: row, Column: column}, true
}

func (g *imageGridOverlay) selectRectangle(anchor, target project.GridCell) bool {
	rowStart := min(anchor.Row, target.Row)
	rowEnd := max(anchor.Row, target.Row)
	columnStart := min(anchor.Column, target.Column)
	columnEnd := max(anchor.Column, target.Column)

	changed := false
	for row := rowStart; row <= rowEnd; row++ {
		for column := columnStart; column <= columnEnd; column++ {
			cell := project.GridCell{Row: row, Column: column}
			if _, ok := g.selected[cell]; ok {
				continue
			}
			g.selected[cell] = struct{}{}
			changed = true
		}
	}
	return changed
}

func (g *imageGridOverlay) imageBounds(size fyne.Size) gridBounds {
	if size.Width <= 0 || size.Height <= 0 {
		return gridBounds{}
	}
	if g.imageWidth <= 0 || g.imageHeight <= 0 {
		return gridBounds{size: size}
	}

	imageSize := fyne.NewSize(float32(g.imageWidth), float32(g.imageHeight))
	scale := min(size.Width/imageSize.Width, size.Height/imageSize.Height)
	drawn := fyne.NewSize(imageSize.Width*scale, imageSize.Height*scale)
	return gridBounds{
		pos:  fyne.NewPos((size.Width-drawn.Width)/2, (size.Height-drawn.Height)/2),
		size: drawn,
	}
}

type gridBounds struct {
	pos  fyne.Position
	size fyne.Size
}

func (b gridBounds) contains(pos fyne.Position) bool {
	return pos.X >= b.pos.X &&
		pos.Y >= b.pos.Y &&
		pos.X <= b.pos.X+b.size.Width &&
		pos.Y <= b.pos.Y+b.size.Height
}

type imageGridOverlayRenderer struct {
	overlay *imageGridOverlay
	layer   *fyne.Container
}

func (r *imageGridOverlayRenderer) Layout(size fyne.Size) {
	r.layer.Move(fyne.NewPos(0, 0))
	r.layer.Resize(size)
	r.rebuild(size)
}

func (r *imageGridOverlayRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *imageGridOverlayRenderer) Refresh() {
	r.rebuild(r.layer.Size())
	canvas.Refresh(r.layer)
}

func (r *imageGridOverlayRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.layer}
}

func (r *imageGridOverlayRenderer) Destroy() {}

func (r *imageGridOverlayRenderer) rebuild(size fyne.Size) {
	r.layer.Objects = nil
	if !project.ValidGridSize(r.overlay.rows, r.overlay.columns) || size.Width <= 0 || size.Height <= 0 {
		return
	}

	bounds := r.overlay.imageBounds(size)
	cellWidth := bounds.size.Width / float32(r.overlay.columns)
	cellHeight := bounds.size.Height / float32(r.overlay.rows)

	for cell := range r.overlay.selected {
		rect := canvas.NewRectangle(color.NRGBA{R: 246, G: 187, B: 12, A: 72})
		rect.Move(fyne.NewPos(
			bounds.pos.X+float32(cell.Column)*cellWidth,
			bounds.pos.Y+float32(cell.Row)*cellHeight,
		))
		rect.Resize(fyne.NewSize(cellWidth, cellHeight))
		r.layer.Add(rect)
	}

	lineColor := color.NRGBA{R: 255, G: 211, B: 25, A: 220}
	for row := 0; row <= r.overlay.rows; row++ {
		y := bounds.pos.Y + float32(row)*cellHeight
		r.layer.Add(gridLine(bounds.pos.X, y, bounds.size.Width, 2, lineColor))
	}
	for column := 0; column <= r.overlay.columns; column++ {
		x := bounds.pos.X + float32(column)*cellWidth
		r.layer.Add(gridLine(x, bounds.pos.Y, 2, bounds.size.Height, lineColor))
	}

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = theme.Yellow400
	border.StrokeWidth = 2
	border.Move(bounds.pos)
	border.Resize(bounds.size)
	r.layer.Add(border)
}

func gridLine(x, y, width, height float32, color color.Color) *canvas.Rectangle {
	line := canvas.NewRectangle(color)
	line.Move(fyne.NewPos(x, y))
	line.Resize(fyne.NewSize(width, height))
	return line
}
