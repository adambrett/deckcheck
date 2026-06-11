package fynetest

import "github.com/adambrett/go-fyne/pkg/browse"

// StubPicker satisfies browse.Picker with canned paths instead of a
// native dialog. Every call is recorded with the options it received,
// the canned path (when set) is delivered through onSelected, and the
// caller's OnClosed hook is honoured, mirroring how the real pickers
// behave. The one shared stub for tests that drive picker flows
// without asserting call expectations (use the generated mock for
// those).
type StubPicker struct {
	OpenPath string
	SavePath string

	OpenCalls []browse.OpenOptions
	SaveCalls []browse.SaveOptions
}

// Open records the call, delivers OpenPath when set, and fires OnClosed.
func (p *StubPicker) Open(options browse.OpenOptions, onSelected func(path string)) {
	p.OpenCalls = append(p.OpenCalls, options)
	if p.OpenPath != "" {
		onSelected(p.OpenPath)
	}
	if options.OnClosed != nil {
		options.OnClosed()
	}
}

// Save records the call, delivers SavePath when set, and fires OnClosed.
func (p *StubPicker) Save(options browse.SaveOptions, onSelected func(path string)) {
	p.SaveCalls = append(p.SaveCalls, options)
	if p.SavePath != "" {
		onSelected(p.SavePath)
	}
	if options.OnClosed != nil {
		options.OnClosed()
	}
}
