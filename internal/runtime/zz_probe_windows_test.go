// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

// TestProbeWindowsJunctions is a throwaway behaviour probe. It only logs
// observations (PROBE: lines) about directory junctions and never fails on
// behaviour.
func TestProbeWindowsJunctions(t *testing.T) {
	// Intentional failure: gotestsum's testdox format prints a test's log only
	// when it fails, so the PROBE lines reach the CI log this way.
	t.Cleanup(func() { t.Error("PROBE: intentional failure to surface the log") })
	base := fstree.TempRoot(t)
	root := filepath.Join(base, "root")
	inside := filepath.Join(root, "inside")
	outside := filepath.Join(base, "outside")
	secret := filepath.Join(outside, "secret.txt")
	j := filepath.Join(inside, "j")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatalf("mkdir inside: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.WriteFile(secret, []byte("OUTSIDE-SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	fstree.MustJunction(t, outside, j)
	t.Logf("PROBE: setup base=%q inside=%q outside=%q junction=%q -> %q", base, inside, outside, j, outside)

	probeEvalSymlinks(t, j)
	probeLstat(t, j)
	probeOSRoot(t, inside)
	probeHarness(t, inside, j)
}

func describeErr(err error) string {
	if err == nil {
		return "err=<nil>"
	}
	var pe *fs.PathError
	inner := ""
	if errors.As(err, &pe) {
		inner = fmt.Sprintf(" pathErr.Op=%q pathErr.Err type=%T pathErr.Err=%v", pe.Op, pe.Err, pe.Err)
	}
	return fmt.Sprintf("err=%q errType=%T errors.Is(err, fs.ErrNotExist)=%v%s", err.Error(), err, errors.Is(err, fs.ErrNotExist), inner)
}

func probeEvalSymlinks(t *testing.T, j string) {
	t.Helper()
	for _, p := range []string{j, filepath.Join(j, "secret.txt"), filepath.Join(j, "missing", "deeper")} {
		got, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Logf("PROBE: (a) EvalSymlinks(%q) -> error: %s", p, describeErr(err))
			continue
		}
		t.Logf("PROBE: (a) EvalSymlinks(%q) -> %q (unchanged=%v)", p, got, got == p)
	}
}

func probeLstat(t *testing.T, j string) {
	t.Helper()
	info, err := os.Lstat(j)
	if err != nil {
		t.Logf("PROBE: (b) os.Lstat(junction %q) -> error: %s", j, describeErr(err))
		return
	}
	m := info.Mode()
	t.Logf("PROBE: (b) os.Lstat(junction).Mode()=%v ModeSymlink=%v ModeIrregular=%v IsDir=%v", m, m&fs.ModeSymlink != 0, m&fs.ModeIrregular != 0, m.IsDir())
	if st, err := os.Stat(j); err == nil {
		t.Logf("PROBE: (b) os.Stat(junction).Mode()=%v IsDir=%v", st.Mode(), st.IsDir())
	} else {
		t.Logf("PROBE: (b) os.Stat(junction) -> error: %s", describeErr(err))
	}
}

func probeOSRoot(t *testing.T, inside string) {
	t.Helper()
	r, err := os.OpenRoot(inside)
	if err != nil {
		t.Logf("PROBE: (c) os.OpenRoot(inside) -> error: %s", describeErr(err))
		return
	}
	defer func() { _ = r.Close() }()
	if f, openErr := r.Open("j/secret.txt"); openErr != nil {
		t.Logf("PROBE: (c) os.Root(inside).Open(\"j/secret.txt\") through escaping junction -> REJECTED: %s", describeErr(openErr))
	} else {
		buf := make([]byte, 64)
		n, _ := f.Read(buf)
		_ = f.Close()
		t.Logf("PROBE: (c) os.Root(inside).Open(\"j/secret.txt\") through escaping junction -> ALLOWED, read %q", string(buf[:n]))
	}
	if st, statErr := r.Stat("j"); statErr != nil {
		t.Logf("PROBE: (c) os.Root(inside).Stat(\"j\") (escaping junction, last component) -> REJECTED: %s", describeErr(statErr))
	} else {
		t.Logf("PROBE: (c) os.Root(inside).Stat(\"j\") (escaping junction, last component) -> ALLOWED, mode=%v", st.Mode())
	}
	if st, lstatErr := r.Lstat("j"); lstatErr != nil {
		t.Logf("PROBE: (c) os.Root(inside).Lstat(\"j\") -> error: %s", describeErr(lstatErr))
	} else {
		t.Logf("PROBE: (c) os.Root(inside).Lstat(\"j\") -> ok, mode=%v ModeSymlink=%v ModeIrregular=%v", st.Mode(), st.Mode()&fs.ModeSymlink != 0, st.Mode()&fs.ModeIrregular != 0)
	}
	if f, createErr := r.Create("j/new-via-root.txt"); createErr != nil {
		t.Logf("PROBE: (c) os.Root(inside).Create(\"j/new-via-root.txt\") -> REJECTED: %s", describeErr(createErr))
	} else {
		_ = f.Close()
		t.Logf("PROBE: (c) os.Root(inside).Create(\"j/new-via-root.txt\") -> ALLOWED (file created outside root)")
	}
}

func probeHarness(t *testing.T, inside, j string) {
	t.Helper()
	allowed, err := normalizeExistingOrParent(inside, "")
	if err != nil {
		t.Logf("PROBE: (d) normalizeExistingOrParent(allowed root %q) -> error: %s", inside, describeErr(err))
		return
	}
	roots := normalizedRoots([]string{inside})
	t.Logf("PROBE: (d) allowed root raw=%q normalized=%q normalizedRoots=%q", inside, allowed, roots)
	cases := []struct{ label, path string }{
		{"depth 0, existing file through junction", filepath.Join(j, "secret.txt")},
		{"depth 0, new file through junction", filepath.Join(j, "new.txt")},
		{"depth 2, new file under missing dirs through junction", filepath.Join(j, "a", "b", "new.txt")},
	}
	for _, c := range cases {
		normalized, normErr := normalizeExistingOrParent(c.path, "")
		if normErr != nil {
			t.Logf("PROBE: (d) [%s] normalizeExistingOrParent(%q) -> error: %s", c.label, c.path, describeErr(normErr))
			continue
		}
		within := false
		for _, r := range roots {
			if pathWithin(r, normalized) {
				within = true
			}
		}
		verdict := "DENIED (outside allowed root)"
		if within {
			verdict = "ALLOWED (judged within allowed root)"
		}
		t.Logf("PROBE: (d) [%s] path=%q normalized=%q pathWithin(%q)=%v -> harness %s", c.label, c.path, normalized, allowed, pathWithin(allowed, normalized), verdict)
	}
}
