// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"testing"
)

func TestCheckMissingTransitiveDeps_AllDeclared(t *testing.T) {
	t.Parallel()

	// Root declares A and B. A has transitive dep B. B has no transitive deps.
	// Since B is already declared, nothing is missing.
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
		{
			ModuleRef:      requirements[1],
			ModuleID:       "io.org.B",
			TransitiveDeps: nil,
		},
	}

	diags := checkMissingTransitiveDeps(requirements, resolved)
	if len(diags) != 0 {
		t.Errorf("expected 0 missing deps, got %d: %v", len(diags), diags)
	}
}

func TestCheckMissingTransitiveDeps_SomeMissing(t *testing.T) {
	t.Parallel()

	// Root declares A only. A requires B and C transitively. B and C are missing.
	requirements := []ModuleRef{
		{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"},
	}

	resolved := []*ResolvedModule{
		{
			ModuleRef: requirements[0],
			ModuleID:  "io.org.A",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/B.git", Version: "^2.0.0"},
				{GitURL: "https://github.com/org/C.git", Version: "~1.5.0"},
			},
		},
	}

	diags := checkMissingTransitiveDeps(requirements, resolved)
	if len(diags) != 2 {
		t.Fatalf("expected 2 missing deps, got %d", len(diags))
	}

	// Verify the diagnostics contain the right module info.
	if diags[0].MissingRef.GitURL != "https://github.com/org/B.git" {
		t.Errorf("diag[0] missing URL = %q, want B.git", diags[0].MissingRef.GitURL)
	}
	if diags[1].MissingRef.GitURL != "https://github.com/org/C.git" {
		t.Errorf("diag[1] missing URL = %q, want C.git", diags[1].MissingRef.GitURL)
	}
	if diags[0].RequiringModule != "io.org.A" {
		t.Errorf("diag[0] requiring module = %q, want io.org.A", diags[0].RequiringModule)
	}
}

func TestCheckMissingTransitiveDeps_DiamondDep(t *testing.T) {
	t.Parallel()

	// Root declares A, B, and C. Both A and B require C transitively.
	// C is already declared, so nothing is missing.
	requirements := []ModuleRef{
		{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org/B.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org/C.git", Version: "^1.0.0"},
	}

	resolved := []*ResolvedModule{
		{
			ModuleRef: requirements[0],
			ModuleID:  "io.org.A",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/C.git", Version: "^1.0.0"},
			},
		},
		{
			ModuleRef: requirements[1],
			ModuleID:  "io.org.B",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/C.git", Version: "^1.0.0"},
			},
		},
		{
			ModuleRef:      requirements[2],
			ModuleID:       "io.org.C",
			TransitiveDeps: nil,
		},
	}

	diags := checkMissingTransitiveDeps(requirements, resolved)
	if len(diags) != 0 {
		t.Errorf("expected 0 missing deps (diamond satisfied), got %d", len(diags))
	}
}

func TestCheckMissingTransitiveDeps_Deduplication(t *testing.T) {
	t.Parallel()

	// Root declares A and B. Both A and B require D transitively.
	// D is NOT declared. It should be reported only once.
	requirements := []ModuleRef{
		{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org/B.git", Version: "^1.0.0"},
	}

	resolved := []*ResolvedModule{
		{
			ModuleRef: requirements[0],
			ModuleID:  "io.org.A",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/D.git", Version: "^3.0.0"},
			},
		},
		{
			ModuleRef: requirements[1],
			ModuleID:  "io.org.B",
			TransitiveDeps: []ModuleRef{
				{GitURL: "https://github.com/org/D.git", Version: "^3.0.0"},
			},
		},
	}

	diags := checkMissingTransitiveDeps(requirements, resolved)
	if len(diags) != 1 {
		t.Fatalf("expected 1 deduplicated missing dep, got %d", len(diags))
	}
	if diags[0].MissingRef.GitURL != "https://github.com/org/D.git" {
		t.Errorf("missing URL = %q, want D.git", diags[0].MissingRef.GitURL)
	}
}
