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
