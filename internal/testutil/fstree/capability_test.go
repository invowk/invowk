// SPDX-License-Identifier: MPL-2.0

package fstree

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// policyRecorder records the policy's verdict instead of ending the test.
type policyRecorder struct {
	fatal, skip string
}

func (r *policyRecorder) Helper() {}

func (r *policyRecorder) Fatalf(format string, args ...any) { r.fatal = fmt.Sprintf(format, args...) }

func (r *policyRecorder) Skipf(format string, args ...any) { r.skip = fmt.Sprintf(format, args...) }

func TestCapabilityPolicy(t *testing.T) {
	t.Parallel()
	probeErr := errors.New("probe: privilege not held")
	missing := Caps{SymlinkErr: probeErr, JunctionErr: probeErr}
	present := Caps{Symlink: true, Junction: true}

	tests := []struct {
		goos      string
		actions   bool
		capab     Capability
		wantFatal bool
	}{
		{goos: "linux", capab: CapSymlink, wantFatal: true},
		{goos: "linux", actions: true, capab: CapSymlink, wantFatal: true},
		{goos: "darwin", capab: CapSymlink, wantFatal: true},
		{goos: "darwin", actions: true, capab: CapSymlink, wantFatal: true},
		{goos: "windows", capab: CapSymlink, wantFatal: false},
		{goos: "windows", actions: true, capab: CapSymlink, wantFatal: true},
		{goos: "linux", capab: CapJunction, wantFatal: false},
		{goos: "linux", actions: true, capab: CapJunction, wantFatal: false},
		{goos: "darwin", capab: CapJunction, wantFatal: false},
		{goos: "darwin", actions: true, capab: CapJunction, wantFatal: false},
		{goos: "windows", capab: CapJunction, wantFatal: true},
		{goos: "windows", actions: true, capab: CapJunction, wantFatal: true},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%s/%s/actions=%v", tt.goos, tt.capab, tt.actions)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			host := Host{GOOS: tt.goos, GitHubActions: tt.actions}
			if got := tt.capab.Required(host); got != tt.wantFatal {
				t.Fatalf("Required = %v, want %v", got, tt.wantFatal)
			}

			var ok policyRecorder
			require(&ok, tt.capab, present, host)
			if ok.fatal != "" || ok.skip != "" {
				t.Fatalf("an available capability must pass, got fatal %q skip %q", ok.fatal, ok.skip)
			}

			var r policyRecorder
			require(&r, tt.capab, missing, host)
			verdict := r.skip
			if tt.wantFatal {
				verdict = r.fatal
				if r.skip != "" {
					t.Fatalf("a required capability must fail, not skip: %q", r.skip)
				}
			} else if r.fatal != "" {
				t.Fatalf("a capability not required here must skip, not fail: %q", r.fatal)
			}
			if verdict == "" {
				t.Fatal("a missing capability produced no verdict")
			}
			for _, want := range []string{string(tt.capab), tt.goos, probeErr.Error()} {
				if !strings.Contains(verdict, want) {
					t.Errorf("verdict %q does not name %q", verdict, want)
				}
			}
		})
	}
}

// TestCacheGetHonoursThePolicy drives Cache.Get on injected platforms: an
// instance needing a capability required there fails with the GOOS and the
// probe error, and one needing a capability not required there is skipped and
// counted.
func TestCacheGetHonoursThePolicy(t *testing.T) {
	t.Parallel()
	probeErr := errors.New("probe: junction writer failed")
	missing := Caps{Symlink: true, JunctionErr: probeErr}
	spec := Spec{RootAtom: "root", Nodes: []Node{
		{Atom: "out", Parent: "root", Name: "out", Kind: KindDir},
		{Atom: "j", Parent: "root", Name: "j", Kind: KindJunction, Target: "out"},
	}}

	t.Run("required", func(t *testing.T) {
		t.Parallel()
		c := newCache(missing, Host{GOOS: "windows"})
		var r policyRecorder
		c.missing(&r, skipReason(spec, c.caps))
		for _, want := range []string{string(CapJunction), "windows", probeErr.Error()} {
			if !strings.Contains(r.fatal, want) {
				t.Errorf("failure %q does not name %q", r.fatal, want)
			}
		}
		if len(c.skips) != 0 {
			t.Fatalf("a required capability must not be counted as a skip: %v", c.skips)
		}
	})

	t.Run("not required", func(t *testing.T) {
		t.Parallel()
		c := newCache(missing, Host{GOOS: "linux", GitHubActions: true})
		if b := c.Get(t, "k", spec); b != nil {
			t.Fatalf("Get materialised a tree without the capability: %v", b.Root)
		}
		if c.Get(t, "k", spec) != nil || c.skips[CapJunction] != 2 {
			t.Fatalf("skips = %v, want 2 junction skips", c.skips)
		}
	})
}

func TestCapsHasUnknownCapability(t *testing.T) {
	t.Parallel()
	ok, err := Caps{Symlink: true, Junction: true}.Has(Capability("hardlink"))
	if ok || err == nil {
		t.Fatalf("Has(unknown) = %v, %v; want false and an error", ok, err)
	}
	if Capability("hardlink").Required(Host{GOOS: "linux"}) {
		t.Fatal("an unknown capability must not be required")
	}
}

func TestProbeKeepsErrors(t *testing.T) {
	t.Parallel()
	caps := Probe(t)
	if caps.Symlink != (caps.SymlinkErr == nil) {
		t.Fatalf("Symlink = %v with error %v", caps.Symlink, caps.SymlinkErr)
	}
	if caps.Junction != (caps.JunctionErr == nil) {
		t.Fatalf("Junction = %v with error %v", caps.Junction, caps.JunctionErr)
	}
}
