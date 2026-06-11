package usererror_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/usererror"
)

func TestWrapCarriesCodeAndSentinel(t *testing.T) {
	// Given
	cause := errors.New("disk is gone")

	// When
	err := usererror.Wrap("TESTING42", usererror.ErrSaveClassification, cause)

	// Then the code, the operation sentinel, and the cause all survive.
	require.Equal(t, "TESTING42", usererror.CodeOf(err))
	require.ErrorIs(t, err, usererror.ErrSaveClassification)
	require.ErrorContains(t, err, "disk is gone")
}

func TestWithCodeNilErrorReturnsNil(t *testing.T) {
	// When / Then
	require.NoError(t, usererror.WithCode("TESTING42", nil))
}

func TestCodeOfFindsCodeThroughWrap(t *testing.T) {
	// Given a coded wrap buried under further fmt.Errorf wrapping
	err := fmt.Errorf("outer context: %w",
		usererror.WithCode("TESTING42", errors.New("inner failure")))

	// When / Then
	require.Equal(t, "TESTING42", usererror.CodeOf(err))
}

func TestCodeOfPrefersOutermostCode(t *testing.T) {
	// Given nested coded wraps
	err := usererror.WithCode("TESTING2",
		usererror.WithCode("TESTING1", errors.New("inner failure")))

	// When / Then the outer wrap wins.
	require.Equal(t, "TESTING2", usererror.CodeOf(err))
}

func TestCodeOfNoCoderInChain(t *testing.T) {
	// When / Then
	require.Empty(t, usererror.CodeOf(fmt.Errorf("ordinary error: %w", errors.New("inner"))))
}

func TestCodeOfNilError(t *testing.T) {
	// When / Then
	require.Empty(t, usererror.CodeOf(nil))
}
