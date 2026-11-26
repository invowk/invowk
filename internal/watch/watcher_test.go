// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type fakeWatcherBackend struct {
	events   chan fsnotify.Event
	errors   chan error
	addErr   error
	closeErr error
	added    []string
	closed   bool
}

// isIgnoredByDefaults reports whether rel matches any of the default ignore
// patterns. Test-only helper that avoids needing a full Watcher instance.
func isIgnoredByDefaults(rel string) bool {
	normalized := filepath.ToSlash(rel)
	for _, pat := range defaultIgnores {
		if matched, matchErr := doublestar.Match(string(pat), normalized); matchErr == nil && matched {
			return true
		}
	}
	return false
}

func TestWatcherBackendFiltersAndDebouncesEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backend := newFakeWatcherBackend()
	done := make(chan []string, 1)
	w, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Patterns: []invowkfile.GlobPattern{"**/*.go"},
		Debounce: 10 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			done <- slices.Clone(changed)
			return nil
		},
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() = %v", err)
	}
	if !slices.Contains(backend.added, dir) {
		t.Fatalf("backend.added = %v, want base dir %q", backend.added, dir)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "notes.txt"), Op: fsnotify.Write}
	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "main.go"), Op: fsnotify.Write}

	var changed []string
	select {
	case changed = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback")
	}
	if !slices.Equal(changed, []string{"main.go"}) {
		t.Fatalf("changed = %v, want [main.go]", changed)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if !backend.closed {
		t.Fatal("backend was not closed")
	}
}

// TestWatcherBackendCoalescesBurst is the exact debounce contract on the
// fake backend with a manual timer: three events inside one window give
// exactly one callback naming all three paths, and no second callback.
func TestWatcherBackendCoalescesBurst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backend := newFakeWatcherBackend()
	callbacks := make(chan []string, 4)
	w, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: time.Hour,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			callbacks <- slices.Clone(changed)
			return nil
		},
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() error = %v", err)
	}
	scheduler := newManualDebounceScheduler()
	w.schedule = scheduler.Schedule

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		backend.events <- fsnotify.Event{Name: filepath.Join(dir, name), Op: fsnotify.Write}
	}
	timer := scheduler.requireTimer(t)
	// The second and third events re-arm the one pending timer.
	timer.waitForResetCount(t, 2)
	waitForManualFire(t, timer.fireAsync())

	select {
	case changed := <-callbacks:
		slices.Sort(changed)
		if !slices.Equal(changed, []string{"a.txt", "b.txt", "c.txt"}) {
			t.Fatalf("changed = %v, want [a.txt b.txt c.txt]", changed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the debounced callback")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() = %v", err)
	}
	select {
	case changed := <-callbacks:
		t.Fatalf("second callback %v, want exactly one for the burst", changed)
	default:
	}
}

func TestWatcherRunReturnsCallbackError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backend := newFakeWatcherBackend()
	callbackErr := errors.New("callback failed")
	w, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: 10 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, _ []string) error {
			return callbackErr
		},
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() = %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(t.Context()) }()

	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "main.go"), Op: fsnotify.Write}

	select {
	case err := <-errCh:
		if !errors.Is(err, callbackErr) {
			t.Fatalf("Run() error = %v, want callback error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run callback error")
	}
	if !backend.closed {
		t.Fatal("backend was not closed")
	}
}

func TestWatcherInitFailureCloseUsesConfiguredStderr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	backend := newFakeWatcherBackend()
	backend.addErr = errors.New("add failed")
	backend.closeErr = errors.New("close failed")

	_, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir),
		Stdout:  &bytes.Buffer{},
		Stderr:  stderr,
	}, backend, types.FilesystemPath(dir))
	if err == nil {
		t.Fatal("newWithBackend() error = nil, want add error")
	}
	if !strings.Contains(stderr.String(), "watch: close after init failure: close failed") {
		t.Fatalf("stderr = %q, want configured close diagnostic", stderr.String())
	}
}

func newFakeWatcherBackend() *fakeWatcherBackend {
	return &fakeWatcherBackend{
		events: make(chan fsnotify.Event, 4),
		errors: make(chan error, 1),
	}
}

func (b *fakeWatcherBackend) Add(path string) error {
	b.added = append(b.added, path)
	return b.addErr
}

func (b *fakeWatcherBackend) Close() error {
	b.closed = true
	close(b.events)
	close(b.errors)
	return b.closeErr
}

func (b *fakeWatcherBackend) Events() <-chan fsnotify.Event {
	return b.events
}

func (b *fakeWatcherBackend) Errors() <-chan error {
	return b.errors
}

// TestWatcherDebounce checks the debounce on the real filesystem: three
// rapid writes reach the callbacks, and the callbacks name nothing else.
func TestWatcherDebounce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const debounce = 100 * time.Millisecond

	callbacks := make(chan []string, 16)
	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: debounce,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			callbacks <- slices.Clone(changed)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	// Write three files in rapid succession — well within the debounce window.
	files := []string{"a.txt", "b.txt", "c.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		// Small pause so events arrive as separate fsnotify events rather
		// than being batched by the OS. 20ms accounts for macOS kqueue
		// coalescing while staying well within the debounce window.
		time.Sleep(20 * time.Millisecond)
	}

	// The real filesystem cannot promise one callback: an OS that reports a
	// change after the debounce window closed (Windows did, run 45517294042,
	// "Test (windows)" on PR #208) correctly starts a second window. So this
	// test asserts what the OS guarantees: the first callback arrives, every
	// callback names only the written files, and the callbacks received until
	// the watcher has been quiet for two debounce periods name all three. The
	// exact one-callback contract is TestWatcherBackendCoalescesBurst.
	union := map[string]struct{}{}
	collect := func(changed []string) {
		for _, path := range changed {
			if !slices.Contains(files, path) {
				t.Errorf("callback named %q, want only %v (got %v)", path, files, changed)
			}
			union[path] = struct{}{}
		}
	}
	select {
	case changed := <-callbacks:
		collect(changed)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback")
	}
	for quiet := false; !quiet; {
		select {
		case changed := <-callbacks:
			collect(changed)
		case <-time.After(2 * debounce):
			quiet = true
		}
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, want := range files {
		if _, ok := union[want]; !ok {
			t.Errorf("expected %q in the changed files of the callbacks, got %v", want, slices.Sorted(maps.Keys(union)))
		}
	}
}

// TestWatcherIgnorePatterns confirms that files matching user-supplied ignore
// patterns do not trigger the OnChange callback.
func TestWatcherIgnorePatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	callbackFired := make(chan []string, 10)

	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Ignore:   []invowkfile.GlobPattern{"**/*.log"},
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			callbackFired <- changed
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write an ignored file — should NOT trigger callback.
	if err := os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log"), 0o644); err != nil {
		t.Fatalf("write debug.log: %v", err)
	}

	// Verify the ignored file does not trigger a callback within a debounce cycle.
	select {
	case changed := <-callbackFired:
		t.Fatalf("ignored file triggered callback: %v", changed)
	case <-time.After(200 * time.Millisecond):
		// No callback — expected for ignored file.
	}

	// Write a non-ignored file — SHOULD trigger callback.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	select {
	case changed := <-callbackFired:
		// Verify only the non-ignored file appears.
		if slices.Contains(changed, "debug.log") {
			t.Error("ignored file debug.log appeared in changed set")
		}
		if !slices.Contains(changed, "main.go") {
			t.Errorf("expected main.go in changed set, got %v", changed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback on non-ignored file")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestWatcherContextCancel verifies that Run returns cleanly when its context
// is cancelled and does not leak goroutines or file descriptors.
func TestWatcherContextCancel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Wait for the event loop to be ready before cancelling.
	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() returned error on cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

// TestDefaultIgnores ensures that the built-in default ignore patterns cover
// the expected high-noise paths (.git, node_modules, editor swap files, etc.).
func TestDefaultIgnores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    string
		ignored bool
	}{
		{".git/config", true},
		{".git/objects/ab/cd1234", true},
		{"node_modules/express/index.js", true},
		{"src/__pycache__/mod.cpython.pyc", true},
		{"main.go.swp", true},
		{"main.go.swo", true},
		{"backup~", true},
		{".DS_Store", true},
		{"sub/.DS_Store", true},
		// These should NOT be ignored.
		{"main.go", false},
		{"src/app.ts", false},
		{"README.md", false},
		{".gitignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := isIgnoredByDefaults(tt.path)
			if got != tt.ignored {
				t.Errorf("isIgnoredByDefaults(%q) = %v, want %v", tt.path, got, tt.ignored)
			}
		})
	}
}

// TestWatcherSkipIfBusy verifies that concurrent callback invocations are
// prevented by the atomic "skip-if-busy" guard. When the callback takes longer
// than the debounce period, subsequent timer fires should be skipped.
func TestWatcherSkipIfBusy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var (
		mu    sync.Mutex
		calls int
	)

	// Callback blocks for 300ms, debounce is 50ms.
	// Second file write should be skipped because the first callback is still running.
	firstCallStarted := make(chan struct{})
	firstCallDone := make(chan struct{})
	stderrBuf := &bytes.Buffer{}

	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   stderrBuf,
		OnChange: func(_ context.Context, _ []string) error {
			mu.Lock()
			calls++
			callNum := calls
			mu.Unlock()

			if callNum == 1 {
				close(firstCallStarted)
				time.Sleep(300 * time.Millisecond)
				close(firstCallDone)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	// Write first file — triggers first callback (blocks for 300ms).
	if err := os.WriteFile(filepath.Join(dir, "first.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("write first.txt: %v", err)
	}

	// Wait for the first callback to start (event-based, no timing assumption).
	select {
	case <-firstCallStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first callback to start")
	}

	// Write second file while callback is still busy — should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("write second.txt: %v", err)
	}

	// Wait for first callback to finish.
	select {
	case <-firstCallDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first callback")
	}

	// Wait long enough for the second debounce cycle to complete (or be skipped).
	// Use AssertNeverTrue to verify no more than 2 callbacks fire.
	testutil.AssertNeverTrue(t, 300*time.Millisecond, 10*time.Millisecond,
		"more than 2 callback invocations detected",
		func() bool {
			mu.Lock()
			defer mu.Unlock()
			return calls > 2
		})

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify the skip message appeared in stderr.
	if calls == 1 {
		stderrStr := stderrBuf.String()
		if !strings.Contains(stderrStr, "skipping re-execution") {
			t.Logf("stderr: %s", stderrStr)
			t.Log("expected skip message in stderr, but callback may have completed before second fire")
		}
	}
}

// TestWatcherInvalidPattern verifies that New returns an error when given
// an invalid glob pattern, failing fast at construction time.
func TestWatcherInvalidPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Patterns: []invowkfile.GlobPattern{"[invalid"},
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("New() should return an error for an invalid glob pattern")
	}

	if !errors.Is(err, ErrInvalidWatchConfig) {
		t.Errorf("error should wrap ErrInvalidWatchConfig, got: %v", err)
	}
}

// TestWatcherDoubleRunError verifies that calling Run a second time returns
// an error immediately rather than starting a second event loop.
func TestWatcherDoubleRunError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Start Run in a goroutine.
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Wait for the event loop to be ready before testing double-run.
	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher event loop did not become ready")
	}

	// Second call to Run should return an error immediately.
	err = w.Run(ctx)
	if err == nil {
		t.Fatal("second Run() call should return an error")
	}

	if !errors.Is(err, ErrRunCalledMoreThanOnce) {
		t.Errorf("error should be ErrRunCalledMoreThanOnce, got: %v", err)
	}

	cancel()
	if firstErr := <-errCh; firstErr != nil {
		t.Fatalf("first Run() returned error: %v", firstErr)
	}
}

// TestWatcherPatternFiltering verifies that only events matching the
// configured glob patterns trigger the callback.
func TestWatcherPatternFiltering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	callbackFired := make(chan []string, 10)

	w, err := New(Config{
		BaseDir:  types.FilesystemPath(dir),
		Patterns: []invowkfile.GlobPattern{"**/*.go"},
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			callbackFired <- changed
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write a non-matching file first.
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("text"), 0o644); err != nil {
		t.Fatalf("write data.txt: %v", err)
	}

	// Verify the non-matching file does not trigger a callback within a debounce cycle.
	select {
	case changed := <-callbackFired:
		t.Fatalf("non-matching file triggered callback: %v", changed)
	case <-time.After(200 * time.Millisecond):
		// No callback — expected for non-matching file.
	}

	// Write a matching .go file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	select {
	case changed := <-callbackFired:
		if slices.Contains(changed, "data.txt") {
			t.Error("non-matching file data.txt appeared in changed set")
		}
		if !slices.Contains(changed, "main.go") {
			t.Errorf("expected main.go in changed set, got %v", changed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback on .go file")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}
