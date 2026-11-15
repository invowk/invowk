// SPDX-License-Identifier: MPL-2.0

package fstree

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

const (
	// CapSymlink is the ability to create symbolic links.
	CapSymlink Capability = "symlink"
	// CapJunction is the ability to create Windows directory junctions.
	CapJunction Capability = "junction"
)

type (
	// Capability is a filesystem capability a binding test can need. Whether a
	// missing capability skips or fails depends on the platform (Required):
	// on a platform that must offer it, a missing capability is a broken
	// instrument, not a reason to skip.
	Capability string

	// Host is the platform a capability policy is evaluated for. Tests build
	// one directly; Require uses CurrentHost.
	Host struct {
		GOOS string
		// GitHubActions is true when GITHUB_ACTIONS=true (a CI runner).
		GitHubActions bool
	}

	// Skipper is the part of *testing.T the capability policy needs.
	Skipper interface {
		Helper()
		Fatalf(format string, args ...any)
		Skipf(format string, args ...any)
	}
)

// CurrentHost reads the running platform: runtime.GOOS and GITHUB_ACTIONS.
func CurrentHost() Host {
	return Host{GOOS: runtime.GOOS, GitHubActions: os.Getenv("GITHUB_ACTIONS") == "true"}
}

// Required reports whether the capability must be available on h:
//   - junctions on Windows;
//   - symlinks on Linux and macOS, and on Windows only in CI, because a local
//     Windows host may lack the symlink privilege (decision 3 of the
//     harden-formal-instruments change).
func (c Capability) Required(h Host) bool {
	switch c {
	case CapJunction:
		return h.GOOS == "windows"
	case CapSymlink:
		return h.GOOS == "linux" || h.GOOS == "darwin" || (h.GOOS == "windows" && h.GitHubActions)
	default:
		return false
	}
}

// Has reports whether caps offers c, and the probe error when it does not.
func (caps Caps) Has(c Capability) (bool, error) {
	switch c {
	case CapSymlink:
		return caps.Symlink, caps.SymlinkErr
	case CapJunction:
		return caps.Junction, caps.JunctionErr
	default:
		return false, fmt.Errorf("unknown capability %q", string(c))
	}
}

// Require returns when the host offers c. Otherwise it fails the test when c
// is required on this platform, and skips it when it is not; both messages
// carry the GOOS and the probe error. Binding tests skip only through this
// policy (scripts/formal.py correspondence rejects a raw Skip in them).
func Require(t *testing.T, c Capability) {
	t.Helper()
	require(t, c, Probe(t), CurrentHost())
}

func require(t Skipper, c Capability, caps Caps, h Host) {
	t.Helper()
	ok, err := caps.Has(c)
	if ok {
		return
	}
	if c.Required(h) {
		t.Fatalf("%s is required on %s: %v", c, h.GOOS, err)
		return
	}
	t.Skipf("%s unavailable on %s, where it is not required: %v", c, h.GOOS, err)
}

// MustSymlink creates a symlink at link pointing at target, or fails t (a
// *testing.T or a *rapid.T). Call Require(t, CapSymlink) before drawing.
func MustSymlink(t Fataler, target, link string) {
	t.Helper()
	if err := createSymlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

// MustJunction creates a directory junction at link pointing at target, or
// fails t. Call Require(t, CapJunction) before drawing.
func MustJunction(t Fataler, target, link string) {
	t.Helper()
	if err := createJunction(target, link); err != nil {
		t.Fatalf("junction %s -> %s: %v", link, target, err)
	}
}
