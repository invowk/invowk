// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

const (
	envLoaded    envOutcome = iota // nil error, the variable merged
	envSkipped                     // nil error, nothing merged
	envContained                   // ErrEnvFileOutsideModule, nothing merged
	envReadError                   // another error, nothing merged
)

// envOutcome is what a module-context LoadEnvFile call must produce.
type envOutcome int

// moduleEnvFixture builds base/{outside.env, mod/{app.env, sub/inner.env}} and
// returns base and the module root, with ancestor aliases resolved.
func moduleEnvFixture(t *testing.T) (base, mod string) {
	t.Helper()
	base = fstree.TempRoot(t)
	mod = filepath.Join(base, "mod")
	writeEnvTestFile(t, filepath.Join(mod, "sub", "inner.env"), "INNER=1\n")
	writeEnvTestFile(t, filepath.Join(mod, "app.env"), "APP=1\n")
	writeEnvTestFile(t, filepath.Join(base, "outside.env"), "LEAKED=1\n")
	return base, mod
}

func writeEnvTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadEnvFile_ModuleContext covers every branch of the module-context read
// (design D3): contained reads, escapes and unresolvable links (required and
// optional), absent files (optional and required), read errors, and the
// lexical checks on the path and on basePath.
func TestLoadEnvFile_ModuleContext(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	tests := []struct {
		name     string
		setup    func(t fstree.Fataler, base, mod string) (path, basePath string)
		want     envOutcome
		wantKey  string
		notExist bool // the read error must wrap fs.ErrNotExist
	}{
		{"regular file", func(_ fstree.Fataler, _, mod string) (string, string) { return "app.env", mod }, envLoaded, "APP", false},
		{"optional regular file", func(_ fstree.Fataler, _, mod string) (string, string) { return "app.env?", mod }, envLoaded, "APP", false},
		{"file in subdirectory", func(_ fstree.Fataler, _, mod string) (string, string) { return "sub/inner.env", mod }, envLoaded, "INNER", false},
		{"basePath below moduleRoot", func(_ fstree.Fataler, _, mod string) (string, string) {
			return "inner.env", filepath.Join(mod, "sub")
		}, envLoaded, "INNER", false},
		{"relative in-module link", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, filepath.Join("sub", "inner.env"), filepath.Join(mod, "l"))
			return "l", mod
		}, envLoaded, "INNER", false},
		{"absolute in-module link", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, filepath.Join(mod, "app.env"), filepath.Join(mod, "l"))
			return "l", mod
		}, envContained, "", false},
		{"escaping link", func(t fstree.Fataler, base, mod string) (string, string) {
			fstree.MustSymlink(t, filepath.Join(base, "outside.env"), filepath.Join(mod, "l"))
			return "l", mod
		}, envContained, "", false},
		{"optional escaping link", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, filepath.Join("..", "outside.env"), filepath.Join(mod, "l"))
			return "l?", mod
		}, envContained, "", false},
		{"dangling link", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, "missing.env", filepath.Join(mod, "l"))
			return "l", mod
		}, envContained, "", false},
		{"optional dangling link", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, "missing.env", filepath.Join(mod, "l"))
			return "l?", mod
		}, envContained, "", false},
		{"optional link cycle", func(t fstree.Fataler, _, mod string) (string, string) {
			fstree.MustSymlink(t, "b", filepath.Join(mod, "a"))
			fstree.MustSymlink(t, "a", filepath.Join(mod, "b"))
			return "a?", mod
		}, envContained, "", false},
		{"absolute path built in Go", func(_ fstree.Fataler, _, mod string) (string, string) {
			return filepath.Join(mod, "app.env"), mod
		}, envContained, "", false},
		{"rooted path on any OS", func(_ fstree.Fataler, _, mod string) (string, string) { return `\app.env?`, mod }, envContained, "", false},
		{"drive path on any OS", func(_ fstree.Fataler, _, mod string) (string, string) { return "C:/app.env?", mod }, envContained, "", false},
		{"lexical escape", func(_ fstree.Fataler, _, mod string) (string, string) { return "../outside.env", mod }, envContained, "", false},
		{"basePath outside moduleRoot", func(_ fstree.Fataler, base, _ string) (string, string) {
			return "outside.env", base
		}, envContained, "", false},
		{"optional missing file", func(_ fstree.Fataler, _, mod string) (string, string) { return "missing.env?", mod }, envSkipped, "", false},
		{"optional missing in missing dir", func(_ fstree.Fataler, _, mod string) (string, string) { return "nodir/missing.env?", mod }, envSkipped, "", false},
		{"required missing file", func(_ fstree.Fataler, _, mod string) (string, string) { return "missing.env", mod }, envReadError, "", true},
		{"directory", func(_ fstree.Fataler, _, mod string) (string, string) { return "sub?", mod }, envReadError, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, mod := moduleEnvFixture(t)
			path, basePath := tt.setup(t, base, mod)
			env := map[string]string{}
			err := LoadEnvFile(env, path, basePath, mod)
			assertEnvOutcome(t, env, err, tt.want, tt.wantKey)
			if tt.notExist && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("err = %v, want it to wrap fs.ErrNotExist", err)
			}
		})
	}
}

func assertEnvOutcome(t *testing.T, env map[string]string, err error, want envOutcome, wantKey string) {
	t.Helper()
	switch want {
	case envLoaded:
		if err != nil || env[wantKey] != "1" || len(env) != 1 {
			t.Fatalf("err = %v, env = %v; want %s=1 loaded", err, env, wantKey)
		}
		return
	case envSkipped:
		if err != nil {
			t.Fatalf("err = %v, want the optional file skipped", err)
		}
	case envContained:
		if !errors.Is(err, ErrEnvFileOutsideModule) {
			t.Fatalf("err = %v, want ErrEnvFileOutsideModule", err)
		}
	case envReadError:
		if err == nil || errors.Is(err, ErrEnvFileOutsideModule) {
			t.Fatalf("err = %v, want a read error that is not a containment error", err)
		}
	}
	if len(env) != 0 {
		t.Fatalf("env = %v, want nothing merged", env)
	}
}

// TestLoadEnvFile_ModuleRootMissing: a module directory that no longer exists
// skips an optional file and fails a required one with a not-exist read error
// that is not a containment error.
func TestLoadEnvFile_ModuleRootMissing(t *testing.T) {
	t.Parallel()
	mod := filepath.Join(t.TempDir(), "gone")
	env := map[string]string{}
	if err := LoadEnvFile(env, "app.env?", mod, mod); err != nil {
		t.Fatalf("optional: err = %v, want nil", err)
	}
	err := LoadEnvFile(env, "app.env", mod, mod)
	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrEnvFileOutsideModule) {
		t.Fatalf("required: err = %v, want a not-exist read error", err)
	}
	if len(env) != 0 {
		t.Fatalf("env = %v, want nothing merged", env)
	}
}

// TestLoadEnvFile_ModuleRootNotADirectory: an OpenRoot error other than
// not-exist is a read error, also for an optional file.
func TestLoadEnvFile_ModuleRootNotADirectory(t *testing.T) {
	t.Parallel()
	_, mod := moduleEnvFixture(t)
	notDir := filepath.Join(mod, "app.env")
	err := LoadEnvFile(map[string]string{}, "x.env?", notDir, notDir)
	if err == nil || errors.Is(err, ErrEnvFileOutsideModule) || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want a read error", err)
	}
}

// TestLoadEnvFile_PlainInvowkfileFollowsLinks: without a module root, a
// symlinked .env pointing outside the invowkfile directory still loads
// (decision 46 item 5).
func TestLoadEnvFile_PlainInvowkfileFollowsLinks(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base, mod := moduleEnvFixture(t)
	fstree.MustSymlink(t, filepath.Join(base, "outside.env"), filepath.Join(mod, ".env"))
	env := map[string]string{}
	if err := LoadEnvFile(env, ".env", mod, ""); err != nil || env["LEAKED"] != "1" {
		t.Fatalf("plain invowkfile: err = %v, env = %v; want the linked file loaded", err, env)
	}
}

// TestClassifyModuleEnvFileError drives the shared classifier directly,
// including the escape error that reaches it from ReadFile only through a
// race after Stat.
func TestClassifyModuleEnvFileError(t *testing.T) {
	t.Parallel()
	escape := &fs.PathError{Op: "readat", Path: "x", Err: errors.New("path escapes from parent")}
	notExist := &fs.PathError{Op: "stat", Path: "x", Err: syscall.ENOENT}
	eacces := &fs.PathError{Op: "stat", Path: "x", Err: syscall.EACCES}
	eloop := &fs.PathError{Op: "stat", Path: "x", Err: syscall.ELOOP}

	tests := []struct {
		name          string
		err, lstatErr error
		absent        bool
		escapes       bool
	}{
		{"escape from ReadFile", escape, escape, false, true},
		{"escape from Stat with Lstat ok", escape, nil, false, true},
		{"dangling link", notExist, nil, false, true},
		{"link cycle", eloop, nil, false, true},
		{"absent", notExist, notExist, true, false},
		{"not-exist with another Lstat error", notExist, eacces, false, false},
		{"permission", eacces, eacces, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			absent, err := classifyModuleEnvFileError(tt.err, tt.lstatErr)
			if absent != tt.absent {
				t.Errorf("absent = %v, want %v", absent, tt.absent)
			}
			if err == nil {
				t.Fatal("classified error is nil")
			}
			if got := errors.Is(err, errEnvFileEscapes); got != tt.escapes {
				t.Errorf("escapes = %v, want %v (err %v)", got, tt.escapes, err)
			}
			if !errors.Is(err, tt.err) {
				t.Errorf("classified error %v does not wrap the cause", err)
			}
		})
	}
}

// TestIsAbsoluteOnAnyOS: rooted and drive paths are absolute on every host,
// relative names (a drive-like name without a separator included) are not.
func TestIsAbsoluteOnAnyOS(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"":           false,
		"/etc/x.env": true,
		`\x.env`:     true,
		`\\srv\s`:    true,
		"C:/x.env":   true,
		`c:\x.env`:   true,
		"Z:/":        true,
		"C:x.env":    false,
		"1:/x.env":   false,
		"x.env":      false,
		"sub/x.env":  false,
		"./x.env":    false,
		"::/x":       false,
	} {
		if got := isAbsoluteOnAnyOS(path); got != want {
			t.Errorf("isAbsoluteOnAnyOS(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestEnvFileOutsideModuleError names the path and the module, and wraps the
// cause when there is one.
func TestEnvFileOutsideModuleError(t *testing.T) {
	t.Parallel()
	cause := errors.New("cause")
	err := envFileOutsideModuleError("a.env", "/m", "resolves outside", cause)
	if !errors.Is(err, ErrEnvFileOutsideModule) || !errors.Is(err, cause) ||
		!strings.Contains(err.Error(), "'a.env'") || !strings.Contains(err.Error(), "'/m'") {
		t.Fatalf("err = %v", err)
	}
	if err = envFileOutsideModuleError("a.env", "/m", "is absolute", nil); !errors.Is(err, ErrEnvFileOutsideModule) || strings.Contains(err.Error(), "%!") {
		t.Fatalf("err without cause = %v", err)
	}
}
