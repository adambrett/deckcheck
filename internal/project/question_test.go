package project_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/project"
)

func TestParseAnswers(t *testing.T) {
	// Given
	tests := map[string]struct {
		input    string
		expected []string
	}{
		"splits on commas": {
			input:    "Yes,No,Maybe",
			expected: []string{"Yes", "No", "Maybe"},
		},
		"trims surrounding whitespace": {
			input:    "  Yes , No ,\tMaybe ",
			expected: []string{"Yes", "No", "Maybe"},
		},
		"drops empty parts": {
			input:    "Yes,,No,  ,",
			expected: []string{"Yes", "No"},
		},
		"single answer": {
			input:    "Yes",
			expected: []string{"Yes"},
		},
		"empty input yields empty slice": {
			input:    "",
			expected: []string{},
		},
		"whitespace-only input yields empty slice": {
			input:    "  , \t ,  ",
			expected: []string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// When
			actual := project.ParseAnswers(tt.input)

			// Then
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestFindDuplicateAnswer(t *testing.T) {
	// Given
	tests := map[string]struct {
		answers           []string
		expectedDuplicate string
		expectedFound     bool
	}{
		"no duplicates": {
			answers:           []string{"Yes", "No", "Maybe"},
			expectedDuplicate: "",
			expectedFound:     false,
		},
		"case-insensitive duplicate returns the second spelling": {
			answers:           []string{"Yes", "no", "YES"},
			expectedDuplicate: "YES",
			expectedFound:     true,
		},
		"trims before comparing": {
			answers:           []string{"Yes", "  yes "},
			expectedDuplicate: "  yes ",
			expectedFound:     true,
		},
		"empty slice": {
			answers:           nil,
			expectedDuplicate: "",
			expectedFound:     false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// When
			duplicate, found := project.FindDuplicateAnswer(tt.answers)

			// Then
			require.Equal(t, tt.expectedFound, found)
			require.Equal(t, tt.expectedDuplicate, duplicate)
		})
	}
}
