// SPDX-License-Identifier: MPL-2.0

package modulelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// DefaultPollInterval is how often a busy lock is retried.
	DefaultPollInterval = 100 * time.Millisecond
	// DefaultTimeout is how long a busy lock is waited for.
	DefaultTimeout = 5 * time.Minute
)

var (
	// ErrLockHeld is returned when a FileLocker is asked for a lock file it
	// already holds: waiting would wait for itself.
	ErrLockHeld = errors.New("lock file already held by this process")
	// ErrLockUnsupported is returned on platforms without an advisory file lock.
	ErrLockUnsupported = errors.New("advisory file locks are not supported on this platform")
)

type (
	// FileLocker takes exclusive OS advisory locks on lock files. One value
	// serves one command run; it refuses to take a lock file twice.
	FileLocker struct {
		pollInterval time.Duration
		timeout      time.Duration

		mu   sync.Mutex
		held map[types.FilesystemPath]struct{}
	}

	// TimeoutError reports a lock that stayed busy for the whole wait.
	TimeoutError struct {
		Path    types.FilesystemPath
		Timeout time.Duration
	}
)

// NewFileLocker returns a FileLocker that retries a busy lock every 100 ms
// and gives up after 5 minutes.
func NewFileLocker() *FileLocker {
	return &FileLocker{
		pollInterval: DefaultPollInterval,
		timeout:      DefaultTimeout,
		held:         map[types.FilesystemPath]struct{}{},
	}
}

// Error implements error.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timed out after %s waiting for lock file %s: another invowk process holds it; let it finish or stop it, then retry", e.Timeout, e.Path)
}

// Lock opens (creating it with mode 0600 when missing) and exclusively locks
// path without blocking. While the lock is busy it calls onBusy once, retries
// every poll interval, and gives up when ctx ends or the timeout passes.
// Any other failure returns at once and names the path. The returned unlock
// releases the lock and closes the file; it is safe to call more than once.
func (l *FileLocker) Lock(ctx context.Context, path types.FilesystemPath, onBusy func()) (unlock func() error, err error) {
	if lockerErr := l.Validate(); lockerErr != nil {
		return nil, lockerErr
	}
	if pathErr := path.Validate(); pathErr != nil {
		return nil, fmt.Errorf("lock file path: %w", pathErr)
	}
	if heldErr := l.markHeld(path); heldErr != nil {
		return nil, heldErr
	}
	defer func() {
		if err != nil {
			l.unmarkHeld(path)
		}
	}()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("waiting for lock file %s: %w", path, ctxErr)
	}

	f, err := os.OpenFile(string(path), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if waitErr := l.wait(ctx, f, path, onBusy); waitErr != nil {
		return nil, errors.Join(waitErr, f.Close())
	}

	var once sync.Once
	var unlockErr error
	return func() error {
		once.Do(func() {
			unlockErr = errors.Join(unlockFile(f), f.Close())
			if unlockErr != nil {
				unlockErr = fmt.Errorf("release lock file %s: %w", path, unlockErr)
			}
			l.unmarkHeld(path)
		})
		return unlockErr
	}, nil
}

// Validate returns an error when the poll interval or the timeout is not
// positive (a FileLocker not built by NewFileLocker).
func (l *FileLocker) Validate() error {
	if l.pollInterval <= 0 || l.timeout <= 0 {
		return fmt.Errorf("file locker: poll interval %s and timeout %s must be positive; use NewFileLocker", l.pollInterval, l.timeout)
	}
	return nil
}

func (l *FileLocker) wait(ctx context.Context, f *os.File, path types.FilesystemPath, onBusy func()) error {
	// The deadline starts at the first busy try. Unreferenced timers are
	// collected without Stop (Go 1.23+), so time.After needs no cleanup.
	var deadline <-chan time.Time
	for {
		busy, err := tryLockFile(f)
		if err != nil {
			return fmt.Errorf("lock file %s: %w", path, err)
		}
		if !busy {
			return nil
		}
		if deadline == nil {
			if onBusy != nil {
				onBusy()
			}
			deadline = time.After(l.timeout)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for lock file %s: %w", path, ctx.Err())
		case <-deadline:
			return &TimeoutError{Path: path, Timeout: l.timeout}
		case <-time.After(l.pollInterval):
		}
	}
}

func (l *FileLocker) markHeld(path types.FilesystemPath) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held == nil {
		l.held = map[types.FilesystemPath]struct{}{}
	}
	if _, ok := l.held[path]; ok {
		return fmt.Errorf("%w: %s", ErrLockHeld, path)
	}
	l.held[path] = struct{}{}
	return nil
}

func (l *FileLocker) unmarkHeld(path types.FilesystemPath) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.held, path)
}
