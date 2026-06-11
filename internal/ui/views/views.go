package views

import "fyne.io/fyne/v2"

// View is implemented by each top-level screen the App can activate.
//
// Activate is called once just before the view is installed as the
// window's content; it should load any data the view needs to render.
// Container returns the visual content that will be installed. Size
// is the preferred initial window size when the view becomes active.
//
// Views that own background work (timers, goroutines) should also
// satisfy [Closer] so the App can tear that work down cleanly when
// the view is replaced.
type View interface {
	Activate() error
	Container() fyne.CanvasObject
	Size() fyne.Size
}

// Closer is implemented by views that own background work the App
// needs to cancel when the view is replaced.
type Closer interface {
	Close()
}

// Quiescer is implemented by views that own background writes the
// App must let drain before tearing shared resources down. Quiesce
// runs then (on a fresh goroutine) once every pending operation has
// finished.
type Quiescer interface {
	Quiesce(then func())
}

// CloseGuard is implemented by views that may hold unsaved user
// input. ConfirmClose asks the view to verify with the user and must
// call done exactly once, with proceed true to continue the close
// false when the user declined. A view with nothing to lose calls
// done(true) directly.
type CloseGuard interface {
	ConfirmClose(done func(proceed bool))
}
