// SPDX-License-Identifier: MPL-2.0

package fspath_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

func TestJoin(t *testing.T) {
	t.Parallel()

	got := fspath.Join(types.FilesystemPath("home"), types.FilesystemPath("user"))
	want := types.FilesystemPath(filepath.Join("home", "user"))
	if got != want {
		t.Errorf("Join() = %q, want %q", got, want)
	}
}

func TestJoinStr(t *testing.T) {
	t.Parallel()

	got := fspath.JoinStr(types.FilesystemPath("modules"), "invowkmod.cue")
	want := types.FilesystemPath(filepath.Join("modules", "invowkmod.cue"))
	if got != want {
		t.Errorf("JoinStr() = %q, want %q", got, want)
	}
}

func TestJoinStr_MultipleSegments(t *testing.T) {
	t.Parallel()

	got := fspath.JoinStr(types.FilesystemPath("cache"), "sources", "repo")
	want := types.FilesystemPath(filepath.Join("cache", "sources", "repo"))
	if got != want {
		t.Errorf("JoinStr() = %q, want %q", got, want)
	}
}

func TestDir(t *testing.T) {
	t.Parallel()

	got := fspath.Dir(types.FilesystemPath("home/user/file.txt"))
	want := types.FilesystemPath(filepath.Dir("home/user/file.txt"))
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestAbs(t *testing.T) {
	t.Parallel()

	got, err := fspath.Abs(types.FilesystemPath("."))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	wantRaw, _ := filepath.Abs(".")
	want := types.FilesystemPath(wantRaw)
	if got != want {
		t.Errorf("Abs() = %q, want %q", got, want)
	}
}

func TestClean(t *testing.T) {
	t.Parallel()

	got := fspath.Clean(types.FilesystemPath("home/user/../user/./file.txt"))
	want := types.FilesystemPath(filepath.Clean("home/user/../user/./file.txt"))
	if got != want {
		t.Errorf("Clean() = %q, want %q", got, want)
	}
}

func TestFromSlash(t *testing.T) {
	t.Parallel()

	got := fspath.FromSlash(types.FilesystemPath("a/b/c"))
	want := types.FilesystemPath(filepath.FromSlash("a/b/c"))
	if got != want {
		t.Errorf("FromSlash() = %q, want %q", got, want)
	}
}

func TestIsAbs(t *testing.T) {
	t.Parallel()

	// filepath.IsAbs() is OS-specific: on Windows, paths need a drive letter
	// (e.g., C:\path) to be absolute; POSIX-style /path is not absolute.
	absPath := types.FilesystemPath("/absolute/path")
	if runtime.GOOS == platform.Windows {
		absPath = types.FilesystemPath(`C:\absolute\path`)
	}
	if !fspath.IsAbs(absPath) {
		t.Error("IsAbs() = false for absolute path")
	}
	if fspath.IsAbs(types.FilesystemPath("relative/path")) {
		t.Error("IsAbs() = true for relative path")
	}
}
