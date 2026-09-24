// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/invowk/invowk/internal/testutil/tlatrace"
	"github.com/invowk/invowk/pkg/types"
)

// modelEvent maps a watched path to its Watch.tla event.
var modelEvent = map[string]string{"first.go": "ev1", "second.go": "ev2"}

func watchRecord(delivered []string, callbacks int, timer string, returned bool) tlatrace.Record {
	return tlatrace.Record{
		"delivered": tlatrace.Set(delivered...),
		"callbacks": strconv.Itoa(callbacks),
		"timer":     tlatrace.Str(timer),
		"returned":  tlatrace.Bool(returned),
	}
}

// TestWatch_TraceHarness records the skip-if-busy scenario on the real
// Watcher.Run for trace validation against Watch.tla, plus targeted mutations
// that validation must reject.
func TestWatch_TraceHarness(t *testing.T) {
	t.Parallel()
	traceDir := tlatrace.Dir(t)

	dir := t.TempDir()
	backend := newFakeWatcherBackend()
	w, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir), Debounce: time.Hour,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() error = %v", err)
	}

	var (
		mu           sync.Mutex
		trace        []tlatrace.Record
		delivered    []string
		inCallback   int
		timerState   = "none"
		calls        int
		firstStarted = make(chan struct{})
		releaseFirst = make(chan struct{})
	)
	record := func() {
		mu.Lock()
		defer mu.Unlock()
		trace = append(trace, watchRecord(delivered, inCallback, timerState, false))
	}
	setTimer := func(state string) {
		mu.Lock()
		timerState = state
		mu.Unlock()
		record()
	}
	w.cfg.OnChange = func(_ context.Context, changed []string) error {
		mu.Lock()
		calls++
		call := calls
		inCallback++
		mu.Unlock()
		record() // FireStart: the callback begins
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		mu.Lock()
		for _, path := range changed {
			delivered = append(delivered, modelEvent[path])
		}
		inCallback--
		mu.Unlock()
		return nil
	}
	scheduler := newManualDebounceScheduler()
	w.schedule = scheduler.Schedule

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runErr := make(chan error, 1)
	record()
	go func() { runErr <- w.Run(ctx) }()
	<-w.Ready()

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "first.go"), Op: fsnotify.Write}
	timer := scheduler.requireTimer(t)
	setTimer("armed")
	firstDone := timer.fireAsyncAfter(func() { setTimer("expired") })
	<-firstStarted

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "second.go"), Op: fsnotify.Write}
	timer.waitForResetCount(t, 1)
	setTimer("armed")
	busyDone := timer.fireAsyncAfter(func() { setTimer("expired") })
	<-busyDone
	timer.waitForResetCount(t, 2)
	setTimer("armed")

	close(releaseFirst)
	<-firstDone
	record() // CallbackReturn: the first burst is delivered
	lastDone := timer.fireAsyncAfter(func() { setTimer("expired") })
	<-lastDone
	record() // CallbackReturn: the second burst is delivered

	cancel()
	setTimer("none") // LoopCancel: the loop leaves its select
	if err := <-runErr; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	mu.Lock()
	trace = append(trace, watchRecord(delivered, 0, "none", true))
	mu.Unlock()

	start := watchRecord(nil, 0, "none", false)
	tlatrace.Write(t, traceDir, "WatchTraces", tlatrace.Traces{
		Accepted: [][]tlatrace.Record{trace},
		Rejected: [][]tlatrace.Record{
			// Two callbacks never run at once.
			{
				start, watchRecord(nil, 0, "armed", false), watchRecord(nil, 0, "expired", false),
				watchRecord(nil, 1, "expired", false), watchRecord(nil, 2, "expired", false),
			},
			// A burst is delivered only by a callback that ran for it.
			{start, watchRecord(nil, 0, "armed", false), watchRecord([]string{"ev1"}, 0, "armed", false)},
			// Run does not return while a callback is still running.
			{
				start, watchRecord(nil, 0, "armed", false), watchRecord(nil, 0, "expired", false),
				watchRecord(nil, 1, "expired", false), watchRecord(nil, 1, "none", false), watchRecord(nil, 1, "none", true),
			},
		},
	})
}
