// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeJunctionMode is what os.Lstat reports for a Windows junction: irregular,
// and neither a symbolic link nor a directory.
const fakeJunctionMode = fs.ModeIrregular | 0o666

type (
	// fakePathEntry is one entry of fakePathFS. A link has a target or a
	// readlinkErr; any entry may fail its own lstat with lstatErr.
	fakePathEntry struct {
		mode        fs.FileMode
		target      string
		readlinkErr error
		lstatErr    error
	}

	// fakePathFS is the filesystem TestVirtualPathHarness_WalkOverFakeFilesystem
	// gives walkVirtualPath. Unlike a real filesystem it never resolves a link
	// for the caller: an lstat or readlink below a link, or below a missing
	// entry, fails the test. That is the property that makes the walk's "does
	// not exist" reliable on Windows, where the system reports a path through
	// a junction as missing.
	fakePathFS struct {
		t        *testing.T
		entries  map[string]fakePathEntry
		canon    map[string]string
		canonErr map[string]error
	}

	fakePathInfo struct {
		name string
		mode fs.FileMode
	}
)

func (i fakePathInfo) Name() string       { return i.name }
func (i fakePathInfo) Size() int64        { return 0 }
func (i fakePathInfo) Mode() fs.FileMode  { return i.mode }
func (i fakePathInfo) ModTime() time.Time { return time.Time{} }
func (i fakePathInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakePathInfo) Sys() any           { return nil }

// fakeVolume is the volume every fake path lives on: a drive on Windows, none
// elsewhere.
func fakeVolume() string {
	if runtime.GOOS == goosWindows {
		return "C:"
	}
	return ""
}

func fakeRoot() string { return fakeVolume() + string(filepath.Separator) }

// fakePath joins parts below the fake volume root.
func fakePath(parts ...string) string {
	return filepath.Join(append([]string{fakeRoot()}, parts...)...)
}

// fakeTarget writes a link target exactly as given, with the platform
// separator: the walk must not depend on a cleaned target.
func fakeTarget(parts ...string) string {
	return strings.Join(parts, string(filepath.Separator))
}

// newFakePathFS builds a filesystem holding the volume root, every directory
// in dirs with its parents, and then entries (keyed by fakePath).
func newFakePathFS(t *testing.T, dirs []string, entries map[string]fakePathEntry) *fakePathFS {
	t.Helper()
	fake := &fakePathFS{
		t:        t,
		entries:  map[string]fakePathEntry{fakeRoot(): {mode: fs.ModeDir | 0o755}},
		canon:    map[string]string{},
		canonErr: map[string]error{},
	}
	for _, dir := range dirs {
		for path := dir; path != fakeRoot(); path = filepath.Dir(path) {
			fake.entries[path] = fakePathEntry{mode: fs.ModeDir | 0o755}
		}
	}
	maps.Copy(fake.entries, entries)
	return fake
}

func (f *fakePathFS) vfs() virtualPathFS {
	return virtualPathFS{lstat: f.lstat, readlink: f.readlink, canonical: f.canonical}
}

// checkAncestors fails the test when the walk looks below a link or below a
// missing entry. It returns ENOTDIR below a plain file, as Linux and macOS do;
// on Windows syscall.ENOTDIR is ERROR_PATH_NOT_FOUND, which is what Windows
// reports there.
func (f *fakePathFS) checkAncestors(op, name string) error {
	f.t.Helper()
	ancestors := []string{}
	for path := filepath.Dir(name); ; path = filepath.Dir(path) {
		ancestors = append(ancestors, path)
		if path == filepath.Dir(path) {
			break
		}
	}
	slices.Reverse(ancestors)
	for _, ancestor := range ancestors {
		if ancestor == name {
			continue
		}
		entry, ok := f.entries[ancestor]
		switch {
		case !ok:
			f.t.Errorf("%s %s: the walk looked below the missing entry %s", op, name, ancestor)
			return fs.ErrNotExist
		case isVirtualPathLink(entry.mode):
			f.t.Errorf("%s %s: the walk looked through the unresolved link %s", op, name, ancestor)
			return fs.ErrNotExist
		case !entry.mode.IsDir():
			return syscall.ENOTDIR
		}
	}
	return nil
}

func (f *fakePathFS) lstat(name string) (fs.FileInfo, error) {
	f.t.Helper()
	cleaned := filepath.Clean(name)
	if err := f.checkAncestors("lstat", cleaned); err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}
	entry, ok := f.entries[cleaned]
	if !ok {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	if entry.lstatErr != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: entry.lstatErr}
	}
	return fakePathInfo{name: filepath.Base(cleaned), mode: entry.mode}, nil
}

func (f *fakePathFS) readlink(name string) (string, error) {
	f.t.Helper()
	cleaned := filepath.Clean(name)
	if err := f.checkAncestors("readlink", cleaned); err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	entry, ok := f.entries[cleaned]
	switch {
	case !ok:
		f.t.Errorf("readlink %s: the walk read a link that does not exist", name)
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	case entry.readlinkErr != nil:
		return "", &fs.PathError{Op: "readlink", Path: name, Err: entry.readlinkErr}
	case !isVirtualPathLink(entry.mode):
		f.t.Errorf("readlink %s: the walk read a link target from an entry that is not a link", name)
		return "", &fs.PathError{Op: "readlink", Path: name, Err: syscall.EINVAL}
	}
	return entry.target, nil
}

// canonical checks the invariant of every path the walk calls existing and
// link-free (the result prefix and the directory of an unresolved link): each
// component exists in the fake filesystem and is not a link. It then applies
// the test's canonical-form table.
func (f *fakePathFS) canonical(path string) (string, error) {
	f.t.Helper()
	cleaned := filepath.Clean(path)
	if cleaned != path {
		f.t.Errorf("canonical %s: the walk passed a path that is not clean", path)
	}
	if err := f.checkAncestors("canonical", cleaned); err != nil {
		f.t.Errorf("canonical %s: %v", path, err)
	}
	entry, ok := f.entries[cleaned]
	switch {
	case !ok:
		f.t.Errorf("canonical %s: the walk called a missing entry existing", path)
	case isVirtualPathLink(entry.mode):
		f.t.Errorf("canonical %s: the walk left a link in the existing prefix", path)
	}
	if err := f.canonErr[cleaned]; err != nil {
		return "", err
	}
	if canon, ok := f.canon[cleaned]; ok {
		return canon, nil
	}
	return cleaned, nil
}

// checkWalkResult asserts the result invariant: the suffix holds plain names
// only, and its first name does not exist under the prefix (not checked when
// the canonical-form table renamed the prefix). The prefix itself was checked
// by fakePathFS.canonical.
func (f *fakePathFS) checkWalkResult(walk virtualPathWalk, renamedPrefix bool) {
	f.t.Helper()
	for _, name := range walk.suffix {
		if name == "" || name == "." || name == virtualPathParent || strings.ContainsRune(name, filepath.Separator) {
			f.t.Errorf("suffix %q holds %q, which is not a plain name", walk.suffix, name)
		}
	}
	if len(walk.suffix) == 0 || renamedPrefix {
		return
	}
	if _, ok := f.entries[filepath.Join(walk.prefix, walk.suffix[0])]; ok {
		f.t.Errorf("suffix %q starts with a name that exists under the prefix %s", walk.suffix, walk.prefix)
	}
}

// TestVirtualPathHarness_WalkOverFakeFilesystem drives walkVirtualPath with a
// fake filesystem, so that Linux (and the Linux mutation lane) covers what only
// Windows can materialise: a junction-like entry, a reparse entry whose target
// cannot be read, a rooted link target, a missing volume root, and the
// canonical-form step. The fake fails the test when the walk inspects anything
// below a link or below a missing entry, and when the existing prefix it
// reports holds a link.
func TestVirtualPathHarness_WalkOverFakeFilesystem(t *testing.T) {
	t.Parallel()

	errUnreadable := errors.New("unreadable")
	link := func(target string) fakePathEntry { return fakePathEntry{mode: fs.ModeSymlink | 0o777, target: target} }
	junction := func(target string) fakePathEntry { return fakePathEntry{mode: fakeJunctionMode, target: target} }
	file := fakePathEntry{mode: 0o644}

	root := fakePath("root")
	outside := fakePath("outside")
	dirs := []string{
		fakePath("root", "in"),
		fakePath("outside", "sub"),
		fakePath("elsewhere", "deep"),
		fakePath("proc", "4242", "fd"),
		fakePath("dev"),
		fakePath("root", "short~1"),
	}
	entries := map[string]fakePathEntry{
		fakePath("root", "in", "f"):         file,
		fakePath("root", "in", "dang"):      link(fakeTarget("..", "..", "outside", "new")),
		fakePath("root", "x"):               file,
		fakePath("outside", "secret"):       file,
		fakePath("elsewhere", "x"):          file,
		fakePath("root", "rel"):             link(fakeTarget("..", "outside")),
		fakePath("root", "abs"):             link(outside),
		fakePath("root", "rooted"):          link(string(filepath.Separator) + fakeTarget("outside", "secret")),
		fakePath("root", "j"):               junction(outside),
		fakePath("root", "injunction"):      junction(fakePath("root", "in")),
		fakePath("root", "app"):             {mode: fakeJunctionMode, readlinkErr: syscall.ENOENT},
		fakePath("root", "cloud"):           {mode: fs.ModeDir | fs.ModeIrregular | 0o755},
		fakePath("root", "cloud", "f"):      file,
		fakePath("root", "sub"):             link(fakePath("elsewhere", "deep")),
		fakePath("root", "updown"):          link(fakeTarget("sub", "..", "x")),
		fakePath("root", "dots"):            link(fakeTarget(".", "", "in", ".", "f")),
		fakePath("root", "dotsmissing"):     link(fakeTarget(".", "in", "new", ".", "deep")),
		fakePath("root", "climb"):           link(fakeTarget("..", "..", "..", "..", "outside")),
		fakePath("root", "toin"):            link(outside),
		fakePath("root", "d"):               link(fakeTarget("nope", "..", "toin", "new")),
		fakePath("root", "dabs"):            link(root + string(filepath.Separator) + fakeTarget("nope", "..", "toin", "new")),
		fakePath("root", "dangling"):        link(fakeTarget("..", "outside", "new")),
		fakePath("root", "danglingdeep"):    link(fakeTarget("..", "outside", "a", "b")),
		fakePath("root", "loop"):            link("loop"),
		fakePath("root", "ping"):            link("pong"),
		fakePath("root", "pong"):            link("ping"),
		fakePath("root", "locked"):          {mode: fs.ModeDir | 0o755, lstatErr: fs.ErrPermission},
		fakePath("root", "tobroken"):        link("app"),
		fakePath("root", "tolocked"):        link(fakeTarget("locked", "x")),
		fakePath("dev", "stdout"):           link(fakePath("proc", "self", "fd", "1")),
		fakePath("proc", "self"):            link("4242"),
		fakePath("proc", "4242", "fd", "1"): link("pipe:[7]"),
		fakePath("root", "short~1", "d"):    link("gone"),
	}
	for i := range virtualPathMaxLinks + 1 {
		// chainNNN -> chainNNN+1: following chain001 crosses 255 links to reach
		// the directory chain256, and following chain000 crosses 256.
		entries[fakePath("root", fmt.Sprintf("chain%03d", i))] = link(fmt.Sprintf("chain%03d", i+1))
	}
	dirs = append(dirs, fakePath("root", fmt.Sprintf("chain%03d", virtualPathMaxLinks+1)))

	type walkCase struct {
		name       string
		request    string
		wantPrefix string
		wantSuffix []string
		wantLink   string
		wantErr    error
	}
	// A path below a plain file. Linux and macOS report ENOTDIR there, which
	// is an error and not "missing". On Windows the system reports
	// ERROR_PATH_NOT_FOUND, which is "does not exist" (syscall.ENOTDIR is that
	// same code there, so the fake reports it too): the walk ends at the file,
	// and the request is judged by where the file is (design D2).
	fileUsedAsDirectory := walkCase{name: "file used as a directory", request: fakePath("root", "x", "below"), wantErr: syscall.ENOTDIR}
	if runtime.GOOS == goosWindows {
		fileUsedAsDirectory.wantErr = nil
		fileUsedAsDirectory.wantPrefix = fakePath("root", "x")
		fileUsedAsDirectory.wantSuffix = []string{"below"}
	}
	tests := []walkCase{
		{name: "existing path with no link", request: fakePath("root", "in", "f"), wantPrefix: fakePath("root", "in", "f")},
		{name: "volume root", request: fakeRoot(), wantPrefix: fakeRoot()},
		{name: "one missing component", request: fakePath("root", "in", "new"), wantPrefix: fakePath("root", "in"), wantSuffix: []string{"new"}},
		{name: "three missing components", request: fakePath("root", "a", "b", "c"), wantPrefix: root, wantSuffix: []string{"a", "b", "c"}},
		{name: "relative symlink", request: fakePath("root", "rel", "secret"), wantPrefix: fakePath("outside", "secret")},
		{name: "absolute symlink", request: fakePath("root", "abs", "secret"), wantPrefix: fakePath("outside", "secret")},
		{name: "symlink as the last component", request: fakePath("root", "abs"), wantPrefix: outside},
		{name: "rooted target", request: fakePath("root", "rooted"), wantPrefix: fakePath("outside", "secret")},
		{name: "junction as the last component", request: fakePath("root", "j"), wantPrefix: outside},
		{name: "existing file through a junction", request: fakePath("root", "j", "secret"), wantPrefix: fakePath("outside", "secret")},
		{name: "new file through a junction", request: fakePath("root", "j", "new"), wantPrefix: outside, wantSuffix: []string{"new"}},
		{name: "deep new file through a junction", request: fakePath("root", "j", "a", "b", "new"), wantPrefix: outside, wantSuffix: []string{"a", "b", "new"}},
		{name: "junction that stays inside", request: fakePath("root", "injunction", "f"), wantPrefix: fakePath("root", "in", "f")},
		{name: "reparse entry that is not a link", request: fakePath("root", "app"), wantErr: syscall.ENOENT},
		{name: "path through a reparse entry that is not a link", request: fakePath("root", "app", "x"), wantErr: syscall.ENOENT},
		{name: "irregular directory is not a link", request: fakePath("root", "cloud", "f"), wantPrefix: fakePath("root", "cloud", "f")},
		{name: "parent of a link's target", request: fakePath("root", "updown"), wantPrefix: fakePath("elsewhere", "x")},
		{name: "dot and empty target components", request: fakePath("root", "dots"), wantPrefix: fakePath("root", "in", "f")},
		{
			name: "dot after a missing component", request: fakePath("root", "dotsmissing"),
			wantPrefix: fakePath("root", "in"), wantSuffix: []string{"new", "deep"}, wantLink: fakePath("root", "dotsmissing"),
		},
		{name: "parent of the volume root", request: fakePath("root", "climb", "secret"), wantPrefix: fakePath("outside", "secret")},
		{name: "parent after a missing component", request: fakePath("root", "d"), wantErr: errVirtualPathParentOfMissing, wantLink: fakePath("root", "d")},
		{name: "parent after a missing component, absolute target", request: fakePath("root", "dabs"), wantErr: errVirtualPathParentOfMissing, wantLink: fakePath("root", "dabs")},
		{
			name: "dangling link", request: fakePath("root", "dangling"),
			wantPrefix: outside, wantSuffix: []string{"new"}, wantLink: fakePath("root", "dangling"),
		},
		{
			name: "path below a dangling link", request: fakePath("root", "dangling", "more", "names"),
			wantPrefix: outside, wantSuffix: []string{"new", "more", "names"}, wantLink: fakePath("root", "dangling", "more", "names"),
		},
		{
			name: "dangling link two missing levels deep", request: fakePath("root", "danglingdeep"),
			wantPrefix: outside, wantSuffix: []string{"a", "b"}, wantLink: fakePath("root", "danglingdeep"),
		},
		{name: "missing own component after a resolved link", request: fakePath("root", "abs", "new"), wantPrefix: outside, wantSuffix: []string{"new"}},
		{
			// The record names the link in its physical directory.
			name: "dangling link below a resolved junction", request: fakePath("root", "injunction", "dang", "more"),
			wantPrefix: outside, wantSuffix: []string{"new", "more"}, wantLink: fakePath("root", "in", "dang", "more"),
		},
		{name: "self loop", request: fakePath("root", "loop"), wantErr: errVirtualPathTooManyLinks, wantLink: fakePath("root", "loop")},
		{name: "two-link cycle below the request", request: fakePath("root", "ping", "after"), wantErr: errVirtualPathTooManyLinks, wantLink: fakePath("root", "ping", "after")},
		{name: "255 links", request: fakePath("root", "chain001"), wantPrefix: fakePath("root", fmt.Sprintf("chain%03d", virtualPathMaxLinks+1))},
		{name: "256 links", request: fakePath("root", "chain000"), wantErr: errVirtualPathTooManyLinks, wantLink: fakePath("root", "chain000")},
		{name: "lstat error other than not-exist", request: fakePath("root", "locked", "x"), wantErr: fs.ErrPermission},
		{name: "lstat error inside a link target", request: fakePath("root", "tolocked"), wantErr: fs.ErrPermission, wantLink: fakePath("root", "tolocked")},
		fileUsedAsDirectory,
		{name: "unreadable link inside a link target", request: fakePath("root", "tobroken"), wantErr: syscall.ENOENT, wantLink: fakePath("root", "tobroken")},
		{
			name: "standard stream on a pipe", request: fakePath("dev", "stdout"),
			wantPrefix: fakePath("proc", "4242", "fd"), wantSuffix: []string{"pipe:[7]"}, wantLink: fakePath("dev", "stdout"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fake := newFakePathFS(t, dirs, entries)
			request := tt.request
			walk, err := walkVirtualPath(fake.vfs(), request)
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("walkVirtualPath(%s) error = %v, want %v", request, err, tt.wantErr)
			}
			if walk.prefix != tt.wantPrefix || !slices.Equal(walk.suffix, tt.wantSuffix) {
				t.Errorf("walkVirtualPath(%s) = (%q, %q), want (%q, %q)", request, walk.prefix, walk.suffix, tt.wantPrefix, tt.wantSuffix)
			}
			if walk.unresolvedLink != tt.wantLink {
				t.Errorf("walkVirtualPath(%s) unresolved link = %q, want %q", request, walk.unresolvedLink, tt.wantLink)
			}
			fake.checkWalkResult(walk, false)
		})
	}

	// Design D1: `root/toin` is a link to `outside` and `root/d` a link to
	// `nope/../toin/new`. Read as text, the target is `root/toin/new`, inside
	// the root and through an unwalked link. No result may name it.
	t.Run("no result names the unwalked link", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"d", "dabs"} {
			fake := newFakePathFS(t, dirs, entries)
			walk, err := walkVirtualPath(fake.vfs(), fakePath("root", name))
			if err == nil {
				t.Fatalf("%s: walk succeeded with (%q, %q)", name, walk.prefix, walk.suffix)
			}
			unwalked := fakePath("root", "toin", "new")
			for _, got := range []string{walk.prefix, filepath.Join(append([]string{walk.prefix}, walk.suffix...)...)} {
				if got == unwalked {
					t.Errorf("%s: the walk returned %s", name, got)
				}
			}
		}
	})

	t.Run("missing volume root", func(t *testing.T) {
		t.Parallel()
		fake := newFakePathFS(t, nil, nil)
		delete(fake.entries, fakeRoot())
		walk, err := walkVirtualPath(fake.vfs(), fakeRoot())
		if !errors.Is(err, errVirtualPathNoExistingAncestor) {
			t.Fatalf("error = %v, want errVirtualPathNoExistingAncestor", err)
		}
		if walk.prefix != "" || walk.suffix != nil || walk.unresolvedLink != "" {
			t.Errorf("walk = %+v, want the zero value", walk)
		}
	})

	t.Run("volume root that cannot be inspected", func(t *testing.T) {
		t.Parallel()
		fake := newFakePathFS(t, nil, nil)
		fake.entries[fakeRoot()] = fakePathEntry{mode: fs.ModeDir, lstatErr: fs.ErrPermission}
		_, err := walkVirtualPath(fake.vfs(), fakePath("a"))
		if !errors.Is(err, fs.ErrPermission) || errors.Is(err, errVirtualPathNoExistingAncestor) {
			t.Fatalf("error = %v, want the permission error", err)
		}
	})

	// The canonical form is applied to the existing prefix and to the
	// directory of an unresolved link, never to the missing names.
	t.Run("canonical form", func(t *testing.T) {
		t.Parallel()
		short := fakePath("root", "short~1")
		long := fakePath("root", "LongName")
		fake := newFakePathFS(t, dirs, entries)
		fake.canon[short] = long
		walk, err := walkVirtualPath(fake.vfs(), filepath.Join(short, "short~2", "new"))
		if err != nil {
			t.Fatalf("walk: %v", err)
		}
		if walk.prefix != long || !slices.Equal(walk.suffix, []string{"short~2", "new"}) {
			t.Errorf("walk = (%q, %q), want (%q, [short~2 new])", walk.prefix, walk.suffix, long)
		}
		fake.checkWalkResult(walk, true)

		walk, err = walkVirtualPath(fake.vfs(), filepath.Join(short, "d", "rest"))
		if err != nil {
			t.Fatalf("walk through the dangling link: %v", err)
		}
		if want := filepath.Join(long, "d", "rest"); walk.unresolvedLink != want {
			t.Errorf("unresolved link = %q, want %q", walk.unresolvedLink, want)
		}
		if walk.prefix != long || !slices.Equal(walk.suffix, []string{"gone", "rest"}) {
			t.Errorf("walk = (%q, %q), want (%q, [gone rest])", walk.prefix, walk.suffix, long)
		}

		fake.canonErr[short] = errUnreadable
		walk, err = walkVirtualPath(fake.vfs(), filepath.Join(short, "d"))
		if !errors.Is(err, errUnreadable) {
			t.Fatalf("error = %v, want the canonical-form error", err)
		}
		if walk.prefix != "" || walk.suffix != nil || walk.unresolvedLink != "" {
			t.Errorf("walk = %+v, want the zero value when no canonical form exists", walk)
		}
	})

	// A link target written relative to a drive (`C:name`) names no place the
	// walk can express. Only Windows has such targets.
	if runtime.GOOS == goosWindows {
		t.Run("drive-relative target", func(t *testing.T) {
			t.Parallel()
			fake := newFakePathFS(t, dirs, entries)
			fake.entries[fakePath("root", "driverel")] = link("C:outside")
			if _, err := walkVirtualPath(fake.vfs(), fakePath("root", "driverel")); !errors.Is(err, errVirtualPathLinkTarget) {
				t.Fatalf("error = %v, want errVirtualPathLinkTarget", err)
			}
		})
	}
}
