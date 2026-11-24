// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

const derivedTestDeniedText = "virtual path denied"

var errDerivedTestDenied = errors.New(derivedTestDeniedText)

type (
	// derivedTestValidator resolves paths physically, through every link to
	// the deepest existing ancestor (following a dangling final link), and
	// refuses each resolved path outside root. An empty root accepts every
	// path. It records every path it is asked to resolve.
	derivedTestValidator struct {
		root  string
		mu    sync.Mutex
		calls []string
	}

	// derivedTestTree is base (physical) with the validator's fixed root
	// base/root and a sibling base/outside, which is outside that root.
	derivedTestTree struct {
		base    string
		root    string
		outside string
	}

	// derivedTestRun is the result of one built-in run.
	derivedTestRun struct {
		err    error
		stderr string
	}

	// tarTestEntry is one entry of a test archive.
	tarTestEntry struct {
		name     string
		typeflag byte
		mode     int64
		body     string
		linkname string
	}
)

func newDerivedTestTree(t *testing.T) derivedTestTree {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := derivedTestTree{base: base, root: filepath.Join(base, "root"), outside: filepath.Join(base, "outside")}
	for _, dir := range []string{tr.root, tr.outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

func (tr derivedTestTree) in(parts ...string) string {
	return filepath.Join(append([]string{tr.root}, parts...)...)
}

func (tr derivedTestTree) out(parts ...string) string {
	return filepath.Join(append([]string{tr.outside}, parts...)...)
}

// run runs args through the default registry with the physical validator,
// rooted at tr.root, in dir, with the given FullAccess flag.
func (tr derivedTestTree) run(t *testing.T, dir string, full bool, args ...string) derivedTestRun {
	t.Helper()
	// Under full access the runtime's validator resolves and never refuses.
	root := tr.root
	if full {
		root = ""
	}
	validator := &derivedTestValidator{root: root}
	return runDerivedWith(t, validator, dir, full, args...)
}

func runDerivedWith(t *testing.T, validator *derivedTestValidator, dir string, full bool, args ...string) derivedTestRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	hc := &HandlerContext{
		Stdin:                strings.NewReader(""),
		Stdout:               &stdout,
		Stderr:               &stderr,
		Dir:                  dir,
		LookupEnv:            os.LookupEnv,
		ValidatePath:         validator.follow,
		ValidateNoFollowPath: validator.noFollow,
		FullAccess:           full,
	}
	err := BuildDefaultRegistry().Run(WithHandlerContext(t.Context(), hc), args[0], args)
	return derivedTestRun{err: err, stderr: stderr.String()}
}

func (v *derivedTestValidator) follow(cwd, path string) (string, error) {
	v.record(path)
	return v.check(physicalTestPath(absTestPath(cwd, path), 0))
}

func (v *derivedTestValidator) noFollow(cwd, path string) (string, error) {
	v.record(path)
	abs := absTestPath(cwd, path)
	return v.check(filepath.Join(physicalTestPath(filepath.Dir(abs), 0), filepath.Base(abs)))
}

func (v *derivedTestValidator) record(path string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls = append(v.calls, path)
}

func (v *derivedTestValidator) recorded() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.calls)
}

func (v *derivedTestValidator) check(resolved string) (string, error) {
	if v.root == "" {
		return resolved, nil
	}
	rel, err := filepath.Rel(v.root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", &os.PathError{Op: "resolve", Path: resolved, Err: errDerivedTestDenied}
	}
	return resolved, nil
}

func absTestPath(cwd, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

// physicalTestPath resolves path through every link: the deepest existing
// ancestor with filepath.EvalSymlinks, a dangling link through its target,
// and the missing components joined as written.
func physicalTestPath(path string, depth int) string {
	const maxDepth = 64
	if depth > maxDepth {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			return physicalTestPath(filepath.Clean(target), depth+1)
		}
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(physicalTestPath(parent, depth+1), filepath.Base(path))
}

// derivedSnapshot lists every entry below base with its type, permissions,
// and a regular file's content or a link's target.
func derivedSnapshot(t *testing.T, base string) map[string]string {
	t.Helper()
	type walked struct {
		path string
		info fs.FileInfo
	}
	var all []walked
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", path, err)
		}
		all = append(all, walked{path: path, info: info})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]string, len(all))
	for _, entry := range all {
		value := entry.info.Mode().String()
		switch {
		case entry.info.Mode().IsRegular():
			content, err := os.ReadFile(entry.path)
			if err != nil {
				t.Fatal(err)
			}
			value += ":" + string(content)
		case entry.info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(entry.path)
			if err != nil {
				t.Fatal(err)
			}
			value += "->" + target
		}
		entries[entry.path] = value
	}
	return entries
}

func assertSnapshotUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if maps.Equal(before, after) {
		return
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if before[path] != after[path] {
			t.Errorf("changed: %s: %q -> %q", path, before[path], after[path])
		}
	}
	for _, path := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[path]; !ok {
			t.Errorf("removed: %s", path)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func symlinkTest(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	fstree.MustSymlink(t, target, link)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertLinkTo(t *testing.T, path, target string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("%s is %s, want a symbolic link", path, info.Mode())
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("%s -> %q, want %q", path, got, target)
	}
}

func assertRegularFile(t *testing.T, path, content string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %s, want a regular file", path, info.Mode())
	}
	if got := readTestFile(t, path); got != content {
		t.Fatalf("%s holds %q, want %q", path, got, content)
	}
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s exists or cannot be read: %v", path, err)
	}
}

func writeTarArchive(t *testing.T, path string, entries ...tarTestEntry) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		hdr := &tar.Header{Name: entry.name, Typeflag: entry.typeflag, Mode: entry.mode, Linkname: entry.linkname}
		if entry.typeflag == tar.TypeReg {
			hdr.Size = int64(len(entry.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if entry.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, buf.String())
}

func tarFile(name, body string, mode int64) tarTestEntry {
	return tarTestEntry{name: name, typeflag: tar.TypeReg, mode: mode, body: body}
}

func tarDir(name string, mode int64) tarTestEntry {
	return tarTestEntry{name: name, typeflag: tar.TypeDir, mode: mode}
}

// assertDerivedRefusal checks a refused run: errors.Is want, the
// "virtual path denied" text, and every entry below base unchanged.
func assertDerivedRefusal(t *testing.T, run derivedTestRun, want error, before, after map[string]string) {
	t.Helper()
	if !errors.Is(run.err, want) {
		t.Fatalf("error = %v, want %v", run.err, want)
	}
	if !strings.Contains(run.err.Error(), derivedTestDeniedText) {
		t.Fatalf("error %q does not contain %q", run.err, derivedTestDeniedText)
	}
	assertSnapshotUnchanged(t, before, after)
}

func assertNotDenied(t *testing.T, run derivedTestRun) {
	t.Helper()
	if run.err != nil && strings.Contains(run.err.Error(), derivedTestDeniedText) {
		t.Fatalf("unexpected refusal: %v", run.err)
	}
}
