# inotify and Linux Filesystem Watching

Deep reference for the inotify API and Linux filesystem watching behavior.
Covers the API surface, per-user limits, error handling, event coalescing, and
comparison with other platform watching mechanisms.

---

## inotify API

inotify is the Linux kernel's file monitoring interface. It provides efficient,
event-driven notification of filesystem changes without polling. Available since
Linux 2.6.13 (2005).

### Core System Calls

**`inotify_init1(flags)`**: Creates a new inotify instance (file descriptor).

- `IN_NONBLOCK`: Non-blocking reads on the fd. `read()` returns `EAGAIN`
  instead of blocking when no events are available.
- `IN_CLOEXEC`: Close-on-exec flag. Prevents the fd from leaking to child
  processes created via `exec`. Always use this flag in multi-process
  environments.
- Returns an fd that can be used with `epoll`, `select`, or `poll`.

**`inotify_add_watch(fd, path, mask)`**: Adds a watch on a filesystem path.

- `fd`: The inotify instance from `inotify_init1`.
- `path`: The file or directory to watch. Must exist at the time of the call.
- `mask`: Bitfield of event types to watch for (see Event Mask Flags below).
- Returns a watch descriptor (wd) used to identify events from this watch.

**`read(fd, buf, len)`**: Reads events from the inotify instance.

- Each event is a `struct inotify_event` with: wd (watch descriptor), mask
  (event type), cookie (for rename correlation), len (name length), name
  (filename within the watched directory).
- Multiple events may be returned in a single `read()` call.
- The name field is only present for events on files within a watched directory
  (not for events on the watched directory itself).

### Event Mask Flags

**File events (can watch for and receive):**

| Flag | Description |
|------|-------------|
| `IN_CREATE` | File or directory created in watched directory |
| `IN_DELETE` | File or directory deleted from watched directory |
| `IN_MODIFY` | File content modified (write, truncate) |
| `IN_MOVED_FROM` | File moved out of watched directory |
| `IN_MOVED_TO` | File moved into watched directory |
| `IN_ATTRIB` | Metadata changed (permissions, timestamps, xattrs) |
| `IN_CLOSE_WRITE` | File opened for writing was closed |
| `IN_CLOSE_NOWRITE` | File opened read-only was closed |
| `IN_OPEN` | File or directory was opened |
| `IN_DELETE_SELF` | The watched item itself was deleted |
| `IN_MOVE_SELF` | The watched item itself was moved |

**Convenience masks:**

| Mask | Included Flags |
|------|---------------|
| `IN_ALL_EVENTS` | All of the above |
| `IN_CLOSE` | `IN_CLOSE_WRITE \| IN_CLOSE_NOWRITE` |
| `IN_MOVE` | `IN_MOVED_FROM \| IN_MOVED_TO` |

**Control flags (set in the mask for `inotify_add_watch`):**

| Flag | Description |
|------|-------------|
| `IN_ONESHOT` | Watch for only one event, then auto-remove |
| `IN_ONLYDIR` | Only watch if the path is a directory |
| `IN_DONT_FOLLOW` | Do not follow symlinks |
| `IN_MASK_ADD` | Add events to an existing watch (instead of replacing) |
| `IN_EXCL_UNLINK` | Exclude events for children after they are unlinked |

---

## Per-User Watch Limits

inotify has three per-user kernel limits:

### `max_user_watches`

**Path:** `/proc/sys/fs/inotify/max_user_watches`

Maximum number of inotify watches per user (across all inotify instances).

- **Default**: Varies by distro. Ubuntu 22.04+: 524288. Older distros: 8192.
- **Check current value**: `cat /proc/sys/fs/inotify/max_user_watches`
- **Temporary increase**: `sudo sysctl -w fs.inotify.max_user_watches=524288`
- **Permanent increase**: Add `fs.inotify.max_user_watches=524288` to
  `/etc/sysctl.d/99-inotify.conf` and run `sudo sysctl --system`.
- **Exhaustion error**: `ENOSPC` (errno 28, "No space left on device").

Each watched directory consumes one watch. A project with 10,000 directories
consumes 10,000 watches. CI runners running multiple test binaries concurrently
may exhaust the limit.

### `max_user_instances`

**Path:** `/proc/sys/fs/inotify/max_user_instances`

Maximum number of inotify file descriptors per user.

- **Default**: Usually 128 or 256.
- **Exhaustion error**: `EMFILE` or `ENFILE` (at the fd level, not inotify-specific).

Each call to `inotify_init1` creates one instance. Normally, one instance per
watcher is sufficient (Go's `fsnotify` uses one instance).

### `max_queued_events`

**Path:** `/proc/sys/fs/inotify/max_queued_events`

Maximum number of events queued per inotify instance before events are dropped.

- **Default**: 16384.
- **Overflow**: When the queue is full, an `IN_Q_OVERFLOW` event is generated
  (with wd=-1). The overflow event itself does not identify which watches were
  affected -- the application must re-scan.
- Relevant for burst-heavy workloads (e.g., `git checkout` of a large repo
  generates thousands of events simultaneously).

---

## Error Handling

### ENOSPC (errno 28)

**"No space left on device"** -- despite the name, this has nothing to do with
disk space when it comes from inotify. It means the per-user watch limit
(`max_user_watches`) is exhausted.

**Diagnosis:**
- `dmesg | grep inotify` may show kernel messages.
- Check `cat /proc/sys/fs/inotify/max_user_watches`.
- Count current watches: `cat /proc/*/fdinfo/* 2>/dev/null | grep -c inotify`
  (approximate).

**CI implications:**
- Shared CI runners may have many concurrent watchers from other jobs.
- GitHub Actions `ubuntu-latest` runners have a generous default (524288) but
  this can still be exhausted by many concurrent Go test binaries, each with
  their own fsnotify instance watching test directories.

### EMFILE (errno 24)

**"Too many open files"** -- the per-process file descriptor limit is reached.
This is different from inotify-specific limits. The inotify fd counts toward
the total open file count.

**Diagnosis:**
- `ulimit -n` shows the current soft limit.
- `cat /proc/<pid>/limits | grep 'Max open files'` for a specific process.

**Fix:**
- Go's runtime raises the soft limit to the hard limit at startup (since Go
  1.19), so this is rare in Go programs unless the hard limit is also low.
- Increase with `ulimit -n <higher_value>` or via `/etc/security/limits.conf`.

### ENFILE (errno 23)

**"File table overflow"** -- the system-wide file table is full. All processes
on the system share this limit.

- **Check**: `cat /proc/sys/fs/file-max`
- Rare in normal CI but possible under extreme parallel test load.

---

## Recursive Watching

inotify watches individual directories, NOT directory trees. To watch a
directory tree recursively:

1. Walk the directory tree (`filepath.Walk` or `filepath.WalkDir`).
2. Add an inotify watch on each directory.
3. Watch for `IN_CREATE` events with `IN_ISDIR` flag to detect new
   subdirectories.
4. When a new directory is created, add a watch on it and walk it (to catch
   any files/directories created between the `IN_CREATE` event and the
   watch being added -- this is a well-known race condition).
5. Watch for `IN_DELETE_SELF` and `IN_MOVE_SELF` to handle directory removal
   and rename.

**Go's `fsnotify` library** handles all of this internally. The invowk
`internal/watch/` package uses `fsnotify` and does not interact with inotify
directly. However, understanding the recursive-watch implementation is
important for diagnosing watch limit exhaustion (each subdirectory consumes
one watch).

---

## Event Coalescing Behavior

inotify coalesces consecutive identical events under specific conditions:

**Coalescing rule:** If two events have the same `wd`, `mask`, `cookie`, and
`name`, AND the older event has not yet been read from the inotify fd, the
kernel drops the newer event (keeps only one copy).

**What this means in practice:**
- Two rapid `IN_MODIFY` events on the same file in the same directory: only
  one event is delivered (if the first has not been read yet).
- `IN_MODIFY` followed by `IN_CLOSE_WRITE` on the same file: both are
  delivered (different masks).
- `IN_MODIFY` on `foo.txt` followed by `IN_MODIFY` on `bar.txt` in the same
  directory: both are delivered (different names).

**Comparison with other platforms:**

This coalescing is LESS aggressive than macOS's kqueue, which may merge
multiple distinct write events into a single `NOTE_WRITE`. Tests that rely on
exact event counts are more reliable on Linux than on macOS, but should still
use debouncing/polling patterns rather than exact count assertions.

---

## Cross-Platform Comparison

| Feature | inotify (Linux) | kqueue (macOS) | ReadDirectoryChangesW (Windows) |
|---------|-----------------|----------------|-------------------------------|
| Watches | Path-based (per-directory) | FD-based (per-file/directory) | Directory handle |
| Recursive | Manual walk + per-dir watches | Manual (similar to inotify) | Built-in (`FILE_NOTIFY_CHANGE_*`) |
| Coalescing | Moderate (identical events only) | Aggressive (multiple writes to one event) | Moderate |
| Rename tracking | `IN_MOVED_FROM` + `IN_MOVED_TO` with cookie for correlation | `NOTE_RENAME` (no old name available) | `FILE_ACTION_RENAMED_OLD_NAME` + `FILE_ACTION_RENAMED_NEW_NAME` |
| Resource limits | `max_user_watches` (kernel tunable) | Open file limit (general fd limit) | No specific limit |
| Event buffer | Kernel queue (configurable via `max_queued_events`) | Kernel queue | User-provided overlap buffer |
| Overflow signal | `IN_Q_OVERFLOW` event | `EV_ERROR` flag | ERROR_NOTIFY_ENUM_DIR |
| Watch-on-delete | `IN_DELETE_SELF` auto-removes watch | `NOTE_DELETE` + `EV_EOF` | Directory handle becomes invalid |

---

## Go's fsnotify on Linux

The `github.com/fsnotify/fsnotify` library provides cross-platform file
watching in Go. On Linux, it uses inotify internally.

**Event mapping:**

| inotify Event | fsnotify Event |
|---------------|----------------|
| `IN_CREATE` | `fsnotify.Create` |
| `IN_DELETE` | `fsnotify.Remove` |
| `IN_MODIFY` | `fsnotify.Write` |
| `IN_MOVED_FROM` | `fsnotify.Rename` |
| `IN_MOVED_TO` | `fsnotify.Create` (in target directory) |
| `IN_ATTRIB` | `fsnotify.Chmod` |

**Recursive watching:** fsnotify handles directory walking and new-directory
detection internally. The caller adds a top-level path and receives events
for all descendants.

**Error handling:** fsnotify wraps inotify errors into its own error types.
`ENOSPC` becomes a watcher-level error that closes the watcher. The invowk
`internal/watch/` package handles this via `isFatalFsnotifyError`.

---

## The watcher_fatal_unix_test.go Test

**Source:** `internal/watch/watcher_fatal_unix_test.go`

```go
//go:build !windows

func TestIsFatalFsnotifyError(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        err  error
        want bool
    }{
        {name: "ENOSPC is fatal", err: syscall.ENOSPC, want: true},
        {name: "EMFILE is fatal", err: syscall.EMFILE, want: true},
        {name: "ENFILE is fatal", err: syscall.ENFILE, want: true},
        {name: "wrapped ENOSPC is fatal", err: fmt.Errorf("fsnotify: %w", syscall.ENOSPC), want: true},
        {name: "EPERM is not fatal", err: syscall.EPERM, want: false},
        {name: "EACCES is not fatal", err: syscall.EACCES, want: false},
        {name: "generic error is not fatal", err: errors.New("something went wrong"), want: false},
    }
    // ... table-driven subtests ...
}
```

**Build tag:** `//go:build !windows` (not `linux` specifically) because the
`syscall.Errno` constants (`ENOSPC`, `EMFILE`, `ENFILE`) exist on both Linux
and macOS. However, `ENOSPC` from inotify is a Linux-specific phenomenon --
on macOS, kqueue does not produce `ENOSPC` for watch exhaustion.

**What it tests:**
- `ENOSPC`, `EMFILE`, and `ENFILE` are classified as fatal (watcher should shut
  down and surface the error).
- Wrapped errors (via `%w`) are correctly unwrapped.
- Non-fatal errno values (`EPERM`, `EACCES`) and generic errors do not trigger
  fatal handling.

**Why these errors are fatal:**
- `ENOSPC`: Cannot add more watches. The watcher is permanently degraded --
  it will miss events in directories that could not be watched.
- `EMFILE`: Cannot open more file descriptors. The watcher cannot recover
  without external intervention (closing other fds).
- `ENFILE`: System-wide fd exhaustion. Same as `EMFILE` but system-wide.
