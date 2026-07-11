// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	stubVendoredHashEvaluator struct {
		results map[invowkmod.ModuleID]invowkmod.VendoredHashEvaluation
	}

	cancelingVendoredHashEvaluator struct {
		cancel context.CancelFunc
	}
)

func (s stubVendoredHashEvaluator) EvaluateVendoredModuleHash(_ *invowkmod.LockFile, module *invowkmod.Module) invowkmod.VendoredHashEvaluation {
	if module == nil || module.Metadata == nil {
		return invowkmod.VendoredHashEvaluation{Status: invowkmod.VendoredHashMissing}
	}
	if result, ok := s.results[module.Metadata.Module]; ok {
		return result
	}
	return invowkmod.VendoredHashEvaluation{Status: invowkmod.VendoredHashMatched, ModuleID: module.Metadata.Module}
}

func (c cancelingVendoredHashEvaluator) EvaluateVendoredModuleHash(_ *invowkmod.LockFile, module *invowkmod.Module) invowkmod.VendoredHashEvaluation {
	c.cancel()
	return invowkmod.VendoredHashEvaluation{Status: invowkmod.VendoredHashMatched, ModuleID: module.Metadata.Module}
}

// testLockedModule returns a LockedModule with valid DDD typed fields suitable
// for lock file test fixtures. The namespace encodes the module ID so that
// ExtractModuleIDFromNamespace recovers it correctly.
func testLockedModule(ns string) invowkmod.LockedModule {
	return invowkmod.LockedModule{
		GitURL:          "https://example.com/repo.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Namespace:       invowkmod.ModuleNamespace(ns + "@1.0.0"),
		ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
}

// hasFinding returns true if findings contains one matching the given severity
// and title (exact match or substring via strings.Contains).
func hasFinding(findings []Finding, sev Severity, titleSubstr string) bool {
	for i := range findings {
		if findings[i].Severity == sev && (findings[i].Title == titleSubstr || strings.Contains(findings[i].Title, titleSubstr)) {
			return true
		}
	}
	return false
}

func TestLockFileChecker_SingleFindingCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		module    *ScannedModule
		wantSev   Severity
		wantTitle string
	}{
		{
			name: "NoDepsNoLock",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "testmod"}},
			},
			// Zero-finding case: wantSev stays at zero value, checked below.
		},
		{
			name: "DepsWithoutLockFile",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
					Module:   "testmod",
					Requires: []invowkmod.ModuleRequirement{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}},
				}},
			},
			wantSev:   SeverityHigh,
			wantTitle: "Module has dependencies but no lock file",
		},
		{
			name: "VendoredWithoutLockFile",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "testmod"}},
				VendoredModules: []*invowkmod.Module{{
					Metadata: &invowkmod.Invowkmod{Module: "vendored.dep"},
					Path:     "/test/mod.invowkmod/invowk_modules/vendored.dep.invowkmod",
				}},
			},
			wantSev:   SeverityMedium,
			wantTitle: "Vendored modules present without lock file",
		},
		{
			name: "LockFileParseErr",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				LockPath:         "/test/mod.invowkmod/invowkmod.lock.cue",
				LockFileParseErr: errors.New("unexpected token"),
				Module:           &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "testmod"}},
			},
			wantSev:   SeverityMedium,
			wantTitle: "Lock file present but unparseable",
		},
		{
			name: "V1LockFile",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
				Module:   &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "testmod"}},
				LockFile: &invowkmod.LockFile{
					Version: invowkmod.LockFileVersionV1,
					Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{},
				},
			},
			wantSev:   SeverityMedium,
			wantTitle: "v1.0 format",
		},
		{
			name: "MissingEntry",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
				Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
					Module:   "testmod",
					Requires: []invowkmod.ModuleRequirement{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}},
				}},
				LockFile: &invowkmod.LockFile{
					Version: invowkmod.LockFileVersionV2,
					Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{},
				},
			},
			wantSev:   SeverityMedium,
			wantTitle: "Required module has no lock file entry",
		},
		{
			name: "OrphanedEntry",
			module: &ScannedModule{
				Path: "/test/mod.invowkmod", SurfaceID: "testmod",
				LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
				Module:   &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "testmod"}},
				LockFile: &invowkmod.LockFile{
					Version: invowkmod.LockFileVersionV2,
					Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
						"https://example.com/orphan.git": testLockedModule("io.example.orphan"),
					},
				},
			},
			wantSev:   SeverityLow,
			wantTitle: "Orphaned lock file entry",
		},
	}

	checker := NewLockFileChecker()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newModuleOnlyContext(t, tt.module)
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			if tt.wantTitle == "" {
				// Zero-finding case (e.g., NoDepsNoLock).
				if len(findings) != 0 {
					t.Errorf("expected 0 findings, got %d", len(findings))
				}
				return
			}

			if !hasFinding(findings, tt.wantSev, tt.wantTitle) {
				t.Errorf("expected %s finding with title containing %q", tt.wantSev, tt.wantTitle)
				for _, f := range findings {
					t.Logf("  got: [%s] %s", f.Severity, f.Title)
				}
			}
		})
	}
}

func TestLockFileChecker_MissingEntrySubpath(t *testing.T) {
	t.Parallel()

	// The lock file has an entry for the bare GitURL, but the requirement
	// specifies GitURL + Path (subpath). The fixed checkMissingEntries must
	// require an exact key match ("url#subpath"), not a substring match.
	gitURL := invowkmod.GitURL("https://example.com/monorepo.git")

	sc := newModuleOnlyContext(t, &ScannedModule{
		Path: types.FilesystemPath("/test/mod.invowkmod"), SurfaceID: "testmod",
		LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
		Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
			Module:   "testmod",
			Requires: []invowkmod.ModuleRequirement{{GitURL: gitURL, Version: "^1.0.0", Path: "modules/tools"}},
		}},
		LockFile: &invowkmod.LockFile{
			Version: invowkmod.LockFileVersionV2,
			Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
				// Entry for the bare URL without subpath -- should NOT satisfy
				// a requirement that specifies a subpath.
				invowkmod.ModuleRefKey(gitURL): testLockedModule("io.example.monorepo"),
			},
		},
	})

	checker := NewLockFileChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings, SeverityMedium, "Required module has no lock file entry") {
		t.Error("expected SeverityMedium finding when subpath key is missing (bare URL should not match)")
	}
}

func TestLockFileChecker_MissingEntryUsesModuleRefKeyNormalization(t *testing.T) {
	t.Parallel()

	requirement := invowkmod.ModuleRequirement{
		GitURL:  "https://example.com/monorepo.git",
		Version: "^1.0.0",
		Path:    `modules\tools`,
	}
	sc := newModuleOnlyContext(t, &ScannedModule{
		Path: types.FilesystemPath("/test/mod.invowkmod"), SurfaceID: "testmod",
		LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
		Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
			Module:   "testmod",
			Requires: []invowkmod.ModuleRequirement{requirement},
		}},
		LockFile: &invowkmod.LockFile{
			Version: invowkmod.LockFileVersionV2,
			Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
				"https://example.com/monorepo.git#modules/tools": testLockedModule("io.example.tools"),
			},
		},
	})

	findings, err := NewLockFileChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if hasFinding(findings, SeverityMedium, "Required module has no lock file entry") {
		t.Fatalf("normalized lock entry was reported missing: %v", findings)
	}
}

func TestLockFileChecker_DeclaredLockEntryWithoutVendoredModuleIsNotOrphaned(t *testing.T) {
	t.Parallel()

	requirement := invowkmod.ModuleRequirement{
		GitURL:  "https://example.com/dep.git",
		Version: "^1.0.0",
	}
	sc := newModuleOnlyContext(t, &ScannedModule{
		Path: types.FilesystemPath("/test/mod.invowkmod"), SurfaceID: "testmod",
		LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
		Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
			Module:   "testmod",
			Requires: []invowkmod.ModuleRequirement{requirement},
		}},
		LockFile: &invowkmod.LockFile{
			Version: invowkmod.LockFileVersionV2,
			Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
				invowkmod.ModuleRef(requirement).Key(): testLockedModule("io.example.dep"),
			},
		},
	})

	findings, err := NewLockFileChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if hasFinding(findings, SeverityLow, "Orphaned lock file entry") {
		t.Fatalf("declared lock entry was reported as orphaned: %v", findings)
	}
}

func TestLockFileChecker_HashCancellationIsReturned(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	sc := newModuleOnlyContext(t, &ScannedModule{
		Path: types.FilesystemPath("/test/mod.invowkmod"), SurfaceID: "testmod",
		LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
		Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
			Module: "testmod",
		}},
		LockFile: &invowkmod.LockFile{
			Version: invowkmod.LockFileVersionV2,
			Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
				"https://example.com/dep1.git": testLockedModule("io.example.dep1"),
				"https://example.com/dep2.git": testLockedModule("io.example.dep2"),
			},
		},
		VendoredModules: []*invowkmod.Module{
			{
				Metadata: &invowkmod.Invowkmod{Module: "io.example.dep1"},
				Path:     "/test/mod.invowkmod/invowk_modules/dep1.invowkmod",
			},
			{
				Metadata: &invowkmod.Invowkmod{Module: "io.example.dep2"},
				Path:     "/test/mod.invowkmod/invowk_modules/dep2.invowkmod",
			},
		},
	})

	checker := NewLockFileChecker(WithHashEvaluator(cancelingVendoredHashEvaluator{cancel: cancel}))
	findings, err := checker.Check(ctx, sc)
	if err == nil {
		t.Fatal("Check() returned nil error, want cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check() error = %v, want context.Canceled", err)
	}
	if len(findings) != 0 {
		t.Fatalf("Check() findings = %v, want none before cancellation", findings)
	}
}

func TestLockFileChecker_Clean(t *testing.T) {
	t.Parallel()

	lockKey := invowkmod.ModuleRefKey("https://example.com/dep.git")

	sc := newModuleOnlyContext(t, &ScannedModule{
		Path: types.FilesystemPath("/test/mod.invowkmod"), SurfaceID: "testmod",
		LockPath: "/test/mod.invowkmod/invowkmod.lock.cue",
		Module: &invowkmod.Module{Metadata: &invowkmod.Invowkmod{
			Module:   "testmod",
			Requires: []invowkmod.ModuleRequirement{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}},
		}},
		LockFile: &invowkmod.LockFile{
			Version: invowkmod.LockFileVersionV2,
			Modules: map[invowkmod.ModuleRefKey]invowkmod.LockedModule{
				lockKey: testLockedModule("io.example.dep"),
			},
		},
		VendoredModules: []*invowkmod.Module{{
			Metadata: &invowkmod.Invowkmod{Module: "io.example.dep"},
			Path:     "/test/mod.invowkmod/invowk_modules/io.example.dep.invowkmod",
		}},
	})

	checker := NewLockFileChecker(WithHashEvaluator(stubVendoredHashEvaluator{}))
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range findings {
		if f.CheckerName != lockFileCheckerName {
			continue
		}
		if f.Title == "Lock file size could not be verified" {
			continue
		}
		t.Errorf("unexpected finding: [%s] %s: %s", f.Severity, f.Title, f.Description)
	}
}
