package project_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/project"
)

func TestDefaultFilename(t *testing.T) {
	// Given
	tests := map[string]string{
		"My Project":        "My Project.deckcheck",
		"  My   Project  ":  "My Project.deckcheck",
		"Project.deckcheck": "Project.deckcheck",
		"":                  "project.deckcheck",
		"Team/Review:One":   "Team-Review-One.deckcheck",
	}

	for name, expected := range tests {
		t.Run(name, func(t *testing.T) {
			// Given

			// When
			actual := project.DefaultFilename(name)

			// Then
			require.Equal(t, expected, actual)
		})
	}
}
