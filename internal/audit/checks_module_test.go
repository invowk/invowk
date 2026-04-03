// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestModuleMetadataChecker_GlobalTrust(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		modules: []*ScannedModule{{
			Path:      types.FilesystemPath("/home/user/.invowk/cmds/global.invowkmod"),
			SurfaceID: "global-module",
			IsGlobal:  true,
			Module: &invowkmod.Module{
				Metadata: &invowkmod.Invowkmod{
					Module: "global-module",
				},
			},
		}},
	}

	checker := NewModuleMetadataChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasGlobalTrust := false
	for _, f := range findings {
		if f.SurfaceID == "SC-10" {
			hasGlobalTrust = true
		}
	}
	if !hasGlobalTrust {
		t.Error("expected global trust finding (SC-10)")
	}
}

func TestModuleMetadataChecker_VersionPinning(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		modules: []*ScannedModule{{
			Path:      types.FilesystemPath("/test/mod.invowkmod"),
			SurfaceID: "testmod",
			Module: &invowkmod.Module{
				Metadata: &invowkmod.Invowkmod{
					Module: "testmod",
					Requires: []invowkmod.ModuleRequirement{{
						GitURL:  "https://example.com/dep.git",
						Version: "*",
					}},
				},
			},
		}},
	}

	checker := NewModuleMetadataChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasVersionPin := false
	for _, f := range findings {
		if f.Title == "Module dependency has no version constraint" {
			hasVersionPin = true
		}
	}
	if !hasVersionPin {
		t.Error("expected version pinning finding for '*' constraint")
	}
}

func TestModuleMetadataChecker_Typosquatting(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		modules: []*ScannedModule{
			{
				Path:      types.FilesystemPath("/test/modA.invowkmod"),
				SurfaceID: "io.invowk.deploy",
				Module: &invowkmod.Module{
					Metadata: &invowkmod.Invowkmod{Module: "io.invowk.deploy"},
				},
			},
			{
				Path:      types.FilesystemPath("/test/modB.invowkmod"),
				SurfaceID: "io.invowk.deploi",
				Module: &invowkmod.Module{
					Metadata: &invowkmod.Invowkmod{Module: "io.invowk.deploi"},
				},
			},
		},
	}

	checker := NewModuleMetadataChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasTyposquat := false
	for _, f := range findings {
		if f.Title == "Module ID similar to another module" {
			hasTyposquat = true
		}
	}
	if !hasTyposquat {
		t.Error("expected typosquatting finding for similar module IDs")
	}
}

func TestModuleMetadataChecker_Clean(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		modules: []*ScannedModule{{
			Path:      types.FilesystemPath("/test/mod.invowkmod"),
			SurfaceID: "io.invowk.sample",
			Module: &invowkmod.Module{
				Metadata: &invowkmod.Invowkmod{
					Module: "io.invowk.sample",
					Requires: []invowkmod.ModuleRequirement{{
						GitURL:  "https://example.com/dep.git",
						Version: "^1.0.0",
					}},
				},
			},
		}},
	}

	checker := NewModuleMetadataChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("clean module produced %d findings, want 0", len(findings))
	}
}

func TestLevenshtein(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"deploy", "deploi", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
