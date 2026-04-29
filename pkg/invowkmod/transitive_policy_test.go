// SPDX-License-Identifier: MPL-2.0

package invowkmod

import "testing"

func TestCheckMissingTransitiveDeps_AllDeclared(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRef{
		{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org/B.git", Version: "^2.0.0"},
	}
	resolved := []*ResolvedModule{
		{
			ModuleRef: requirements[0],
			ModuleID:  "io.org.A",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/B.git", Version: "^2.0.0"},
			},
		},
		{ModuleRef: requirements[1], ModuleID: "io.org.B"},
	}

	if diags := CheckMissingTransitiveDeps(requirements, resolved); len(diags) != 0 {
		t.Errorf("CheckMissingTransitiveDeps() returned %d diagnostics, want 0", len(diags))
	}
}

func TestCheckMissingTransitiveDeps_DeduplicatesMissingRefs(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRef{
		{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org/B.git", Version: "^1.0.0"},
	}
	missing := ModuleRef{GitURL: "https://github.com/org/D.git", Version: "^3.0.0"}
	resolved := []*ResolvedModule{
		{ModuleRef: requirements[0], ModuleID: "io.org.A", TransitiveDeps: []ModuleRef{missing}},
		{ModuleRef: requirements[1], ModuleID: "io.org.B", TransitiveDeps: []ModuleRef{missing}},
	}

	diags := CheckMissingTransitiveDeps(requirements, resolved)
	if len(diags) != 1 {
		t.Fatalf("CheckMissingTransitiveDeps() returned %d diagnostics, want 1", len(diags))
	}
	if diags[0].MissingRef.GitURL != missing.GitURL {
		t.Errorf("diagnostic missing URL = %q, want %q", diags[0].MissingRef.GitURL, missing.GitURL)
	}
}

func TestCheckMissingVendoredTransitiveDeps_UsesModuleRefKeys(t *testing.T) {
	t.Parallel()

	sharedURL := GitURL("https://example.com/mono.git")
	requirements := []ModuleRequirement{
		{GitURL: sharedURL, Path: "modules/A", Version: "^1.0.0"},
		{GitURL: sharedURL, Path: "modules/B", Version: "^1.0.0"},
	}
	vendored := []*Module{{
		Metadata: &Invowkmod{
			Module: "io.example.A",
			Requires: []ModuleRequirement{
				{GitURL: sharedURL, Path: "modules/B", Version: "^1.0.0"},
			},
		},
	}}

	if diags := CheckMissingVendoredTransitiveDeps(requirements, vendored); len(diags) != 0 {
		t.Errorf("CheckMissingVendoredTransitiveDeps() returned %d diagnostics, want 0", len(diags))
	}
}

func TestCheckMissingVendoredTransitiveDeps_ReportsMissingRef(t *testing.T) {
	t.Parallel()

	requirements := []ModuleRequirement{
		{GitURL: "https://example.com/root.git", Version: "^1.0.0"},
	}
	missing := ModuleRequirement{GitURL: "https://example.com/transitive.git", Version: "^2.0.0"}
	vendored := []*Module{{
		Metadata: &Invowkmod{
			Module:   "io.example.dep",
			Requires: []ModuleRequirement{missing},
		},
	}}

	diags := CheckMissingVendoredTransitiveDeps(requirements, vendored)
	if len(diags) != 1 {
		t.Fatalf("CheckMissingVendoredTransitiveDeps() returned %d diagnostics, want 1", len(diags))
	}
	if diags[0].RequiringModule != "io.example.dep" {
		t.Errorf("requiring module = %q, want io.example.dep", diags[0].RequiringModule)
	}
	if diags[0].MissingRef.GitURL != missing.GitURL {
		t.Errorf("missing URL = %q, want %q", diags[0].MissingRef.GitURL, missing.GitURL)
	}
}
