// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// LockWaitProject is a wait for the project lock: another module command
	// edits the same project.
	LockWaitProject LockWaitKind = "project"
	// LockWaitSource is a wait for a source lock: another module command
	// checks out the same Git source.
	LockWaitSource LockWaitKind = "source"

	// locksDirName is the directory under the module cache root that holds
	// the lock files.
	locksDirName = "locks"
)

var (
	// ErrEditLockerRequired is returned by a locking use case that was given
	// no EditLocker: production always passes one, so a missing locker is a
	// wiring bug, never a reason to run unlocked.
	ErrEditLockerRequired = errors.New("module edit requires an edit locker")

	// errLockOrder is returned when a lock is requested out of order: the
	// project lock while a source lock is held, or a second source lock.
	errLockOrder = errors.New("module lock order violated")
)

type (
	// EditLocker takes exclusive OS advisory locks on lock files. The
	// production adapter is internal/modulelock.FileLocker.
	EditLocker interface {
		// Lock tries path without blocking; while it is busy it calls onBusy
		// once, retries, and gives up when ctx ends or its timeout passes.
		Lock(ctx context.Context, path types.FilesystemPath, onBusy func()) (unlock func() error, err error)
		// PhysicalDir returns dir's physical absolute path (links and
		// junctions resolved).
		PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error)
	}

	// LockWaitKind names the lock a module command waits for.
	LockWaitKind string

	// LockWait describes a lock a module command found busy and now waits
	// for. GitURL is set for a source lock.
	LockWait struct {
		Kind     LockWaitKind
		LockPath types.FilesystemPath
		GitURL   GitURL
	}

	// EditLocks is the explicit dependency the locking use cases take: the
	// locker, the callback reporting a wait (nil prints nothing), and the
	// module cache root that holds the lock files (empty means the default
	// root NewResolver uses).
	EditLocks struct {
		Locker   EditLocker
		OnWait   func(LockWait)
		CacheDir types.FilesystemPath
	}

	// ResolverOption configures a Resolver built by NewResolver.
	ResolverOption func(*resolverConfig)

	resolverConfig struct {
		editLocks *EditLocks
	}

	// editLocks takes the project and source locks of one Resolver and
	// records which it holds, so the order project-then-source is enforced.
	editLocks struct {
		locks    EditLocks
		cacheDir types.FilesystemPath

		mu      sync.Mutex
		project bool
		source  bool
	}
)

// WithEditLocks gives the Resolver the edit locks its locking methods take
// (LockProject and the use cases built on it).
func WithEditLocks(locks EditLocks) ResolverOption {
	return func(cfg *resolverConfig) {
		cfg.editLocks = &locks
	}
}

// String returns the wait kind name.
func (k LockWaitKind) String() string { return string(k) }

// Validate returns an error when k is not a known wait kind.
func (k LockWaitKind) Validate() error {
	switch k {
	case LockWaitProject, LockWaitSource:
		return nil
	default:
		return fmt.Errorf("invalid lock wait kind %q", string(k))
	}
}

// Validate returns an error when the wait kind or the lock path is invalid,
// or a source wait has no Git URL.
func (w LockWait) Validate() error {
	if err := w.Kind.Validate(); err != nil {
		return err
	}
	if err := w.LockPath.Validate(); err != nil {
		return fmt.Errorf("lock path: %w", err)
	}
	if w.Kind == LockWaitSource && w.GitURL == "" {
		return errors.New("source lock wait without a Git URL")
	}
	if w.GitURL != "" {
		if err := w.GitURL.Validate(); err != nil {
			return fmt.Errorf("lock wait Git URL: %w", err)
		}
	}
	return nil
}

// Validate returns ErrEditLockerRequired when no locker is set, and an error
// when CacheDir is set but invalid.
func (l EditLocks) Validate() error {
	if l.Locker == nil {
		return ErrEditLockerRequired
	}
	if l.CacheDir != "" {
		if err := l.CacheDir.Validate(); err != nil {
			return fmt.Errorf("edit locks cache directory: %w", err)
		}
	}
	return nil
}

// Validate returns the edit locks' validation error, if any.
func (c resolverConfig) Validate() error {
	if c.editLocks == nil {
		return nil
	}
	return c.editLocks.Validate()
}

// Validate returns an error when the edit locks or the cache root are
// invalid.
func (l *editLocks) Validate() error {
	if err := l.locks.Validate(); err != nil {
		return err
	}
	if err := l.cacheDir.Validate(); err != nil {
		return fmt.Errorf("lock cache root: %w", err)
	}
	return nil
}

// LockProject takes the project lock of this resolver's working directory:
// `<cache root>/locks/project-<hex SHA-256 of the directory's physical
// path>.lock`. It creates the cache root and its locks directory when
// missing, reports a busy lock through EditLocks.OnWait once, and fails with
// ErrEditLockerRequired when the resolver has no locker. Every locking use
// case takes the project lock here, and the returned unlock releases it
// through editLocks.release.
func (m *Resolver) LockProject(ctx context.Context) (unlock func() error, err error) {
	l := m.locks
	if l == nil {
		return nil, ErrEditLockerRequired
	}
	if orderErr := l.claim(LockWaitProject); orderErr != nil {
		return nil, orderErr
	}
	defer func() {
		if err != nil {
			l.unclaim(LockWaitProject)
		}
	}()

	physical, err := l.locks.Locker.PhysicalDir(m.workingDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project directory %s for its lock: %w", m.workingDir, err)
	}
	path, err := l.lockPath("project", string(physical))
	if err != nil {
		return nil, err
	}
	release, err := l.locks.Locker.Lock(ctx, path, l.onBusy(LockWait{Kind: LockWaitProject, LockPath: path}))
	if err != nil {
		return nil, lockFileError(err)
	}
	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() { releaseErr = l.release(LockWaitProject, release) })
		return releaseErr
	}, nil
}

// lockSource takes the source lock of gitURL's Git checkout:
// `<cache root>/locks/source-<hex SHA-256 of sourceLockKey>.lock`.
// resolveOne takes it right before the fetch and releases it when it returns
// (design D3), so the clone or fetch, the checkout, the copy into the cache
// and its verification never interleave with another process's checkout of
// the same source. It fails with errLockOrder while this resolver holds
// another source lock. A resolver without edit locks takes no source lock
// (unit tests that build a Resolver directly); every production construction
// sets them.
func (m *Resolver) lockSource(ctx context.Context, gitURL GitURL) (unlock func() error, err error) {
	l := m.locks
	if l == nil {
		return func() error { return nil }, nil
	}
	if orderErr := l.claim(LockWaitSource); orderErr != nil {
		return nil, orderErr
	}
	defer func() {
		if err != nil {
			l.unclaim(LockWaitSource)
		}
	}()

	path, err := l.lockPath("source", sourceLockKey(gitURL))
	if err != nil {
		return nil, err
	}
	release, err := l.locks.Locker.Lock(ctx, path, l.onBusy(LockWait{Kind: LockWaitSource, LockPath: path, GitURL: gitURL}))
	if err != nil {
		return nil, lockFileError(err)
	}
	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() { releaseErr = l.release(LockWaitSource, release) })
		return releaseErr
	}, nil
}

// sourceLockKey is the key a source lock file is named after: sourceCheckoutKey
// cleaned as filepath.Join cleans it for the checkout path, in slash form and
// lower case. Spellings that differ only in a trailing slash, a doubled slash
// or letter case can share one checkout (filepath.Join cleans the path, and
// NTFS and APFS ignore case by default), so they share one lock; on a
// case-sensitive volume two such spellings only wait for each other.
//
//goplint:ignore -- internal lock-name part, hashed into the file name.
func sourceLockKey(gitURL GitURL) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(filepath.FromSlash(sourceCheckoutKey(gitURL)))))
}

// claim records that kind is being taken, enforcing the lock order: no
// project lock under a source lock, one project lock at a time, and one
// source lock at a time.
func (l *editLocks) claim(kind LockWaitKind) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch kind {
	case LockWaitProject:
		if l.source {
			return fmt.Errorf("%w: project lock requested while a source lock is held", errLockOrder)
		}
		if l.project {
			return fmt.Errorf("%w: a second project lock requested while one is held", errLockOrder)
		}
		l.project = true
	case LockWaitSource:
		if l.source {
			return fmt.Errorf("%w: a second source lock requested while one is held", errLockOrder)
		}
		l.source = true
	}
	return nil
}

func (l *editLocks) unclaim(kind LockWaitKind) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch kind {
	case LockWaitProject:
		l.project = false
	case LockWaitSource:
		l.source = false
	}
}

// release is the one release point of every lock: it drops the held-lock
// record and then calls the adapter's unlock.
func (l *editLocks) release(kind LockWaitKind, unlock func() error) error {
	l.unclaim(kind)
	return unlock()
}

// onBusy returns the adapter callback reporting wait to EditLocks.OnWait.
func (l *editLocks) onBusy(wait LockWait) func() {
	return func() {
		if l.locks.OnWait != nil {
			l.locks.OnWait(wait)
		}
	}
}

// lockPath returns `<cache root>/locks/<prefix>-<hex SHA-256 of key>.lock`,
// creating the cache root (0755) and the locks directory (0700) when missing;
// an existing directory keeps its mode.
//
//goplint:ignore -- prefix and key are internal lock-name parts, hashed into the file name.
func (l *editLocks) lockPath(prefix, key string) (types.FilesystemPath, error) {
	locksDir := filepath.Join(string(l.cacheDir), locksDirName)
	if err := os.MkdirAll(string(l.cacheDir), 0o755); err != nil {
		return "", lockDirError(string(l.cacheDir), err)
	}
	if err := os.Mkdir(locksDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", lockDirError(locksDir, err)
	}
	sum := sha256.Sum256([]byte(key))
	path := types.FilesystemPath(filepath.Join(locksDir, prefix+"-"+hex.EncodeToString(sum[:])+".lock")) //goplint:ignore -- built from the validated cache root and a hex name, validated below.
	if err := path.Validate(); err != nil {
		return "", fmt.Errorf("lock file path: %w", err)
	}
	return path, nil
}

// lockDirError names dir, the cache root or its locks directory, whichever
// could not be created.
//
//goplint:ignore -- private error helper over a raw directory path.
func lockDirError(dir string, cause error) error {
	return fmt.Errorf("cannot create module lock directory %s: %w; set INVOWK_MODULES_PATH to a writable directory", dir, cause)
}

// lockFileError adds the INVOWK_MODULES_PATH hint to a lock file that could
// not be created or opened (a *fs.PathError from the adapter's open: no
// permission on locks/, or a locks path that is not a directory). A busy-lock
// timeout, a cancelled wait and every other error pass through unchanged.
func lockFileError(err error) error {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok && pathErr != nil {
		return fmt.Errorf("%w; set INVOWK_MODULES_PATH to a writable directory", err)
	}
	return err
}
