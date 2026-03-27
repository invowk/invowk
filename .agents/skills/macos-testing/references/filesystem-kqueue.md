# macOS Filesystem and kqueue Deep Dive

Reference material for the `macos-testing` skill. Covers APFS filesystem
behavior and kqueue event notification in depth.

---

## APFS (Apple File System)

APFS replaced HFS+ as the default macOS filesystem in macOS 10.13 (High Sierra).
It is optimized for flash/SSD storage and brings several behaviors that differ
from ext4 (Linux) and NTFS (Windows).

### Case Sensitivity

APFS supports two modes:
- **Case-insensitive, case-preserving** (default on macOS)
- **Case-sensitive** (opt-in at volume creation; used on some developer machines)

The default mode means the filesystem preserves the case you specify when
creating a file, but treats lookups as case-insensitive:

```
$ touch Readme.md
$ ls
Readme.md
$ cat readme.md     # works -- same file
$ cat README.MD     # works -- same file
```

**Testing implications:**

1. **Two files cannot differ only by case.** Creating `Config.cue` when
   `config.cue` already exists overwrites the original silently. Tests that
   rely on case-variant filenames coexisting in the same directory will
   silently corrupt data on macOS.

2. **`os.ReadDir()` returns the preserved case.** A file created as
   `README.md` will always appear as `README.md` in directory listings,
   even if you access it via `readme.md`.

3. **Path comparison must be case-insensitive on macOS.** Two paths that
   differ only in case may refer to the same file. Use `strings.EqualFold`
   for case-insensitive comparison, or normalize both paths to the same case
   before comparing.

4. **Cross-platform test fixtures must use distinct names.** Do not create
   test fixtures where two files differ only by case. This works on Linux
   (ext4 is case-sensitive by default) but fails silently on macOS.

```go
// WRONG: fails silently on macOS (case-insensitive APFS)
os.WriteFile(filepath.Join(dir, "Module.cue"), moduleData, 0o644)
os.WriteFile(filepath.Join(dir, "module.cue"), fileData, 0o644)
// On macOS: Module.cue is overwritten; only module.cue content remains

// CORRECT: distinct names that work on all platforms
os.WriteFile(filepath.Join(dir, "invowkmod.cue"), moduleData, 0o644)
os.WriteFile(filepath.Join(dir, "invowkfile.cue"), fileData, 0o644)
```

### Unicode Normalization (NFD vs NFC)

APFS stores filenames in **NFD (Normalization Form Decomposed)**. This means
accented characters are stored as base character + combining mark:

| Form | Representation | Bytes |
|------|---------------|-------|
| NFC (composed) | `caf\u00e9` | `63 61 66 c3 a9` |
| NFD (decomposed) | `cafe\u0301` | `63 61 66 65 cc 81` |

Go string literals use NFC by default. When a Go program creates a file with
an accented name, APFS may store it in NFD. Subsequent `os.ReadDir()` returns
the NFD form, which will not match the original NFC Go string.

```go
// Potential pitfall with non-ASCII filenames
name := "caf\u00e9.txt"  // NFC
os.WriteFile(filepath.Join(dir, name), data, 0o644)

entries, _ := os.ReadDir(dir)
// entries[0].Name() may return "cafe\u0301.txt" (NFD) on macOS
// Direct string comparison with "caf\u00e9.txt" fails
```

**Mitigation:** Use `golang.org/x/text/unicode/norm` to normalize both
strings to the same form before comparison. Or, more practically, avoid
non-ASCII characters in test filenames entirely.

**invowk relevance:** The project uses ASCII-only filenames (`invowkfile.cue`,
`invowkmod.cue`, module directory names). Unicode normalization is not a
current concern but would become relevant if non-ASCII paths are supported
in the future.

### The `/tmp` -> `/private/tmp` Symlink

macOS has a symlink: `/tmp` -> `/private/tmp`. This affects path comparison
in tests:

```
$ ls -la /tmp
lrwxr-xr-x  1 root  wheel  11 Jan  1  2020 /tmp -> private/tmp

$ python3 -c "import tempfile; print(tempfile.gettempdir())"
/var/folders/xx/.../T  (Python uses a different temp location)

$ go run -e 'fmt.Println(os.TempDir())'
/tmp                    (Go returns the symlink)

$ go run -e 'fmt.Println(filepath.EvalSymlinks("/tmp"))'
/private/tmp            (resolved path)
```

Go's `t.TempDir()` returns the **resolved** path (e.g.,
`/private/tmp/TestFoo1234567/001`). But `os.TempDir()` returns `/tmp`.

**Impact on tests:**

```go
// BROKEN: path mismatch
func TestPathOutput(t *testing.T) {
    dir := t.TempDir() // /private/tmp/TestFoo.../001
    expected := filepath.Join(os.TempDir(), "TestFoo", "output.txt")
    // expected = /tmp/TestFoo/output.txt
    // These paths refer to the same location but string comparison fails
}

// CORRECT: use t.TempDir() for all paths
func TestPathOutput(t *testing.T) {
    dir := t.TempDir()
    expected := filepath.Join(dir, "output.txt")
    actual := filepath.Join(dir, "output.txt")
    // Both use the same resolved base -- comparison works
}
```

**Additional symlinks on macOS:**
- `/var` -> `/private/var`
- `/etc` -> `/private/etc`

Any code that resolves these paths and compares against hardcoded strings
will have mismatches on macOS.

### Extended Attributes (`xattr`)

macOS uses extended attributes extensively:

| Attribute | Meaning | Set By |
|-----------|---------|--------|
| `com.apple.quarantine` | File downloaded from internet | Browser, curl |
| `com.apple.metadata:*` | Spotlight metadata | Finder, mdimportworker |
| `com.apple.FinderInfo` | Finder display metadata | Finder |
| `com.apple.ResourceFork` | Legacy resource fork | Old apps |

Key behaviors:
- `os.Stat()` size does NOT include xattr data.
- `os.Remove()` removes the file and all its xattrs.
- `os.Rename()` preserves xattrs.
- Copying files with `io.Copy` does NOT copy xattrs (use `xattr` commands
  or `copyfile(3)` for full metadata preservation).

**Testing relevance:** Extended attributes rarely affect Go tests directly.
The main exception is `com.apple.quarantine` on downloaded binaries (see the
"Code Signing / Gatekeeper" section in the main SKILL.md).

### `.DS_Store` Files

Finder creates `.DS_Store` files when a user views a directory in Finder.
These binary files store view settings (icon positions, sort order, etc.).

**Impact on tests:**
- `os.ReadDir()` may include `.DS_Store` in its results.
- Tests that count directory entries or iterate over all files may get
  unexpected results on developer machines.
- CI runners are less likely to have `.DS_Store` files but it is not
  guaranteed.

**Mitigation:** Filter `.DS_Store` when listing directories in test code:

```go
entries, err := os.ReadDir(dir)
if err != nil {
    t.Fatal(err)
}
for _, e := range entries {
    if e.Name() == ".DS_Store" {
        continue
    }
    // process entry
}
```

The invowk discovery code already handles this through its ignore patterns,
but test code that directly lists directories should be aware.

---

## kqueue Event Notification

kqueue is the BSD (and macOS) kernel event notification mechanism. Go's
`fsnotify` library uses kqueue on macOS to implement cross-platform file
watching.

### Architecture

kqueue operates on file descriptors. To watch a file:

1. Open the file to get a file descriptor (`open()` or `openat()`).
2. Register the fd with kqueue using `kevent()` and the desired filters.
3. Wait for events with `kevent()` (blocking or with timeout).
4. Process events and re-register if needed.

### Event Filters

| Filter | Description | Notes |
|--------|-------------|-------|
| `EVFILT_VNODE` | File/directory changes | Most relevant for file watching |
| `EVFILT_READ` | Data available for reading | Pipes, sockets, FIFOs |
| `EVFILT_WRITE` | Write buffer space available | Sockets, pipes |
| `EVFILT_PROC` | Process events | fork, exec, exit, signal |
| `EVFILT_SIGNAL` | Signal delivery | Alternative to sigaction |
| `EVFILT_TIMER` | Timer events | Millisecond resolution |

### EVFILT_VNODE Flags

For file watching, `EVFILT_VNODE` provides these event flags:

| Flag | Meaning | Triggered By |
|------|---------|-------------|
| `NOTE_WRITE` | Data was written | `write()`, `truncate()` |
| `NOTE_DELETE` | File was deleted | `unlink()` |
| `NOTE_RENAME` | File was renamed | `rename()` |
| `NOTE_ATTRIB` | Attributes changed | `chmod()`, `chown()`, `utimes()` |
| `NOTE_EXTEND` | File size increased | `write()` beyond previous EOF |
| `NOTE_LINK` | Link count changed | `link()`, `unlink()` of hardlink |
| `NOTE_REVOKE` | Access revoked | Volume unmount |

### Event Coalescing Behavior

kqueue coalesces multiple events on the same file descriptor between
`kevent()` calls. This is the most impactful difference from inotify for
file watcher tests.

**How coalescing works:**
1. Program calls `kevent()` to wait for events.
2. Three rapid writes happen to the same file.
3. kqueue records `NOTE_WRITE` on the first write.
4. Subsequent writes to the same fd before the next `kevent()` call do not
   generate additional events -- the existing `NOTE_WRITE` flag is already set.
5. Program wakes from `kevent()` and sees a single `NOTE_WRITE` event.

**Practical impact for invowk's watcher tests:**

```go
// In watcher_test.go, the 10ms sleep between writes ensures each write
// generates a separate fsnotify event:
for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
    os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o644)
    time.Sleep(10 * time.Millisecond) // Allow kqueue to deliver event
}
```

Without the sleep, all three writes may be coalesced into fewer events,
and the test would see fewer changed paths than expected.

### kqueue vs inotify Comparison

| Feature | kqueue (macOS) | inotify (Linux) |
|---------|---------------|-----------------|
| Watch target | File descriptor | Pathname |
| Watch granularity | Per-fd | Per-path |
| Recursive watching | Not supported | Not supported (both need manual) |
| Event coalescing | Aggressive (flags OR-ed) | Less aggressive (queue per event) |
| Rename handling | Follows fd (watch persists) | Reports `IN_MOVED_FROM` + `IN_MOVED_TO` |
| Delete handling | `NOTE_DELETE` on watched fd | `IN_DELETE` on parent dir watch |
| Resource cost | 1 open fd per watch | 1 inotify watch descriptor per path |
| File descriptor impact | Consumes from process fd limit | Separate inotify fd pool |
| Max watches | Process `RLIMIT_NOFILE` | `/proc/sys/fs/inotify/max_user_watches` |
| Cross-mount | Works (fd is mount-agnostic) | Per-filesystem |
| Modify detection | `NOTE_WRITE` (coalesced) | `IN_MODIFY` (per-syscall) |

### How `fsnotify` Uses kqueue

Go's `fsnotify` library maps kqueue events to platform-agnostic types:

| kqueue Flag | fsnotify Event |
|-------------|---------------|
| `NOTE_WRITE` | `fsnotify.Write` |
| `NOTE_DELETE` | `fsnotify.Remove` |
| `NOTE_RENAME` | `fsnotify.Rename` |
| `NOTE_ATTRIB` | `fsnotify.Chmod` |
| `NOTE_WRITE` + `NOTE_EXTEND` | `fsnotify.Write` (size increase not distinguished) |

**Important:** The coalescing behavior leaks through the abstraction.
`fsnotify.Write` events may be fewer than actual `write()` syscalls. Code
that counts individual write events will get different results on macOS vs
Linux.

### kqueue and File Descriptor Limits

Each kqueue watch requires an open file descriptor. For directory watching
(which invowk's watcher does), this means:

- 1 fd for the kqueue instance itself
- 1 fd for each watched file or directory
- Directories do not provide recursive events, so each subdirectory needs
  its own watch

For a project with 100 files across 20 directories, the watcher needs ~121
file descriptors (1 kqueue + 100 files + 20 directories). This is well within
limits for typical projects but can become significant for monorepos.

**Connection to fd limits:** macOS's default soft limit of 256 fds means
watching large directory trees can fail with `EMFILE`. Go raises the soft
limit at startup, but the hard limit still applies.

### Debugging kqueue Issues

To see active kqueue watches on macOS:

```bash
# Count open file descriptors for a process
lsof -p <pid> | wc -l

# See kqueue-specific fds
lsof -p <pid> | grep KQUEUE
```

To trace kqueue syscalls:

```bash
# Trace kevent calls (requires SIP disabled or dtrace entitlement)
sudo dtruss -t kevent -p <pid>
```

Note: SIP restricts dtrace on modern macOS. The `fs_usage` tool is more
readily available:

```bash
# Trace filesystem events for a process
sudo fs_usage -w -f filesys <pid>
```
