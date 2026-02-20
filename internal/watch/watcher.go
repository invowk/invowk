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
		Stdout io.Writer
		Stderr io.Writer
	}

	// Watcher monitors filesystem paths and fires a debounced callback when
	// matching files change. Run must be called exactly once; calling it a
	// second time returns an error.
	Watcher struct {
		cfg      Config
		fsw      *fsnotify.Watcher
		ignores  []string
		stdout   io.Writer
		stderr   io.Writer
		debounce time.Duration
		baseDir  string
		started  atomic.Bool
	}
)

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

	// Validate all patterns eagerly so invalid globs fail at construction
	// time rather than silently failing to match at runtime.
	if err := validatePatterns(cfg.Patterns, "watch"); err != nil {
		fsw.Close() //nolint:errcheck // best-effort cleanup
		return nil, err
	}
	if err := validatePatterns(cfg.Ignore, "ignore"); err != nil {
		fsw.Close() //nolint:errcheck // best-effort cleanup
		return nil, err
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

// Run blocks until ctx is cancelled, processing filesystem events and
// dispatching debounced callbacks. It returns nil on clean context
// cancellation and propagates any fatal watcher errors. Run must be
// called exactly once; a second call returns an error immediately.
func (w *Watcher) Run(ctx context.Context) error {
	if !w.started.CompareAndSwap(false, true) {
		return fmt.Errorf("watch: Run called more than once")
	}

	var (
		mu      sync.Mutex
		pending = make(map[string]struct{})
		timer   *time.Timer
		running atomic.Bool
	)

	// fire drains the pending set and invokes the OnChange callback.
	// It may be scheduled by time.AfterFunc after the context is cancelled,
	// so check ctx.Err() as a best-effort guard. A narrow TOCTOU window
	// remains between the check and OnChange invocation; the callback
	// receives ctx and should check it for cancellation-sensitive work.
	// Uses atomic "skip-if-busy" guard to prevent concurrent callback
	// invocations when the command takes longer than the debounce period.
	fire := func() {
		if ctx.Err() != nil {
			return
		}
		if !running.CompareAndSwap(false, true) {
			fmt.Fprintf(w.stderr, "watch: skipping re-execution (previous run still in progress)\n")
			// Schedule a retry so pending events are not permanently lost.
			// Without this, if no further filesystem events arrive, the
			// accumulated pending set would be silently discarded.
			mu.Lock()
			if timer != nil {
				timer.Reset(w.debounce)
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

	// Ensure the timer channel is drained on exit. The timer is accessed
	// under mu because it is written by the event loop under the same lock.
	defer func() {
		mu.Lock()
		localTimer := timer
		mu.Unlock()
		if localTimer != nil && !localTimer.Stop() {
			select {
			case <-localTimer.C:
			default:
			}
		}
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
				timer = time.AfterFunc(w.debounce, fire)
			} else {
				timer.Reset(w.debounce)
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

// addDirectories walks BaseDir and adds every non-ignored directory to the
// fsnotify watcher. All directories are registered regardless of watch
// patterns; pattern filtering is applied when events arrive (see
// matchesPatterns).
func (w *Watcher) addDirectories() error {
	walkErr := filepath.WalkDir(w.baseDir, func(path string, d os.DirEntry, walkDirErr error) error {
		if walkDirErr != nil {
			// Best-effort: skip directories we cannot access rather than aborting
			// the entire walk. Permission errors on individual dirs are common
			// (e.g., .git/objects/pack) and should not prevent watching.
			// Log to stderr so users know which paths are not being watched.
			fmt.Fprintf(w.stderr, "watch: skipping inaccessible path %q: %v\n", path, walkDirErr)
			return nil //nolint:nilerr // intentional skip of inaccessible paths
		}
		if !d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(w.baseDir, path)
		if relErr != nil {
			return nil //nolint:nilerr // skip paths that cannot be made relative
		}

		// Skip ignored directories entirely to avoid descending into them.
		if w.isIgnored(rel) || w.isIgnored(rel+"/") {
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
	if err != nil || !info.IsDir() {
		return
	}

	rel, err := filepath.Rel(w.baseDir, path)
	if err != nil {
		return
	}

	if w.isIgnored(rel) || w.isIgnored(rel+"/") {
		return
	}

	if addErr := w.fsw.Add(path); addErr != nil {
		fmt.Fprintf(w.stderr, "watch: add new directory %q: %v\n", path, addErr)
	}
}

// isIgnored returns true if the given path (relative to BaseDir) matches any
// ignore pattern.
func (w *Watcher) isIgnored(rel string) bool {
	// Normalise to forward slashes for consistent glob matching.
	normalized := filepath.ToSlash(rel)
	for _, pat := range w.ignores {
		if matched, matchErr := doublestar.Match(pat, normalized); matchErr == nil && matched {
			return true
		}
	}
	return false
}

// matchesPatterns returns true if the given path (relative to BaseDir) matches
// at least one of the configured watch patterns. When no patterns are
// configured, all paths match.
func (w *Watcher) matchesPatterns(rel string) bool {
	if len(w.cfg.Patterns) == 0 {
		return true
	}
	normalized := filepath.ToSlash(rel)
	for _, pat := range w.cfg.Patterns {
		if matched, matchErr := doublestar.Match(pat, normalized); matchErr == nil && matched {
			return true
		}
	}
	return false
}

// DefaultIgnores returns a copy of the built-in ignore patterns. This is
// useful for tests and tooling that need to verify the default behaviour.
func DefaultIgnores() []string {
	out := make([]string, len(defaultIgnores))
	copy(out, defaultIgnores)
	return out
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

// isIgnoredByDefaults reports whether rel matches any of the default ignore
// patterns. Package-internal helper used by tests.
func isIgnoredByDefaults(rel string) bool {
	normalized := filepath.ToSlash(rel)
	for _, pat := range defaultIgnores {
		if matched, matchErr := doublestar.Match(pat, normalized); matchErr == nil && matched {
			return true
		}
	}
	return false
}
