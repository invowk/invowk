// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	manualDebounceScheduler struct {
		mu    sync.Mutex
		timer *manualDebounceTimer
	}

	manualDebounceTimer struct {
		mu         sync.Mutex
		fire       func()
		resetCount int
	}
)

func TestConfigValidateMutationContracts(t *testing.T) {
	t.Parallel()

	err := Config{
		Patterns: []invowkfile.GlobPattern{""},
		Ignore:   []invowkfile.GlobPattern{""},
		BaseDir:  types.FilesystemPath("   "),
	}.Validate()

	configErr := requireWatchMutationErrorAs[*InvalidWatchConfigError](t, err)
	if !errors.Is(err, ErrInvalidWatchConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidWatchConfig wrapper", err)
	}
	if got, want := configErr.Error(), "invalid watch config: 3 field errors"; got != want {
		t.Fatalf("InvalidWatchConfigError.Error() = %q, want %q", got, want)
	}
	if len(configErr.FieldErrors) != 3 {
		t.Fatalf("FieldErrors length = %d, want 3: %v", len(configErr.FieldErrors), configErr.FieldErrors)
	}

	requireWatchMutationFieldError(
		t,
		configErr.FieldErrors[0],
		invowkfile.ErrInvalidGlobPattern,
		"patterns[0]:",
	)
	requireWatchMutationFieldError(
		t,
		configErr.FieldErrors[1],
		invowkfile.ErrInvalidGlobPattern,
		"ignore[0]:",
	)
	requireWatchMutationFieldError(t, configErr.FieldErrors[2], types.ErrInvalidFilesystemPath, "")
}

func TestNewWithBackendMutationContracts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zeroDebounceBackend := newFakeWatcherBackend()
	zeroDebounce, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir),
		Ignore:  []invowkfile.GlobPattern{"custom/**"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	}, zeroDebounceBackend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() zero debounce error = %v", err)
	}
	if zeroDebounce.debounce != defaultDebounce {
		t.Fatalf("zero debounce = %v, want default %v", zeroDebounce.debounce, defaultDebounce)
	}
	if !slices.Contains(zeroDebounce.ignores, invowkfile.GlobPattern("custom/**")) {
		t.Fatalf("ignores = %v, want custom ignore copied", zeroDebounce.ignores)
	}
	if !slices.Contains(zeroDebounce.ignores, defaultIgnores[0]) {
		t.Fatalf("ignores = %v, want default ignores copied", zeroDebounce.ignores)
	}

	positiveDebounceBackend := newFakeWatcherBackend()
	positiveDebounce, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: time.Nanosecond,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	}, positiveDebounceBackend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() positive debounce error = %v", err)
	}
	if positiveDebounce.debounce != time.Nanosecond {
		t.Fatalf("positive debounce = %v, want 1ns", positiveDebounce.debounce)
	}

	manyIgnores := make([]invowkfile.GlobPattern, len(defaultIgnores)+1)
	for i := range manyIgnores {
		manyIgnores[i] = invowkfile.GlobPattern("custom-" + string(rune('a'+i)))
	}
	manyIgnoreBackend := newFakeWatcherBackend()
	manyIgnoreWatcher, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir),
		Ignore:  manyIgnores,
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	}, manyIgnoreBackend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() many ignores error = %v", err)
	}
	if !slices.Contains(manyIgnoreWatcher.ignores, manyIgnores[len(manyIgnores)-1]) {
		t.Fatalf("ignores = %v, want last custom ignore %q", manyIgnoreWatcher.ignores, manyIgnores[len(manyIgnores)-1])
	}
}

func TestAddDirectoriesMutationContracts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	addErr := errors.New("add failed")
	backend := newFakeWatcherBackend()
	backend.addErr = addErr

	_, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	}, backend, types.FilesystemPath(dir))
	if err == nil {
		t.Fatal("newWithBackend() error = nil, want backend add error")
	}
	if !errors.Is(err, addErr) {
		t.Fatalf("newWithBackend() error = %v, want wrapped add error", err)
	}
	if !strings.Contains(err.Error(), "watch: walk directory tree:") ||
		!strings.Contains(err.Error(), "watch: add directory") {
		t.Fatalf("newWithBackend() error = %q, want walk and add context", err.Error())
	}
}

func TestWatcherHelperMutationContracts(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	w := &Watcher{
		cfg: Config{
			Patterns: []invowkfile.GlobPattern{"[", "**/*.go"},
		},
		ignores: []invowkfile.GlobPattern{"[", "**/node_modules/**", "**/*.tmp"},
		stderr:  stderr,
	}

	if !w.isIgnoredDir("node_modules") {
		t.Fatal("isIgnoredDir(node_modules) = false, want trailing-slash ignore match")
	}
	if !w.isIgnoredDir("src/node_modules") {
		t.Fatal("isIgnoredDir(src/node_modules) = false, want nested trailing-slash ignore match")
	}
	w.ignores = append(w.ignores, "build", "vendor/")
	if !w.isIgnoredDir("build") {
		t.Fatal("isIgnoredDir(build) = false, want plain directory ignore match")
	}
	if !w.isIgnoredDir("vendor") {
		t.Fatal("isIgnoredDir(vendor) = false, want trailing slash directory ignore match")
	}
	if !w.isIgnored("cache.tmp") {
		t.Fatal("isIgnored(cache.tmp) = false, want later ignore pattern to match")
	}
	if !w.matchesPatterns("cmd/main.go") {
		t.Fatal("matchesPatterns(cmd/main.go) = false, want later watch pattern to match")
	}

	stderrText := stderr.String()
	if !strings.Contains(stderrText, "watch: ignore pattern") {
		t.Fatalf("stderr = %q, want invalid ignore pattern diagnostic", stderrText)
	}
	if !strings.Contains(stderrText, "watch: pattern") {
		t.Fatalf("stderr = %q, want invalid watch pattern diagnostic", stderrText)
	}

	if err := validatePatterns([]invowkfile.GlobPattern{"**/*.go", "["}, "watch"); err == nil {
		t.Fatal("validatePatterns() error = nil, want second pattern syntax error")
	}
}

func TestWatcherSkipIfBusyMutationContracts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	backend := newFakeWatcherBackend()
	w, err := newWithBackend(Config{
		BaseDir:  types.FilesystemPath(dir),
		Debounce: time.Hour,
		Stdout:   &bytes.Buffer{},
		Stderr:   stderr,
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() error = %v", err)
	}

	var (
		mu            sync.Mutex
		calls         int
		firstStarted  = make(chan struct{})
		retryFinished = make(chan struct{})
		releaseFirst  = make(chan struct{})
	)
	w.cfg.OnChange = func(ctx context.Context, changed []string) error {
		if ctx == nil {
			t.Fatal("OnChange() context = nil, want caller context")
		}
		if len(changed) == 0 {
			t.Fatal("OnChange() changed = empty, want pending paths")
		}

		mu.Lock()
		calls++
		call := calls
		mu.Unlock()

		if call == 1 {
			close(firstStarted)
			<-releaseFirst
			return nil
		}
		close(retryFinished)
		return nil
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

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "first.go"), Op: fsnotify.Write}
	timer := scheduler.requireTimer(t)
	firstDone := timer.fireAsync()

	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first callback")
	}

	backend.events <- fsnotify.Event{Name: filepath.Join(dir, "second.go"), Op: fsnotify.Write}
	timer.waitForResetCount(t, 1)
	skipDone := timer.fireAsync()
	timer.waitForResetCount(t, 2)
	if !strings.Contains(stderr.String(), "watch: skipping re-execution") {
		t.Fatalf("stderr = %q, want skip-if-busy diagnostic", stderr.String())
	}

	close(releaseFirst)
	waitForManualFire(t, firstDone)
	waitForManualFire(t, skipDone)
	retryDone := timer.fireAsync()
	select {
	case <-retryFinished:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retry callback")
	}
	waitForManualFire(t, retryDone)

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if !backend.closed {
		t.Fatal("backend was not closed")
	}
}

func TestMaybeAddDirMutationContracts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	childDir := filepath.Join(dir, "src")
	if err := os.Mkdir(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	childFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(childFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	stderr := &bytes.Buffer{}
	backend := newFakeWatcherBackend()
	w, err := newWithBackend(Config{
		BaseDir: types.FilesystemPath(dir),
		Stdout:  &bytes.Buffer{},
		Stderr:  stderr,
	}, backend, types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("newWithBackend() error = %v", err)
	}

	backend.added = nil
	w.maybeAddDir(childFile)
	if len(backend.added) != 0 {
		t.Fatalf("maybeAddDir(file) added = %v, want none", backend.added)
	}

	w.maybeAddDir(childDir)
	if !slices.Contains(backend.added, childDir) {
		t.Fatalf("maybeAddDir(dir) added = %v, want %q", backend.added, childDir)
	}

	backend.added = nil
	ignoredDir := filepath.Join(dir, "__pycache__")
	if err := os.Mkdir(ignoredDir, 0o755); err != nil {
		t.Fatalf("mkdir ignored dir: %v", err)
	}
	w.maybeAddDir(ignoredDir)
	if len(backend.added) != 0 {
		t.Fatalf("maybeAddDir(ignored dir) added = %v, want none", backend.added)
	}

	w.maybeAddDir(filepath.Join(dir, "missing"))
	if !strings.Contains(stderr.String(), "watch: cannot stat new path") {
		t.Fatalf("stderr = %q, want missing-path diagnostic", stderr.String())
	}
}

func requireWatchMutationErrorAs[T error](t *testing.T, err error) T {
	t.Helper()

	got, ok := errors.AsType[T](err)
	if !ok {
		var zero T
		t.Fatalf("error = %v, want %T", err, zero)
	}
	return got
}

func requireWatchMutationFieldError(t *testing.T, err, sentinel error, text string) {
	t.Helper()

	if !errors.Is(err, sentinel) {
		t.Fatalf("field error = %v, want sentinel %v", err, sentinel)
	}
	if text != "" && !strings.Contains(err.Error(), text) {
		t.Fatalf("field error = %q, want text %q", err.Error(), text)
	}
}

func newManualDebounceScheduler() *manualDebounceScheduler {
	return &manualDebounceScheduler{}
}

func (s *manualDebounceScheduler) Schedule(_ time.Duration, fire func()) debounceTimer {
	s.mu.Lock()
	defer s.mu.Unlock()

	timer := &manualDebounceTimer{fire: fire}
	s.timer = timer
	return timer
}

func (s *manualDebounceScheduler) requireTimer(t *testing.T) *manualDebounceTimer {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		timer := s.timer
		s.mu.Unlock()
		if timer != nil {
			return timer
		}
		if time.Now().After(deadline) {
			t.Fatal("manual debounce timer was not scheduled")
		}
		time.Sleep(time.Millisecond)
	}
}

func (t *manualDebounceTimer) Reset(time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resetCount++
	return false
}

func (t *manualDebounceTimer) Stop() bool {
	return false
}

func (t *manualDebounceTimer) fireAsync() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		t.fire()
	}()
	return done
}

func (t *manualDebounceTimer) waitForResetCount(tb *testing.T, want int) {
	tb.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		t.mu.Lock()
		got := t.resetCount
		t.mu.Unlock()
		if got >= want {
			return
		}
		if time.Now().After(deadline) {
			tb.Fatalf("manual timer reset count = %d, want at least %d", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForManualFire(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manual timer fire did not finish")
	}
}
