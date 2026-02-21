// SPDX-License-Identifier: MPL-2.0

// Package watch provides file-watching with debounced re-execution.
//
// It monitors filesystem paths matching glob patterns and invokes a callback
// after a configurable debounce period. Events within the debounce window are
// coalesced so the callback fires once with the full set of changed paths.
package watch

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
)

// defaultDebounce is the delay before firing the onChange callback after the
// last filesystem event. This allows rapid successive events (e.g., an editor
// writing then renaming a temp file) to coalesce into a single callback.
const defaultDebounce = 500 * time.Millisecond

// defaultIgnores lists path patterns that are always excluded from watching,
// regardless of user-supplied ignore patterns. These cover VCS metadata,
// dependency caches, editor swap files, and OS metadata files that generate
// high-frequency noise.
var defaultIgnores = []string{
	"**/.git/**",
	"**/node_modules/**",
	"**/__pycache__/**",
	"**/*.swp",
	"**/*.swo",
	"**/*~",
	"**/.DS_Store",
}

type (
	// Config holds the parameters for a Watcher.
	Config struct {
		// Patterns are doublestar-compatible glob patterns (e.g., "**/*.go")
		// that select which files trigger callbacks. An empty slice watches all
		// non-ignored files.
		Patterns []string

		// Ignore are additional doublestar-compatible glob patterns for paths
		// that should never trigger callbacks. These are merged with the
		// built-in default ignores.
		Ignore []string

		// Debounce is the quiet period after the last event before the callback
		// fires. Zero or negative values fall back to defaultDebounce.
		Debounce time.Duration

		// ClearScreen controls whether the terminal is cleared before each
		// callback invocation by writing ANSI escape sequences to Stdout.
		// No terminal detection is performed; callers should ensure Stdout
		// is a real terminal when enabling this option.
		ClearScreen bool

		// BaseDir is the root directory to watch. All patterns are resolved
		// relative to this path. An empty value defaults to the current working
		// directory.
		BaseDir string

		// OnChange is called after the debounce window closes with the
		// deduplicated list of changed file paths (relative to BaseDir). A nil
		// callback is a no-op.
		OnChange func(ctx context.Context, changed []string) error

		// Stdout and Stderr are the output writers for informational and error
		// messages respectively. nil values default to os.Stdout / os.Stderr.
		// Must be goroutine-safe: writes may occur from timer goroutines
		// (via fire/OnChange) concurrent with the event loop.
		Stdout io.Writer
		Stderr io.Writer
	}

	// Watcher monitors filesystem paths and fires a debounced callback when
	// matching files change. Run must be called exactly once; calling it a
	// second time returns an error. Close releases the underlying fsnotify
	// resources and should be called if Run will not be called (e.g., the
	// caller encounters an error between New and Run).
	Watcher struct {
		cfg      Config
		fsw      *fsnotify.Watcher
		ignores  []string
		stdout   io.Writer
		stderr   io.Writer
		debounce time.Duration
		baseDir  string
		started  atomic.Bool
		closed   atomic.Bool
	}
)

func init() {
	// Validate default ignore patterns at startup. These are programmer-controlled
	// constants; a match error here is always a code bug, not a user error.
	for _, pat := range defaultIgnores {
		if _, err := doublestar.Match(pat, ""); err != nil {
			panic(fmt.Sprintf("BUG: invalid default ignore pattern %q: %v", pat, err))
		}
	}
}

// New creates a Watcher from the given Config. It resolves BaseDir to an
// absolute path, initialises the underlying fsnotify watcher, and registers
// all non-ignored directories under BaseDir for monitoring.
func New(cfg Config) (*Watcher, error) {
	baseDir := cfg.BaseDir
	if baseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("watch: determine working directory: %w", err)
		}
		baseDir = wd
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("watch: resolve base directory: %w", err)
	}

	// Validate all patterns eagerly so invalid globs fail at construction
	// time rather than silently failing to match at runtime.
	if patErr := validatePatterns(cfg.Patterns, "watch"); patErr != nil {
		return nil, patErr
	}
	if patErr := validatePatterns(cfg.Ignore, "ignore"); patErr != nil {
		return nil, patErr
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watch: create fsnotify watcher: %w", err)
	}

	debounce := cfg.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}

	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Merge user ignores with built-in defaults.
	ignores := make([]string, 0, len(defaultIgnores)+len(cfg.Ignore))
	ignores = append(ignores, defaultIgnores...)
	ignores = append(ignores, cfg.Ignore...)

	w := &Watcher{
		cfg:      cfg,
		fsw:      fsw,
		ignores:  ignores,
		stdout:   stdout,
		stderr:   stderr,
		debounce: debounce,
		baseDir:  absBase,
	}

	if err := w.addDirectories(); err != nil {
		if closeErr := fsw.Close(); closeErr != nil {
			fmt.Fprintf(stderr, "watch: close after init failure: %v\n", closeErr)
		}
		return nil, err
	}

	return w, nil
}

// Close releases the underlying fsnotify resources without starting the
// event loop. Use this if Run will not be called (e.g., the caller
// encounters an error between New and Run). Close is idempotent; calling
// it after Run has already cleaned up is a no-op.
func (w *Watcher) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	return w.fsw.Close()
}

// Run blocks until ctx is cancelled, processing filesystem events and
// dispatching debounced callbacks. It returns nil on clean context
// cancellation and propagates any fatal watcher errors. Run must be
// called exactly once; a second call returns an error immediately.
func (w *Watcher) Run(ctx context.Context) error {
	if !w.started.CompareAndSwap(false, true) {
		return fmt.Errorf("watch: Run called more than once")
	}
	// Mark as closed so a subsequent Close() is a no-op — Run owns
	// the fsnotify lifecycle from this point via its defer block.
	w.closed.Store(true)

	var (
		mu      sync.Mutex
		pending = make(map[string]struct{})
		timer   *time.Timer
		running atomic.Bool
		wg      sync.WaitGroup
	)

	// fireBody drains the pending set and invokes the OnChange callback.
	// It may be scheduled by time.AfterFunc after the context is cancelled,
	// so check ctx.Err() as a best-effort guard. A narrow TOCTOU window
	// remains between the check and OnChange invocation; this is acceptable
	// because the callback receives ctx and is expected to check cancellation
	// itself — attempting to eliminate this window via locking would risk
	// deadlock with the event loop.
	//
	// Uses atomic "skip-if-busy" guard to prevent concurrent callback
	// invocations when the command takes longer than the debounce period.
	//
	// IMPORTANT: wg.Add(1) is called at the scheduling site (before
	// time.AfterFunc / timer.Reset), NOT inside this function. This
	// eliminates a race where wg.Wait() in cleanup could see count=0
	// before the timer goroutine increments it. The caller (fireWrapped)
	// is responsible for defer wg.Done().
	fireBody := func() {
		if ctx.Err() != nil {
			return
		}
		if !running.CompareAndSwap(false, true) {
			// Schedule a retry so pending events are not permanently lost.
			// Without this, if no further filesystem events arrive, the
			// accumulated pending set would be silently discarded.
			// Stderr write is under the mutex to prevent interleaved output
			// from concurrent timer goroutines on non-goroutine-safe writers.
			mu.Lock()
			fmt.Fprintf(w.stderr, "watch: skipping re-execution (previous run still in progress)\n")
			if timer != nil {
				// We're inside a fire() invocation — the timer has expired.
				// Only pre-increment WaitGroup when Reset returns false
				// (timer was expired/stopped, new goroutine will be created).
				// When Reset returns true, the event loop already Reset the
				// timer and accounted for the WaitGroup increment.
				if !timer.Reset(w.debounce) {
					wg.Add(1)
				}
			}
			mu.Unlock()
			return
		}
		defer running.Store(false)

		mu.Lock()
		if len(pending) == 0 {
			mu.Unlock()
			return
		}
		changed := slices.Collect(maps.Keys(pending))
		clear(pending)
		mu.Unlock()

		if w.cfg.ClearScreen {
			// ANSI escape: clear screen and move cursor to top-left.
			fmt.Fprint(w.stdout, "\033[2J\033[H")
		}

		if w.cfg.OnChange != nil {
			if err := w.cfg.OnChange(ctx, changed); err != nil {
				fmt.Fprintf(w.stderr, "watch: callback error: %v\n", err)
			}
		}
	}

	// fireWrapped wraps fireBody with WaitGroup tracking.
	fireWrapped := func() {
		defer wg.Done()
		fireBody()
	}

	// Stop the debounce timer and wait for any in-flight fire() callback
	// to complete before closing the fsnotify watcher.
	defer func() {
		mu.Lock()
		localTimer := timer
		// Nil out timer to prevent fire's skip-if-busy retry from
		// re-arming it after we've stopped it.
		timer = nil
		if localTimer != nil {
			if localTimer.Stop() {
				// Timer was still active; the goroutine was never created.
				// Compensate for the wg.Add(1) from the scheduling site.
				wg.Done()
			}
			// AfterFunc timers have a nil C channel — no drain needed.
		}
		mu.Unlock()
		// Wait for any in-flight fire() goroutine to finish before closing
		// fsnotify. Without this, OnChange could race with resource cleanup.
		wg.Wait()
		if closeErr := w.fsw.Close(); closeErr != nil {
			fmt.Fprintf(w.stderr, "watch: close fsnotify: %v\n", closeErr)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case evt, ok := <-w.fsw.Events:
			if !ok {
				return fmt.Errorf("watch: fsnotify event channel closed unexpectedly")
			}

			rel, err := filepath.Rel(w.baseDir, evt.Name)
			if err != nil {
				rel = evt.Name
			}

			if w.isIgnored(rel) {
				continue
			}

			if !w.matchesPatterns(rel) {
				continue
			}

			// Auto-add newly created directories so recursive watches
			// extend to directories created after startup.
			if evt.Has(fsnotify.Create) {
				w.maybeAddDir(evt.Name)
			}

			mu.Lock()
			pending[rel] = struct{}{}
			if timer == nil {
				wg.Add(1)
				timer = time.AfterFunc(w.debounce, fireWrapped)
			} else if !timer.Reset(w.debounce) {
				// Timer had already expired: the old fire goroutine was
				// started (and will wg.Done() independently). Reset
				// re-arms the timer; a new goroutine will be created.
				// If Reset returned true: timer was still pending — the same
				// fire goroutine (with its existing wg.Add) will run.
				wg.Add(1)
			}
			mu.Unlock()

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return fmt.Errorf("watch: fsnotify error channel closed unexpectedly")
			}
			// Classify the error: resource exhaustion (inotify limit, file
			// descriptor limits) indicates the watcher is fundamentally broken.
			// isFatalFsnotifyError is platform-specific (see watcher_fatal_*.go).
			if isFatalFsnotifyError(err) {
				return fmt.Errorf("watch: fatal fsnotify error: %w", err)
			}
			fmt.Fprintf(w.stderr, "watch: fsnotify error: %v\n", err)
		}
	}
}

// addDirectories walks BaseDir (including BaseDir itself) and adds every
// non-ignored directory to the fsnotify watcher. All directories are registered
// regardless of watch patterns; pattern filtering is applied when events arrive
// (see matchesPatterns).
func (w *Watcher) addDirectories() error {
	walkErr := filepath.WalkDir(w.baseDir, func(path string, d os.DirEntry, walkDirErr error) error {
		if walkDirErr != nil {
			// Best-effort: skip paths we cannot access rather than aborting the
			// entire walk. Permission errors on individual dirs are common
			// (e.g., .git/objects/pack) and should not prevent watching.
			// Log to stderr so users know which paths are not being watched.
			fmt.Fprintf(w.stderr, "watch: skipping inaccessible path %q: %v\n", path, walkDirErr)
			// Return SkipDir for directories to prevent descending into children,
			// which would produce a cascade of identical permission errors.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil //nolint:nilerr // intentional skip of inaccessible files
		}
		if !d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(w.baseDir, path)
		if relErr != nil {
			fmt.Fprintf(w.stderr, "watch: skipping path %q (cannot make relative): %v\n", path, relErr)
			return nil //nolint:nilerr // skip paths that cannot be made relative
		}

		// Skip ignored directories entirely to avoid descending into them.
		if w.isIgnoredDir(rel) {
			return filepath.SkipDir
		}

		if addErr := w.fsw.Add(path); addErr != nil {
			return fmt.Errorf("watch: add directory %q: %w", path, addErr)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("watch: walk directory tree: %w", walkErr)
	}
	return nil
}

// maybeAddDir adds path to the fsnotify watcher if it is a directory and is
// not ignored. This enables automatic monitoring of directories created after
// the initial walk.
func (w *Watcher) maybeAddDir(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(w.stderr, "watch: cannot stat new path %q: %v\n", path, err)
		return
	}
	if !info.IsDir() {
		return
	}

	rel, err := filepath.Rel(w.baseDir, path)
	if err != nil {
		fmt.Fprintf(w.stderr, "watch: skipping new path %q (cannot make relative): %v\n", path, err)
		return
	}

	if w.isIgnoredDir(rel) {
		return
	}

	if addErr := w.fsw.Add(path); addErr != nil {
		fmt.Fprintf(w.stderr, "watch: add new directory %q: %v\n", path, addErr)
	}
}

// isIgnored returns true if the given path (relative to BaseDir) matches any
// ignore pattern. Match errors are logged and the pattern is skipped (treated
// as non-matching) so that a single bad pattern doesn't disable all ignoring.
func (w *Watcher) isIgnored(rel string) bool {
	// Normalise to forward slashes for consistent glob matching.
	normalized := filepath.ToSlash(rel)
	for _, pat := range w.ignores {
		matched, matchErr := doublestar.Match(pat, normalized)
		if matchErr != nil {
			fmt.Fprintf(w.stderr, "watch: ignore pattern %q match error for %q: %v\n", pat, normalized, matchErr)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// isIgnoredDir returns true if a directory (relative path) matches any ignore
// pattern. Checks both the plain path and with a trailing slash, since
// doublestar patterns like "**/node_modules/**" need the trailing slash to
// match directory entries.
func (w *Watcher) isIgnoredDir(rel string) bool {
	return w.isIgnored(rel) || w.isIgnored(rel+"/")
}

// matchesPatterns returns true if the given path (relative to BaseDir) matches
// at least one of the configured watch patterns. When no patterns are
// configured, all paths match. Match errors are logged and the pattern is
// skipped (treated as non-matching).
func (w *Watcher) matchesPatterns(rel string) bool {
	if len(w.cfg.Patterns) == 0 {
		return true
	}
	normalized := filepath.ToSlash(rel)
	for _, pat := range w.cfg.Patterns {
		matched, matchErr := doublestar.Match(pat, normalized)
		if matchErr != nil {
			fmt.Fprintf(w.stderr, "watch: pattern %q match error for %q: %v\n", pat, normalized, matchErr)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// DefaultIgnores returns a copy of the built-in ignore patterns. This is
// useful for tests and tooling that need to verify the default behaviour.
func DefaultIgnores() []string {
	return slices.Clone(defaultIgnores)
}

// validatePatterns checks that every pattern in the slice is a valid doublestar
// glob. The label (e.g., "watch" or "ignore") is used in error messages.
func validatePatterns(patterns []string, label string) error {
	for _, pat := range patterns {
		if _, err := doublestar.Match(pat, ""); err != nil {
			return fmt.Errorf("watch: invalid %s pattern %q: %w", label, pat, err)
		}
	}
	return nil
}
