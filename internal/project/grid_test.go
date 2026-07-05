package project_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/project"
)

func TestNormalizeGridSelection(t *testing.T) {
	// When
	actual, err := project.NormalizeGridSelection("c2, A1 a1 B1", 3, 3)

	// Then cells are de-duplicated and stored in row-major order.
	require.NoError(t, err)
	require.Equal(t, "A1 B1 C2", actual)
}

func TestNormalizeGridSelectionAllowsExplicitEmptySelection(t *testing.T) {
	// When
	actual, err := project.NormalizeGridSelection("  ", 3, 3)

	// Then
	require.NoError(t, err)
	require.Empty(t, actual)
}

func TestNormalizeGridSelectionRejectsInvalidCells(t *testing.T) {
	// When
	_, err := project.NormalizeGridSelection("D1", 3, 3)

	// Then
	require.Error(t, err)
}

func TestFormatGridSelectionRejectsOutOfBoundsCells(t *testing.T) {
	// When
	_, err := project.FormatGridSelection([]project.GridCell{{Row: 0, Column: 3}}, 3, 3)

	// Then
	require.Error(t, err)
}

func TestGridSelectionBounds(t *testing.T) {
	// When
	actual, err := project.GridSelectionBounds("C2 A1", 3, 3, 100, 100)

	// Then bounds are sorted by cell and cover the uneven final columns.
	require.NoError(t, err)
	require.Equal(t, []project.GridCellBounds{
		{Cell: "A1", X: 0, Y: 0, Width: 33, Height: 33},
		{Cell: "C2", X: 66, Y: 33, Width: 34, Height: 33},
	}, actual)
}

func TestFormatGridSelectionBoundsJSON(t *testing.T) {
	// When
	actual, err := project.FormatGridSelectionBoundsJSON("A1", 3, 3, 90, 60)

	// Then
	require.NoError(t, err)
	require.Equal(t, `[{"cell":"A1","x":0,"y":0,"width":30,"height":20}]`, actual)
}
