// SPDX-License-Identifier: MPL-2.0

//go:build windows

package fstree

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCreateJunction requires junction creation on Windows through the reparse
// writer, which has no fallback (design D5 of the harden-formal-instruments
// change), and checks the result functionally: the link is a mount point, not
// a symlink, and a file read through it is the target's file.
func TestCreateJunction(t *testing.T) {
	t.Parallel()
	Require(t, CapJunction)
	base := TempRoot(t)
	target := filepath.Join(base, "target")
	link := filepath.Join(base, "link")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "f"), []byte("through the junction"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := createJunction(target, link); err != nil {
		t.Fatalf("create junction (probe error %v): %v", Probe(t).JunctionErr, err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("a junction must not be ModeSymlink (Go 1.23+), got mode %v", info.Mode())
	}
	viaLink, err := os.Stat(filepath.Join(link, "f"))
	if err != nil {
		t.Fatalf("stat through the junction: %v", err)
	}
	direct, err := os.Stat(filepath.Join(target, "f"))
	if err != nil {
		t.Fatalf("stat target file: %v", err)
	}
	if !os.SameFile(viaLink, direct) {
		t.Fatal("the file through the junction is not the target's file")
	}
}
