// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/invowk/invowk/pkg/types"
)

// These tests bind formal/tla/Watch.tla to the real Watcher.Run. The loop's
// WaitGroup accounting depends on what debounceTimer.Reset and Stop return, so
// the fake timer must return exactly what time.AfterFunc returns; the
// conformance test checks that before the scenario relies on it.

type (
	// afterFuncScheduler hands out afterFuncTimers that fire only when the test
	// says so, but otherwise behave like time.AfterFunc timers.
	afterFuncScheduler struct {
		mu     sync.Mutex
		timer  *afterFuncTimer
		signal chan struct{}
	}

	afterFuncTimer struct {
		mu      sync.Mutex
		fn      func()
		armed   bool // pending: Reset and Stop return true
		resets  int
		changed chan struct{}
	}
)

func newAfterFuncScheduler() *afterFuncScheduler {
	return &afterFuncScheduler{signal: make(chan struct{}, 16)}
}

func (s *afterFuncScheduler) Schedule(_ time.Duration, fn func()) debounceTimer {
	timer := &afterFuncTimer{fn: fn, armed: true, changed: make(chan struct{}, 16)}
	s.mu.Lock()
	s.timer = timer
	s.mu.Unlock()
	s.signal <- struct{}{}
	return timer
}

func (s *afterFuncScheduler) waitTimer(t *testing.T) *afterFuncTimer {
	t.Helper()
	select {
	case <-s.signal:
	case <-time.After(5 * time.Second):
		t.Fatal("debounce timer was never scheduled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.timer
}

// Reset re-arms the timer and reports whether it was still pending, like
// time.Timer.Reset for an AfterFunc timer.
func (t *afterFuncTimer) Reset(time.Duration) bool {
	t.mu.Lock()
	wasArmed := t.armed
	t.armed = true
	t.resets++
	t.mu.Unlock()
	t.changed <- struct{}{}
	return wasArmed
}

// Stop disarms the timer and reports whether it was still pending.
func (t *afterFuncTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasArmed := t.armed
	t.armed = false
	return wasArmed
}

// expire fires a pending timer on its own goroutine, as time.AfterFunc does.
func (t *afterFuncTimer) expire(tb *testing.T) <-chan struct{} {
	tb.Helper()
	t.mu.Lock()
	if !t.armed {
		t.mu.Unlock()
		tb.Fatal("expire() on a timer that is not pending")
	}
	t.armed = false
	t.mu.Unlock()
	done := make(chan struct{})
	go func() { defer close(done); t.fn() }()
	return done
}

func (t *afterFuncTimer) waitResets(tb *testing.T, want int) {
	tb.Helper()
	for {
		t.mu.Lock()
		got := t.resets
		t.mu.Unlock()
		if got >= want {
			return
		}
		select {
		case <-t.changed:
		case <-time.After(5 * time.Second):
			tb.Fatalf("timer resets = %d, want %d", got, want)
		}
	}
}

// TestWatch_ManualTimerMatchesAfterFuncContract checks the fake against the
// real time.AfterFunc: Reset and Stop return true while pending, false after
// the timer fired.
func TestWatch_ManualTimerMatchesAfterFuncContract(t *testing.T) {
	t.Parallel()

	type observation struct{ resetPending, stopPending, resetFired, stopFired bool }
	observe := func(schedule func(func()) debounceTimer, fire func(debounceTimer)) observation {
		var o observation
		timer := schedule(func() {})
		o.resetPending = timer.Reset(time.Hour)
		o.stopPending = timer.Stop()
		timer.Reset(time.Hour)
		fire(timer)
		o.resetFired = timer.Reset(time.Hour)
		fire(timer)
		o.stopFired = timer.Stop()
		return o
	}

	afterFunc := observe(
		func(fn func()) debounceTimer { return time.AfterFunc(time.Hour, fn) },
		func(timer debounceTimer) {
			fired := make(chan struct{})
			timer.Reset(0)
			// Wait until the zero-duration timer has fired.
			go func() {
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if !timer.Stop() {
						close(fired)
						return
					}
					timer.Reset(0)
					time.Sleep(time.Millisecond)
				}
			}()
			<-fired
		},
	)
	fake := observe(
		func(fn func()) debounceTimer {
			return &afterFuncTimer{fn: fn, armed: true, changed: make(chan struct{}, 16)}
		},
		func(timer debounceTimer) { <-timer.(*afterFuncTimer).expire(t) },
	)
	if fake != afterFunc {
		t.Fatalf("fake timer %+v, time.AfterFunc %+v", fake, afterFunc)
	}
}

// TestWatch_DeliversEveryBurstThroughSkipIfBusy replays the model's
// skip-if-busy path: a second burst arrives while the first callback runs, the
// fire that finds the callback busy re-arms the timer, and the second burst is
// delivered (NoLostBurst). Run then returns only after in-flight work is done
// (NothingAfterReturn), which also requires balanced WaitGroup accounting.
func TestWatch_DeliversEveryBurstThroughSkipIfBusy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backend := newFakeWatcherBackend()
	w, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: time.Hour,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() error = %v", err)
	}

	var (
		mu           sync.Mutex
		delivered    []string
		firstStarted = make(chan struct{})
		releaseFirst = make(chan struct{})
		calls        int
	)
	w.cfg.OnChange = func(_ context.Context, changed []string) error {
		mu.Lock()
		calls++
		call := calls
		delivered = append(delivered, changed...)
		mu.Unlock()
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		return nil
	}
	scheduler := newAfterFuncScheduler()
	w.schedule = scheduler.Schedule

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()
	<-w.Ready()

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "first.go"), Op: fsnotify.Write}
	timer := scheduler.waitTimer(t)
	firstDone := timer.expire(t)
	<-firstStarted

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "second.go"), Op: fsnotify.Write}
	timer.waitResets(t, 1) // the loop re-arms the expired timer
	busyDone := timer.expire(t)
	<-busyDone // the busy fire re-armed the timer instead of dropping the burst
	timer.waitResets(t, 2)

	close(releaseFirst)
	<-firstDone
	<-timer.expire(t) // the retried fire delivers the second burst

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return: WaitGroup accounting is unbalanced")
	}

	mu.Lock()
	defer mu.Unlock()
	slices.Sort(delivered)
	if !slices.Equal(delivered, []string{"first.go", "second.go"}) {
		t.Fatalf("delivered %v, want both bursts", delivered)
	}
}
