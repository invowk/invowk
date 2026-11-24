// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gzipTestTree creates base/root with the files f and g.gz (a gzip of g),
// and returns base and root. The fixed root of the validator is root.
func gzipTestTree(t *testing.T) (base, root string) {
	t.Helper()
	base = t.TempDir()
	root = filepath.Join(base, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("f-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "g"), []byte("g-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	plain := &HandlerContext{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, Dir: root, LookupEnv: os.LookupEnv}
	if err := BuildDefaultRegistry().Run(WithHandlerContext(t.Context(), plain), "gzip", []string{"gzip", "g"}); err != nil {
		t.Fatalf("prepare g.gz: %v (%s)", err, stderr.String())
	}
	return base, root
}

// treeSnapshot lists every entry below base with its size.
func treeSnapshot(t *testing.T, base string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, path)
		entries = append(entries, rel+":"+info.Mode().String()+":"+strings.Repeat("#", int(min(info.Size(), 64))))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func runGzip(t *testing.T, validator *operandTestValidator, dir string, args ...string) error {
	t.Helper()
	var stdout, stderr bytes.Buffer
	hc := validator.handler(dir, &stdout, &stderr)
	return BuildDefaultRegistry().Run(WithHandlerContext(t.Context(), hc), "gzip", append([]string{"gzip"}, args...))
}

func followCalls(validator *operandTestValidator) []string {
	var paths []string
	for _, call := range validator.recorded() {
		if call.follow {
			paths = append(paths, call.path)
		}
	}
	return paths
}

func TestUrootCommandOperandsGzipOutputPath(t *testing.T) {
	t.Parallel()

	t.Run("suffix option writes and checks f.z", func(t *testing.T) {
		t.Parallel()
		_, root := gzipTestTree(t)
		validator := &operandTestValidator{root: root}
		if err := runGzip(t, validator, root, "-k", "-S", ".z", "f"); err != nil {
			t.Fatalf("gzip -k -S .z f: %v", err)
		}
		want := []string{"f", filepath.Join(root, "f.z")}
		if got := followCalls(validator); !slices.Equal(got, want) {
			t.Errorf("follow-hook calls = %q, want %q", got, want)
		}
		if _, err := os.Stat(filepath.Join(root, "f.z")); err != nil {
			t.Errorf("f.z was not written: %v", err)
		}
	})

	t.Run("decompress checks f.gz and its output", func(t *testing.T) {
		t.Parallel()
		_, root := gzipTestTree(t)
		validator := &operandTestValidator{root: root}
		if err := runGzip(t, validator, root, "-d", "-k", "g.gz"); err != nil {
			t.Fatalf("gzip -d -k g.gz: %v", err)
		}
		want := []string{"g.gz", filepath.Join(root, "g")}
		if got := followCalls(validator); !slices.Equal(got, want) {
			t.Errorf("follow-hook calls = %q, want %q", got, want)
		}
	})

	for _, args := range [][]string{{"-c", "f"}, {"-t", "g.gz"}} {
		t.Run("no output file "+args[0], func(t *testing.T) {
			t.Parallel()
			_, root := gzipTestTree(t)
			validator := &operandTestValidator{root: root}
			if err := runGzip(t, validator, root, args...); err != nil {
				t.Fatalf("gzip %q: %v", args, err)
			}
			if got := followCalls(validator); !slices.Equal(got, []string{args[1]}) {
				t.Errorf("follow-hook calls = %q, want only the operand %q", got, args[1])
			}
		})
	}

	for _, args := range [][]string{{"-k", "."}, {"-k", "-S", ".z", "."}} {
		t.Run("output next to the root "+strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			base, root := gzipTestTree(t)
			before := treeSnapshot(t, base)
			validator := &operandTestValidator{root: root}
			if err := runGzip(t, validator, root, args...); !errors.Is(err, errOperandTestDenied) {
				t.Fatalf("gzip %q in the root = %v, want the denial", args, err)
			}
			if after := treeSnapshot(t, base); !slices.Equal(before, after) {
				t.Errorf("gzip %q changed the tree:\nbefore %q\nafter  %q", args, before, after)
			}
		})
	}

	t.Run("decompress output refused", func(t *testing.T) {
		t.Parallel()
		base, root := gzipTestTree(t)
		before := treeSnapshot(t, base)
		refused := filepath.Join(root, "g")
		validator := &operandTestValidator{root: root, deny: func(path string) bool { return path == refused }}
		if err := runGzip(t, validator, root, "-d", "-k", "g.gz"); !errors.Is(err, errOperandTestDenied) {
			t.Fatalf("gzip -d -k g.gz = %v, want the denial of its output", err)
		}
		if after := treeSnapshot(t, base); !slices.Equal(before, after) {
			t.Errorf("the refused gzip changed the tree:\nbefore %q\nafter  %q", before, after)
		}
	})

	for _, args := range [][]string{
		{"-k", "-S", "x/y", "f"},
		{"-k", "-S=x/y", "f"},
		{"-k", "--S=x/y", "f"},
		{"-d", "-k", "-S", "x/y", "g.gz"},
		{"-c", "-S", "../y", "f"},
	} {
		t.Run("suffix separator "+strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			base, root := gzipTestTree(t)
			before := treeSnapshot(t, base)
			validator := &operandTestValidator{root: root}
			err := runGzip(t, validator, root, args...)
			if !errors.Is(err, errGzipSuffixSeparator) {
				t.Fatalf("gzip %q = %v, want errGzipSuffixSeparator", args, err)
			}
			if !strings.Contains(err.Error(), "-S") {
				t.Errorf("the error does not name gzip -S: %v", err)
			}
			if after := treeSnapshot(t, base); !slices.Equal(before, after) {
				t.Errorf("gzip %q changed the tree:\nbefore %q\nafter  %q", args, before, after)
			}
		})
	}
}
