//go:build integration

package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/ui/views/lifecycle"
)

func TestActivateDerivesContextFromParent(t *testing.T) {
	// Given a lifecycle rooted at a cancellable parent
	parent, cancel := context.WithCancel(context.Background())
	life := lifecycle.New(parent)

	// When
	life.Activate()

	// Then the activation context is live until the parent cancels.
	require.NoError(t, life.Context().Err())
	cancel()
	require.ErrorIs(t, life.Context().Err(), context.Canceled)
}

func TestActivateWithNilParentUsesBackground(t *testing.T) {
	// Given
	life := lifecycle.New(nil)

	// When
	life.Activate()

	// Then
	require.NotNil(t, life.Context())
	require.NoError(t, life.Context().Err())
}

func TestCloseCancelsActivationContext(t *testing.T) {
	// Given an activated lifecycle
	life := lifecycle.New(nil)
	life.Activate()
	require.False(t, life.Closed())

	// When
	life.Close()

	// Then
	require.True(t, life.Closed())
	require.ErrorIs(t, life.Context().Err(), context.Canceled)
}

func TestCloseBeforeActivateIsSafe(t *testing.T) {
	// Given
	life := lifecycle.New(nil)

	// When / Then no panic, and the lifecycle reads as closed.
	require.NotPanics(t, life.Close)
	require.True(t, life.Closed())
}

func TestReactivateResetsClosedAndCancelsPreviousContext(t *testing.T) {
	// Given a closed lifecycle
	life := lifecycle.New(nil)
	life.Activate()
	previous := life.Context()
	life.Close()

	// When
	life.Activate()

	// Then the new activation is fresh and the old context stays dead.
	require.False(t, life.Closed())
	require.NoError(t, life.Context().Err())
	require.ErrorIs(t, previous.Err(), context.Canceled)
}

func TestGoDeliversApplyOnFyneGoroutine(t *testing.T) {
	// Given
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	var got context.Context
	applied := false

	// When
	life.Go(func(ctx context.Context) func() {
		got = ctx
		return func() { applied = true }
	})
	life.Wait()

	// Then work received the activation context and apply ran.
	require.Equal(t, life.Context(), got)
	require.True(t, applied)
}

func TestGoBeforeActivateFallsBackToBackgroundContext(t *testing.T) {
	// Given a lifecycle that was never activated
	test.NewApp()
	life := lifecycle.New(nil)

	var got context.Context

	// When
	life.Go(func(ctx context.Context) func() {
		got = ctx
		return func() {}
	})
	life.Wait()

	// Then work received a usable context, not nil.
	require.NotNil(t, got)
	require.NoError(t, got.Err())
}

func TestGoDropsApplyFromStaleActivation(t *testing.T) {
	// Given work started under one activation that completes only
	// after the lifecycle has been re-activated
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	release := make(chan struct{})
	applied := false
	life.Go(func(context.Context) func() {
		<-release
		return func() { applied = true }
	})

	// When a fresh activation replaces the one the work belongs to
	life.Activate()
	close(release)
	life.Wait()

	// Then the stale apply is dropped rather than delivered into the
	// new activation.
	require.False(t, applied)
}

func TestGoSerialPreservesSubmissionOrder(t *testing.T) {
	// Given a first job that cannot finish until released, and a
	// second job submitted while the first is still blocked
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	release := make(chan struct{})
	var order []int

	// When
	life.GoSerial(func(context.Context) func() {
		<-release
		return func() { order = append(order, 1) }
	})
	life.GoSerial(func(context.Context) func() {
		return func() { order = append(order, 2) }
	})
	close(release)
	life.Wait()

	// Then the second job neither started nor delivered before the
	// first: submission order is the delivery order.
	require.Equal(t, []int{1, 2}, order)
}

func TestAfterDeliversApplyOnceDelayElapses(t *testing.T) {
	// Given
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	applied := false

	// When
	life.After(time.Millisecond, func() { applied = true })
	life.Wait()

	// Then
	require.True(t, applied)
}

func TestAfterCancelDisarmsTimer(t *testing.T) {
	// Given an armed timer with a generous delay
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	applied := false
	cancel := life.After(time.Hour, func() { applied = true })

	// When cancelling before it fires
	cancel()
	life.Wait()

	// Then the apply never runs and Wait does not hang.
	require.False(t, applied)
}

func TestAfterDropsApplyAfterClose(t *testing.T) {
	// Given an armed timer on a lifecycle that closes before it fires
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	applied := false
	life.After(time.Millisecond, func() { applied = true })

	// When
	life.Close()
	life.Wait()

	// Then the late firing is dropped.
	require.False(t, applied)
}

func TestGoDropsApplyAfterClose(t *testing.T) {
	// Given a lifecycle whose owner closes while work is in flight
	test.NewApp()
	life := lifecycle.New(nil)
	life.Activate()

	applied := false

	// When the work step itself triggers the close (the latest moment
	// it can happen before apply would run)
	life.Go(func(context.Context) func() {
		life.Close()
		return func() { applied = true }
	})
	life.Wait()

	// Then the late apply is dropped silently.
	require.False(t, applied)
}
