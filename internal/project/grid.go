package project

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// GridCell identifies one zero-based cell in an image-grid question.
type GridCell struct {
	Row    int
	Column int
}

// GridCellBounds describes the source-image pixel rectangle covered by
// one selected grid cell.
type GridCellBounds struct {
	Cell   string `json:"cell"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ValidGridSize reports whether rows and columns are within the range
// the UI can render and label clearly.
func ValidGridSize(rows, columns int) bool {
	return rows >= MinGridSize && rows <= MaxGridSize &&
		columns >= MinGridSize && columns <= MaxGridSize
}

// GridCellLabel returns the CSV/UI label for a zero-based cell.
func GridCellLabel(cell GridCell) string {
	return columnLabel(cell.Column) + strconv.Itoa(cell.Row+1)
}

// NormalizeGridSelection parses, validates, de-duplicates, sorts, and
// formats a grid selection for storage.
func NormalizeGridSelection(value string, rows, columns int) (string, error) {
	cells, err := ParseGridSelection(value, rows, columns)
	if err != nil {
		return "", err
	}

	return FormatGridSelection(cells, rows, columns)
}

// ParseGridSelection turns a comma- or whitespace-separated cell list
// into zero-based cells. Duplicate cells are collapsed.
func ParseGridSelection(value string, rows, columns int) ([]GridCell, error) {
	if !ValidGridSize(rows, columns) {
		return nil, fmt.Errorf("invalid grid size %dx%d", rows, columns)
	}

	tokens := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(tokens) == 0 {
		return nil, nil
	}

	seen := make(map[GridCell]struct{}, len(tokens))
	cells := make([]GridCell, 0, len(tokens))
	for _, token := range tokens {
		cell, err := parseGridCell(token, rows, columns)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[cell]; ok {
			continue
		}
		seen[cell] = struct{}{}
		cells = append(cells, cell)
	}

	sortGridCells(cells)
	return cells, nil
}

// FormatGridSelection serialises cells as space-separated labels in
// row-major order.
func FormatGridSelection(cells []GridCell, rows, columns int) (string, error) {
	if !ValidGridSize(rows, columns) {
		return "", fmt.Errorf("invalid grid size %dx%d", rows, columns)
	}

	if len(cells) == 0 {
		return "", nil
	}

	out := make([]GridCell, len(cells))
	copy(out, cells)
	sortGridCells(out)

	labels := make([]string, 0, len(out))
	seen := make(map[GridCell]struct{}, len(out))
	for _, cell := range out {
		if cell.Row < 0 || cell.Row >= rows || cell.Column < 0 || cell.Column >= columns {
			return "", fmt.Errorf("grid cell %s is outside %dx%d", GridCellLabel(cell), rows, columns)
		}
		if _, ok := seen[cell]; ok {
			continue
		}
		seen[cell] = struct{}{}
		labels = append(labels, GridCellLabel(cell))
	}

	return strings.Join(labels, " "), nil
}

// GridSelectionBounds converts selected grid cells into source-image
// pixel rectangles. Remainder pixels are distributed by integer
// boundaries, so all cells together cover the image exactly.
func GridSelectionBounds(value string, rows, columns, imageWidth, imageHeight int) ([]GridCellBounds, error) {
	if imageWidth <= 0 || imageHeight <= 0 {
		return nil, fmt.Errorf("invalid image size %dx%d", imageWidth, imageHeight)
	}

	cells, err := ParseGridSelection(value, rows, columns)
	if err != nil {
		return nil, err
	}

	bounds := make([]GridCellBounds, 0, len(cells))
	for _, cell := range cells {
		x0 := cell.Column * imageWidth / columns
		x1 := (cell.Column + 1) * imageWidth / columns
		y0 := cell.Row * imageHeight / rows
		y1 := (cell.Row + 1) * imageHeight / rows

		bounds = append(bounds, GridCellBounds{
			Cell:   GridCellLabel(cell),
			X:      x0,
			Y:      y0,
			Width:  x1 - x0,
			Height: y1 - y0,
		})
	}

	return bounds, nil
}

// FormatGridSelectionBoundsJSON returns a compact JSON array for CSV
// export, preserving the same row-major order as the stored labels.
func FormatGridSelectionBoundsJSON(value string, rows, columns, imageWidth, imageHeight int) (string, error) {
	bounds, err := GridSelectionBounds(value, rows, columns, imageWidth, imageHeight)
	if err != nil {
		return "", err
	}

	encoded, err := json.Marshal(bounds)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseGridCell(token string, rows, columns int) (GridCell, error) {
	token = strings.ToUpper(strings.TrimSpace(token))
	if token == "" {
		return GridCell{}, fmt.Errorf("empty grid cell")
	}

	split := 0
	for split < len(token) && token[split] >= 'A' && token[split] <= 'Z' {
		split++
	}
	if split == 0 || split == len(token) {
		return GridCell{}, fmt.Errorf("invalid grid cell %q", token)
	}

	column := 0
	for _, r := range token[:split] {
		column = column*26 + int(r-'A'+1)
	}
	column--

	row, err := strconv.Atoi(token[split:])
	if err != nil || row < 1 {
		return GridCell{}, fmt.Errorf("invalid grid cell %q", token)
	}
	row--

	cell := GridCell{Row: row, Column: column}
	if row >= rows || column >= columns {
		return GridCell{}, fmt.Errorf("grid cell %q is outside %dx%d", token, rows, columns)
	}
	return cell, nil
}

func sortGridCells(cells []GridCell) {
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Row == cells[j].Row {
			return cells[i].Column < cells[j].Column
		}
		return cells[i].Row < cells[j].Row
	})
}

func columnLabel(column int) string {
	if column < 0 {
		return ""
	}

	var out []byte
	column++
	for column > 0 {
		column--
		out = append([]byte{byte('A' + column%26)}, out...)
		column /= 26
	}
	return string(out)
}
