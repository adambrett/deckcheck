package lifecycle

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
)

// Lifecycle bundles the machinery every view (and the App) needs to
// run work off the Fyne goroutine safely: a per-activation context
// derived from a long-lived parent, a closed flag that drops late
// completions after teardown, and a pending counter tests use to wait
// for quiescence.
//
// Activate, Close, Go, and After must be called from the Fyne
// goroutine; they read and write the activation state without locks,
// which is safe only because the UI is single-threaded. Closed and
// Wait are safe from any goroutine; background callbacks rely on
// exactly that.
type Lifecycle struct {
	// parentCtx is the long-lived context each per-activation context
	// is derived from; nil is treated as context.Background.
	parentCtx context.Context

	// ctx and cancel scope every operation issued during the current
	// activation. They are populated by Activate and torn down by
	// Close, so closing the owner cancels in-flight work.
	ctx    context.Context
	cancel context.CancelFunc

	// closed is set by Close and read by background-work callbacks to
	// short-circuit late completions after the owner has been torn
	// down. atomic.Bool because those reads genuinely race with Close.
	closed atomic.Bool

	// pending counts in-flight background operations started by Go,
	// GoSerial, and After so tests (and a future graceful shutdown)
	// can wait for quiescence.
	pending sync.WaitGroup

	// serialTail is the completion signal of the most recently
	// submitted GoSerial job; the next job waits on it, which is what
	// serialises the queue. Touched only on the Fyne goroutine.
	serialTail chan struct{}

	// applyMu serialises apply deliveries: uncontended on the real
	// event loop, and the ordering guarantee when DoAndWait executes
	// inline (as the Fyne test driver does).
	applyMu sync.Mutex
}

// New returns a Lifecycle rooted at parent. parent may be nil, in
// which case per-activation contexts derive from context.Background.
// Activate must run before the first Go call.
func New(parent context.Context) *Lifecycle {
	return &Lifecycle{parentCtx: parent}
}

// Activate begins a fresh activation: any previous activation context
// is cancelled, a new one is derived from the parent, and the closed
// flag is reset so background work delivers again.
func (l *Lifecycle) Activate() {
	l.closed.Store(false)
	if l.cancel != nil {
		l.cancel()
	}
	parent := l.parentCtx
	if parent == nil {
		parent = context.Background()
	}
	l.ctx, l.cancel = context.WithCancel(parent)
}

// Close tears the current activation down: the closed flag drops any
// late background completions and the activation context is cancelled
// so in-flight operations are interrupted. Close is safe to call
// multiple times.
func (l *Lifecycle) Close() {
	l.closed.Store(true)
	if l.cancel != nil {
		l.cancel()
	}
}

// Context returns the current activation's context. It is nil before
// the first Activate call.
func (l *Lifecycle) Context() context.Context {
	return l.ctx
}

// Closed reports whether Close has run since the last Activate. Late
// callbacks (timers, goroutines) check it before touching the owner.
func (l *Lifecycle) Closed() bool {
	return l.closed.Load()
}

// Go executes work off the Fyne goroutine, then runs the apply step
// work returns back on the Fyne goroutine. work must not touch any
// widget; everything UI-bound belongs in the apply step. The apply
// step is dropped entirely if Close ran in the meantime, checked
// before posting to the Fyne thread (whose event loop may already be
// gone) and again inside it (Close may have won the race), or if a
// newer activation has replaced the one the work was started under.
func (l *Lifecycle) Go(work func(ctx context.Context) func()) {
	ctx := l.ctx
	if ctx == nil {
		// Go before the first Activate: fall back to Background so
		// work never receives a nil context.
		ctx = context.Background()
	}

	l.pending.Add(1)
	go func() {
		defer l.pending.Done()

		apply := work(ctx)

		if l.closed.Load() {
			return
		}
		fyne.DoAndWait(func() { l.deliver(ctx, apply) })
	}()
}

// GoSerial executes work like Go, but jobs submitted through GoSerial
// run strictly in submission order: each waits for the previous one
// to finish before its work starts. Use it for writes, where a later
// delete must never overtake an earlier save; independent reads can
// use Go and run concurrently.
func (l *Lifecycle) GoSerial(work func(ctx context.Context) func()) {
	ctx := l.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	prev := l.serialTail
	done := make(chan struct{})
	l.serialTail = done

	l.pending.Add(1)
	go func() {
		defer l.pending.Done()
		defer close(done)

		if prev != nil {
			<-prev
		}

		apply := work(ctx)

		if l.closed.Load() {
			return
		}
		fyne.DoAndWait(func() { l.deliver(ctx, apply) })
	}()
}

// After arms a one-shot timer that runs apply on the Fyne goroutine
// once delay elapses, with the same drop rules as Go: a timer armed
// under an earlier activation is dropped once Activate runs again.
// The returned cancel function disarms a timer that has not fired
// yet; calling it after the timer fired is a no-op.
func (l *Lifecycle) After(delay time.Duration, apply func()) (cancel func()) {
	ctx := l.ctx
	if ctx == nil {
		// Pre-Activate arm: mirror Go's fallback so dropDelivery has
		// a real context to compare against.
		ctx = context.Background()
	}

	l.pending.Add(1)
	timer := time.AfterFunc(delay, func() {
		defer l.pending.Done()

		if l.closed.Load() {
			return
		}
		// DoAndWait assumes the event loop outlives this delivery; the
		// closed pre-check plus Close-before-window-teardown in every
		// owner keeps that true until process exit.
		fyne.DoAndWait(func() { l.deliver(ctx, apply) })
	})

	return func() {
		if timer.Stop() {
			l.pending.Done()
		}
	}
}

// deliver runs apply under the delivery mutex unless the activation
// it belongs to has been closed or replaced.
func (l *Lifecycle) deliver(ctx context.Context, apply func()) {
	l.applyMu.Lock()
	defer l.applyMu.Unlock()

	if l.dropDelivery(ctx) {
		return
	}
	apply()
}

// dropDelivery reports whether a callback that started under ctx must
// be discarded: the owner closed, or a newer activation replaced the
// one the work belongs to. Runs on the Fyne goroutine, where reading
// l.ctx is race-free.
func (l *Lifecycle) dropDelivery(ctx context.Context) bool {
	if l.closed.Load() {
		return true
	}
	// l.ctx is nil only before the first Activate, when there is no
	// newer activation to be stale against.
	return l.ctx != nil && l.ctx != ctx
}

// Wait blocks until every background operation started by Go or After
// has delivered (or dropped) its result. Intended for tests waiting
// for quiescence.
func (l *Lifecycle) Wait() {
	l.pending.Wait()
}
