//go:build integration

package form_test

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/ui/views/wizard/widgets/form"
)

func TestGroupHelpers(t *testing.T) {
	// Given
	test.NewApp()

	label := widget.NewLabel("Name")
	entry := widget.NewEntry()

	// When
	group := form.Group(label, entry)
	groups := form.Groups(group)

	// Then
	require.Len(t, group.Objects, 2)
	require.Len(t, groups.Objects, 1)
}

func TestFieldErrorState(t *testing.T) {
	// Given
	test.NewApp()

	// When
	field := form.NewField(widget.NewLabel("Name"), widget.NewEntry())

	// Then
	require.NotNil(t, field.Container())
	require.False(t, field.ErrorVisible())

	// When
	field.SetError("Enter a name.")

	// Then
	require.True(t, field.ErrorVisible())
	require.Equal(t, "Enter a name.", field.ErrorText())

	// When
	field.SetError("")

	// Then
	require.False(t, field.ErrorVisible())
}
