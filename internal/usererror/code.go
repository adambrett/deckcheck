package usererror

import (
	"errors"
	"fmt"
)

// Coder is satisfied by any error that exposes a stable support code.
// Extractors match against this interface rather than a concrete type
// so future error implementations can carry codes too.
type Coder interface {
	Code() string
}

// Wrap tags cause with the operation sentinel and a stable support
// code in one call:
//
//	v.showError(usererror.Wrap("DC13", usererror.ErrFindNextUnclassifiedRecord, err))
//
// The code literal stays at the call site so grepping a user-reported
// code lands on the failing line ("searchable in source"). The
// operation sentinel stays matchable through the wrap via errors.Is.
func Wrap(code string, operation, cause error) error {
	return WithCode(code, fmt.Errorf("%w: %w", operation, cause))
}

// WithCode wraps err with a stable support code while preserving the
// chain's underlying message, sentinels, and wrap relationships. Use
// [Wrap] when an operation sentinel is also being attached. Returns
// nil when err is nil so callers can apply it unconditionally. Nested
// wraps are tolerated; [CodeOf] surfaces the outermost code.
func WithCode(code string, err error) error {
	if err == nil {
		return nil
	}

	return &codedWrap{code: code, err: err}
}

// CodeOf returns the first non-empty Code found in err's tree, or ""
// when no error in the chain implements [Coder] with a code set. Uses
// errors.As under the hood so multi-error wraps from fmt.Errorf with
// multiple %w verbs are traversed correctly. Outer codes are found
// first, so an outer wrap takes precedence over inner ones.
func CodeOf(err error) string {
	var coder Coder
	if errors.As(err, &coder) {
		return coder.Code()
	}

	return ""
}

// codedWrap pairs a support code with an arbitrary error. Error and
// Unwrap delegate to the wrapped value so errors.Is / errors.As walks
// see the original chain unchanged.
type codedWrap struct {
	code string
	err  error
}

func (c *codedWrap) Error() string {
	if c == nil || c.err == nil {
		return ""
	}

	return c.err.Error()
}

func (c *codedWrap) Code() string {
	if c == nil {
		return ""
	}

	return c.code
}

func (c *codedWrap) Unwrap() error {
	if c == nil {
		return nil
	}

	return c.err
}
