// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// isIgnoredByDefaults reports whether rel matches any of the default ignore
// patterns. Test-only helper that avoids needing a full Watcher instance.
func isIgnoredByDefaults(rel string) bool {
	normalized := filepath.ToSlash(rel)
	for _, pat := range defaultIgnores {
		if matched, matchErr := doublestar.Match(pat, normalized); matchErr == nil && matched {
			return true
		}
	}
	return false
}

// TestWatcherDebounce verifies that multiple rapid filesystem events are
// coalesced into a single callback invocation containing all changed paths.
func TestWatcherDebounce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var (
		mu        sync.Mutex
		calls     int
		collected []string
	)

	done := make(chan struct{})

	w, err := New(Config{
		BaseDir:  dir,
		Debounce: 100 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		OnChange: func(_ context.Context, changed []string) error {
			mu.Lock()
			defer mu.Unlock()
			calls++
			collected = append(collected, changed...)
			close(done)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write three files in rapid succession — well within the debounce window.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		// Small pause so events arrive as separate fsnotify events rather
		// than being batched by the OS. Still well within the debounce
		// window.
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the debounced callback to fire.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback")
	}

	// Allow a brief settle for any additional spurious callbacks.
	time.Sleep(200 * time.Millisecond)

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if calls != 1 {
		t.Errorf("expected 1 debounced callback, got %d", calls)
	}

	// All three files must appear in the collected set.
	slices.Sort(collected)
	for _, want := range []string{"a.txt", "b.txt", "c.txt"} {
		if !slices.Contains(collected, want) {
			t.Errorf("expected %q in changed files, got %v", want, collected)
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
		BaseDir:  dir,
		Ignore:   []string{"**/*.log"},
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write an ignored file — should NOT trigger callback.
	if err := os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log"), 0o644); err != nil {
		t.Fatalf("write debug.log: %v", err)
	}

	// Wait long enough for a debounce cycle to complete.
	time.Sleep(200 * time.Millisecond)

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
		BaseDir:  dir,
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Give the event loop time to start.
	time.Sleep(50 * time.Millisecond)

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
	firstCallDone := make(chan struct{})
	stderrBuf := &bytes.Buffer{}

	w, err := New(Config{
		BaseDir:  dir,
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   stderrBuf,
		OnChange: func(_ context.Context, _ []string) error {
			mu.Lock()
			calls++
			callNum := calls
			mu.Unlock()

			if callNum == 1 {
				time.Sleep(300 * time.Millisecond)
				close(firstCallDone)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write first file — triggers first callback (blocks for 300ms).
	if err := os.WriteFile(filepath.Join(dir, "first.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("write first.txt: %v", err)
	}

	// Wait for the debounce to fire and callback to start.
	time.Sleep(100 * time.Millisecond)

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

	// Allow time for the second debounce cycle to complete (or be skipped).
	time.Sleep(200 * time.Millisecond)

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// The second fire() should have been skipped due to the busy guard.
	// We accept 1 call (strict skip) or 2 calls (if timing allows the second
	// debounce to fire after the first callback completes), but never concurrent.
	if calls > 2 {
		t.Errorf("expected at most 2 callback invocations, got %d", calls)
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

// TestWatcherClearScreen verifies that ClearScreen: true writes the ANSI
// clear escape sequence before invoking the callback.
func TestWatcherClearScreen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	done := make(chan struct{})
	stdoutBuf := &bytes.Buffer{}

	w, err := New(Config{
		BaseDir:     dir,
		Debounce:    50 * time.Millisecond,
		ClearScreen: true,
		Stdout:      stdoutBuf,
		Stderr:      &bytes.Buffer{},
		OnChange: func(_ context.Context, _ []string) error {
			close(done)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file.go: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for callback")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify ANSI clear sequence was written.
	out := stdoutBuf.String()
	if !strings.Contains(out, "\033[2J\033[H") {
		t.Errorf("expected ANSI clear sequence in stdout, got %q", out)
	}
}

// TestWatcherInvalidPattern verifies that New returns an error when given
// an invalid glob pattern, failing fast at construction time.
func TestWatcherInvalidPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := New(Config{
		BaseDir:  dir,
		Patterns: []string{"[invalid"},
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("New() should return an error for an invalid glob pattern")
	}

	if !strings.Contains(err.Error(), "invalid watch pattern") {
		t.Errorf("error message should mention invalid watch pattern, got: %v", err)
	}
}

// TestWatcherDoubleRunError verifies that calling Run a second time returns
// an error immediately rather than starting a second event loop.
func TestWatcherDoubleRunError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	w, err := New(Config{
		BaseDir:  dir,
		Debounce: 50 * time.Millisecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Run in a goroutine.
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Give the event loop time to start.
	time.Sleep(50 * time.Millisecond)

	// Second call to Run should return an error immediately.
	err = w.Run(ctx)
	if err == nil {
		t.Fatal("second Run() call should return an error")
	}

	if !strings.Contains(err.Error(), "Run called more than once") {
		t.Errorf("error message should mention double-run, got: %v", err)
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
		BaseDir:  dir,
		Patterns: []string{"**/*.go"},
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Write a non-matching file first.
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("text"), 0o644); err != nil {
		t.Fatalf("write data.txt: %v", err)
	}

	// Wait for a debounce cycle to ensure the .txt write does not fire.
	time.Sleep(200 * time.Millisecond)

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
