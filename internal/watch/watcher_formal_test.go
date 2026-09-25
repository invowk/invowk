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

	fired := make(chan struct{}, 4)
	afterFunc := observe(
		func(fn func()) debounceTimer {
			return time.AfterFunc(time.Hour, func() { fn(); fired <- struct{}{} })
		},
		func(timer debounceTimer) { timer.Reset(0); <-fired },
	)
	fake := observe(
		func(fn func()) debounceTimer {
			return &manualDebounceTimer{fire: fn, armed: true, reset: make(chan struct{}, 1)}
		},
		func(timer debounceTimer) { <-timer.(*manualDebounceTimer).fireAsync() },
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
	scheduler := newManualDebounceScheduler()
	w.schedule = scheduler.Schedule

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()
	<-w.Ready()

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "first.go"), Op: fsnotify.Write}
	timer := scheduler.requireTimer(t)
	firstDone := timer.fireAsync()
	<-firstStarted

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "second.go"), Op: fsnotify.Write}
	timer.waitForResetCount(t, 1) // the loop re-arms the expired timer
	busyDone := timer.fireAsync()
	<-busyDone // the busy fire re-armed the timer instead of dropping the burst
	timer.waitForResetCount(t, 2)

	close(releaseFirst)
	<-firstDone
	<-timer.fireAsync() // the retried fire delivers the second burst

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
