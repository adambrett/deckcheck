package picker

import "errors"

// ErrMissingPicker is returned when NewProject is called without a
// backing picker.
var ErrMissingPicker = errors.New("backing picker is required")
