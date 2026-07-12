// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// ============================================
// Tests for invowkmod.cue parsing and validation
// ============================================

func TestCommandScope_CanCallTargetUsesDiscoveryIdentity(t *testing.T) {
	t.Parallel()

	scope := NewCommandScope("io.example.caller")
	scope.AddDirectDependency("io.example.tools", "allowed-tools")
	scope.AddGlobalSource("global-tools")

	//nolint:thelper // Case runners are passed directly to t.Run and begin with t.Parallel.
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "allows resolved direct dependency source pair", run: func(t *testing.T) {
			t.Parallel()

			decision := scope.CanCallTarget(CommandTarget{
				Reference: "allowed-tools test",
				SourceID:  "allowed-tools",
				ModuleID:  "io.example.tools",
			})
			if !decision.Allowed {
				t.Fatalf("CanCallTarget() denied resolved source pair: %+v", decision)
			}
		}},

		{name: "denies presentation alias with unrelated discovery source", run: func(t *testing.T) {
			t.Parallel()

			decision := scope.CanCallTarget(CommandTarget{
				Reference: "allowed-tools test",
				SourceID:  "other-tools",
				ModuleID:  "io.example.tools",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed mismatched source pair: %+v", decision)
			}
			if decision.TargetSource != "other-tools" {
				t.Fatalf("TargetSource = %q, want discovery source", decision.TargetSource)
			}
			if decision.Reason != CommandScopeDenyInaccessible {
				t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
			}
		}},

		{name: "denies discovered target without split identity pair", run: func(t *testing.T) {
			t.Parallel()

			decision := scope.CanCallTarget(CommandTarget{
				Reference: "allowed-tools test",
				SourceID:  "allowed-tools",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed source-only direct dependency: %+v", decision)
			}
		}},

		{name: "allows discovered global command source", run: func(t *testing.T) {
			t.Parallel()

			decision := scope.CanCallTarget(CommandTarget{
				Reference: "global-tools lint",
				SourceID:  "global-tools",
				ModuleID:  "io.example.global",
			})
			if !decision.Allowed {
				t.Fatalf("CanCallTarget() denied global source: %+v", decision)
			}
		}},

		{name: "allows same module only when source identity also matches", run: func(t *testing.T) {
			t.Parallel()

			aliasedScope := NewCommandScope("io.example.caller")
			aliasedScope.ModuleSourceID = "alias-a"

			decision := aliasedScope.CanCallTarget(CommandTarget{
				Reference: "alias-a build",
				SourceID:  "alias-a",
				ModuleID:  "io.example.caller",
			})
			if !decision.Allowed {
				t.Fatalf("CanCallTarget() denied same module/source pair: %+v", decision)
			}
		}},

		{name: "denies same module id from different source identity", run: func(t *testing.T) {
			t.Parallel()

			aliasedScope := NewCommandScope("io.example.caller")
			aliasedScope.ModuleSourceID = "alias-a"

			decision := aliasedScope.CanCallTarget(CommandTarget{
				Reference: "alias-b build",
				SourceID:  "alias-b",
				ModuleID:  "io.example.caller",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed same module id with different source: %+v", decision)
			}
			if decision.Reason != CommandScopeDenyInaccessible {
				t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
			}
		}},

		{name: "denies unrelated module whose source matches caller module id", run: func(t *testing.T) {
			t.Parallel()

			aliasedScope := NewCommandScope("io.example.caller")
			aliasedScope.ModuleSourceID = "caller-alias"

			decision := aliasedScope.CanCallTarget(CommandTarget{
				Reference: "io.example.caller test",
				SourceID:  "io.example.caller",
				ModuleID:  "io.example.other",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed source-only same-module fallback: %+v", decision)
			}
			if decision.Reason != CommandScopeDenyInaccessible {
				t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
			}
		}},

		{name: "denies non-global source sharing a global module id", run: func(t *testing.T) {
			t.Parallel()

			globalScope := NewCommandScope("io.example.caller")

			decision := globalScope.CanCallTarget(CommandTarget{
				Reference: "local-global lint",
				SourceID:  "local-global",
				ModuleID:  "io.example.global",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed module-id-only global fallback: %+v", decision)
			}
			if decision.Reason != CommandScopeDenyInaccessible {
				t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
			}
		}},

		{name: "denies source matching global module id without discovered global source", run: func(t *testing.T) {
			t.Parallel()

			globalScope := NewCommandScope("io.example.caller")

			decision := globalScope.CanCallTarget(CommandTarget{
				Reference: "io.example.global lint",
				SourceID:  "io.example.global",
				ModuleID:  "io.example.other",
			})
			if decision.Allowed {
				t.Fatalf("CanCallTarget() allowed global module id as source fallback: %+v", decision)
			}
			if decision.Reason != CommandScopeDenyInaccessible {
				t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
			}
		}},
	}
	//nolint:paralleltest // Each table case runner begins with t.Parallel.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestNewCommandScope(t *testing.T) {
	t.Parallel()

	scope := NewCommandScope("mymodule")

	if scope.ModuleID != "mymodule" {
		t.Errorf("ModuleID = %q, want %q", scope.ModuleID, "mymodule")
	}
	if scope.ModuleSourceID != "mymodule" {
		t.Errorf("ModuleSourceID = %q, want %q", scope.ModuleSourceID, "mymodule")
	}
	if scope.GlobalSources == nil {
		t.Fatal("GlobalSources should be initialized")
	}
	if len(scope.GlobalSources) != 0 {
		t.Errorf("GlobalSources length = %d, want 0", len(scope.GlobalSources))
	}
	if scope.DirectDependencySources == nil {
		t.Fatal("DirectDependencySources should be initialized")
	}
	if len(scope.DirectDependencySources) != 0 {
		t.Errorf("DirectDependencySources length = %d, want 0", len(scope.DirectDependencySources))
	}
}

func TestCommandScope_AddGlobalSource(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{ModuleID: "mymodule"}

	scope.AddGlobalSource("global-tools")

	if !scope.GlobalSources["global-tools"] {
		t.Error("global-tools should be in GlobalSources after AddGlobalSource")
	}
}

func TestCommandScope_AddDirectDependency(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{
		ModuleID: "mymodule",
	}

	scope.AddDirectDependency("io.example.newdep", "newdep")

	if !scope.DirectDependencySources["io.example.newdep"]["newdep"] {
		t.Error("io.example.newdep/newdep pair should be in DirectDependencySources after AddDirectDependency")
	}
}

func TestHasInvowkfile(t *testing.T) {
	t.Parallel()

	t.Run("with invowkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte("cmds: []"), 0o644); err != nil {
			t.Fatalf("failed to create invowkfile.cue: %v", err)
		}

		if !HasInvowkfile(types.FilesystemPath(tmpDir)) {
			t.Error("HasInvowkfile() should return true when invowkfile.cue exists")
		}
	})

	t.Run("without invowkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		if HasInvowkfile(types.FilesystemPath(tmpDir)) {
			t.Error("HasInvowkfile() should return false when invowkfile.cue doesn't exist")
		}
	})
}

// ============================================
// Tests for Validate() methods on type definitions
// ============================================

func TestValidationIssueType_Validate(t *testing.T) {
	t.Parallel()

	validTypes := []ValidationIssueType{
		IssueTypeStructure, IssueTypeNaming, IssueTypeInvowkmod,
		IssueTypeSecurity, IssueTypeCompatibility, IssueTypeInvowkfile,
		IssueTypeCommandTree,
	}

	for _, vt := range validTypes {
		t.Run(string(vt), func(t *testing.T) {
			t.Parallel()

			err := vt.Validate()
			if err != nil {
				t.Errorf("ValidationIssueType(%q).Validate() returned unexpected error: %v", vt, err)
			}
		})
	}

	invalidTypes := []ValidationIssueType{"", "invalid", "STRUCTURE"}
	for _, vt := range invalidTypes {
		name := string(vt)
		if name == "" {
			name = "empty"
		}

		t.Run("invalid_"+name, func(t *testing.T) {
			t.Parallel()

			err := vt.Validate()
			if err == nil {
				t.Fatalf("ValidationIssueType(%q).Validate() returned nil, want error", vt)
			}
			if !errors.Is(err, ErrInvalidValidationIssueType) {
				t.Errorf("error should wrap ErrInvalidValidationIssueType, got: %v", err)
			}
		})
	}
}

func TestModuleRequirement_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     ModuleRequirement
		want    bool
		wantErr bool
	}{
		{
			"all valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			true, false,
		},
		{
			"all valid with optional fields",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "myalias",
				Path:    "subdir",
			},
			true, false,
		},
		{
			"optional fields empty are valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "~2.0.0",
				Alias:   "",
				Path:    "",
			},
			true, false,
		},
		{
			"invalid git url",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "^1.0.0",
			},
			false, true,
		},
		{
			"invalid version",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "not-semver",
			},
			false, true,
		},
		{
			"multiple invalid fields",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "not-semver",
				Alias:   "   ",
				Path:    "../escape",
			},
			false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleRequirement.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleRequirement.Validate() returned nil, want error")
				}
			} else if err != nil {
				t.Errorf("ModuleRequirement.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestValidationIssueType_String(t *testing.T) {
	t.Parallel()

	if got := IssueTypeStructure.String(); got != "structure" {
		t.Errorf("IssueTypeStructure.String() = %q, want %q", got, "structure")
	}
	if got := ValidationIssueType("").String(); got != "" {
		t.Errorf("ValidationIssueType(\"\").String() = %q, want %q", got, "")
	}
}

func TestModuleID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ModuleID
		want    bool
		wantErr bool
	}{
		{"simple", ModuleID("mymodule"), true, false},
		{"rdns two segments", ModuleID("io.invowk"), true, false},
		{"rdns three segments", ModuleID("io.invowk.sample"), true, false},
		{"single letter", ModuleID("a"), true, false},
		{"alphanumeric", ModuleID("foo123.bar456"), true, false},
		{"mixed case", ModuleID("My.Module1"), true, false},
		{"max length valid", ModuleID(strings.Repeat("a", MaxModuleIDLength)), true, false},
		{"empty", ModuleID(""), false, true},
		{"starts with digit", ModuleID("1module"), false, true},
		{"starts with dot", ModuleID(".module"), false, true},
		{"ends with dot", ModuleID("module."), false, true},
		{"consecutive dots", ModuleID("io..invowk"), false, true},
		{"contains hyphen", ModuleID("io.inv-owk"), false, true},
		{"contains underscore", ModuleID("io.inv_owk"), false, true},
		{"contains space", ModuleID("io invowk"), false, true},
		{"segment starts with digit", ModuleID("io.1invowk"), false, true},
		{"too long", ModuleID(strings.Repeat("a", MaxModuleIDLength+1)), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.id.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleID(%q).Validate() error = %v, wantValid %v", tt.id, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleID(%q).Validate() returned nil, want error", tt.id)
				}
				if !errors.Is(err, ErrInvalidModuleID) {
					t.Errorf("error should wrap ErrInvalidModuleID, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("ModuleID(%q).Validate() returned unexpected error: %v", tt.id, err)
			}
		})
	}
}

func TestModuleID_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   ModuleID
		want string
	}{
		{ModuleID("io.invowk.sample"), "io.invowk.sample"},
		{ModuleID("mymodule"), "mymodule"},
		{ModuleID(""), ""},
	}

	for _, tt := range tests {
		if got := tt.id.String(); got != tt.want {
			t.Errorf("ModuleID(%q).String() = %q, want %q", string(tt.id), got, tt.want)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()

	moduleDir := types.FilesystemPath(filepath.Join(t.TempDir(), "mymodule.invowkmod"))

	invowkfilePath := InvowkfilePath(moduleDir)
	if string(invowkfilePath) != filepath.Join(string(moduleDir), "invowkfile.cue") {
		t.Errorf("InvowkfilePath() = %q, want %q", invowkfilePath, filepath.Join(string(moduleDir), "invowkfile.cue"))
	}

	invowkmodPath := InvowkmodPath(moduleDir)
	if string(invowkmodPath) != filepath.Join(string(moduleDir), "invowkmod.cue") {
		t.Errorf("InvowkmodPath() = %q, want %q", invowkmodPath, filepath.Join(string(moduleDir), "invowkmod.cue"))
	}
}
