package project_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/project"
)

func TestRecordHasImage(t *testing.T) {
	// Given
	withImage := &project.Record{ImagePath: "/tmp/cat.png"}
	withoutImage := &project.Record{}

	// When / Then
	require.True(t, withImage.HasImage())
	require.False(t, withoutImage.HasImage())
}
