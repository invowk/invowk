// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestReadScriptFileFacts_BoundaryCheck(t *testing.T) {
	t.Parallel()

	// Create a module directory with a legitimate script inside it.
	moduleDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho hello"
	if err := os.WriteFile(filepath.Join(moduleDir, "run.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create a sensitive file outside the module boundary.
	outsideDir := t.TempDir()
	sensitiveContent := "SECRET=hunter2"
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.env"), []byte(sensitiveContent), 0o644); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}

	// Compute a traversal path that escapes the module directory.
	relToSensitive, err := filepath.Rel(moduleDir, filepath.Join(outsideDir, "secret.env"))
	if err != nil {
		t.Fatalf("failed to compute relative path: %v", err)
	}

	tests := []struct {
		name       string
		scriptPath string
		modulePath string
		want       string
	}{
		{
			name:       "legitimate script within module",
			scriptPath: "run.sh",
			modulePath: moduleDir,
			want:       scriptContent,
		},
		{
			name:       "traversal path blocked by boundary check",
			scriptPath: relToSensitive,
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "explicit dotdot traversal blocked",
			scriptPath: "../../etc/passwd",
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "absolute path without module context",
			scriptPath: filepath.Join(moduleDir, "run.sh"),
			modulePath: "",
			want:       scriptContent,
		},
		{
			name:       "nonexistent file returns empty",
			scriptPath: "nonexistent.sh",
			modulePath: moduleDir,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := readScriptFileFacts(tt.scriptPath, tt.modulePath).Content
			if got != tt.want {
				t.Errorf("readScriptFileFacts(%q, %q) content = %q, want %q",
					tt.scriptPath, tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestBuildScanContextStandaloneFileScriptUsesInvowkfileDirectory(t *testing.T) {
	root := t.TempDir()
	otherDir := t.TempDir()
	t.Chdir(otherDir)

	scriptDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(scriptDir) error = %v", err)
	}
	scriptContent := "#!/bin/sh\ncurl https://example.test/install.sh | sh"
	if err := os.WriteFile(filepath.Join(scriptDir, "pwn.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	invowkfilePath := filepath.Join(root, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(`cmds: [{
	name: "install"
	implementations: [{
		script: "scripts/pwn.sh"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile) error = %v", err)
	}

	sc, err := BuildScanContext(types.FilesystemPath(invowkfilePath), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() error = %v", err)
	}
	scripts := sc.AllScripts()
	if len(scripts) != 1 {
		t.Fatalf("scripts = %d, want 1", len(scripts))
	}
	if !strings.Contains(scripts[0].Content(), "curl https://example.test/install.sh | sh") {
		t.Fatalf("script content = %q, want standalone file content", scripts[0].Content())
	}
	if scripts[0].ScriptPath != types.FilesystemPath(filepath.Join(scriptDir, "pwn.sh")) {
		t.Fatalf("ScriptPath = %q, want %q", scripts[0].ScriptPath, filepath.Join(scriptDir, "pwn.sh"))
	}
}

func TestBuildScanContextIncludedModuleKeepsLockAndVendoredArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	includeDir := filepath.Join(t.TempDir(), "io.example.included.invowkmod")
	createAuditTestModule(t, includeDir, "io.example.included", "included-cmd")
	vendorDir := filepath.Join(includeDir, invowkmod.VendoredModulesDir, "io.example.dep.invowkmod")
	createAuditTestModule(t, vendorDir, "io.example.dep", "dep-cmd")
	hash, err := invowkmod.ComputeModuleHash(vendorDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules["https://example.com/dep.git"] = invowkmod.LockedModule{
		GitURL:          "https://example.com/dep.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       "io.example.dep",
		ModuleID:        "io.example.dep",
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(includeDir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}
	if writeErr := os.WriteFile(filepath.Join(vendorDir, "tampered.txt"), []byte("changed after lock"), 0o644); writeErr != nil {
		t.Fatalf("write tampered file: %v", writeErr)
	}

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(includeDir)}}
	sc, err := BuildScanContext(types.FilesystemPath(root), cfg, false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	var included *ScannedModule
	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.included" {
			included = mod
			break
		}
	}
	if included == nil {
		t.Fatalf("included module not scanned; modules: %v", sc.Modules())
	}
	if included.LockFile == nil {
		t.Fatal("included LockFile = nil, want loaded lock file")
	}
	if len(included.VendoredModules) != 1 {
		t.Fatalf("included VendoredModules = %d, want 1", len(included.VendoredModules))
	}

	var vendored *ScannedModule
	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.dep" {
			vendored = mod
			break
		}
	}
	if vendored == nil {
		t.Fatalf("vendored module not scanned as first-class surface; modules: %v", sc.Modules())
	}
	if vendored.SurfaceKind != SurfaceKindVendoredModule {
		t.Fatalf("vendored SurfaceKind = %q, want %q", vendored.SurfaceKind, SurfaceKindVendoredModule)
	}

	hasVendoredScript := false
	for _, script := range sc.AllScripts() {
		if script.SurfaceID == "io.example.dep" && script.SurfaceKind == SurfaceKindVendoredModule {
			hasVendoredScript = true
			break
		}
	}
	if !hasVendoredScript {
		t.Fatal("vendored module scripts were not exposed to audit checkers")
	}
}

func TestBuildScanContextWarnsAndIgnoresNestedVendoredModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "io.example.root.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.root", "root-cmd")
	childDir := filepath.Join(moduleDir, invowkmod.VendoredModulesDir, "io.example.child.invowkmod")
	createAuditTestModule(t, childDir, "io.example.child", "child-cmd")
	nestedDir := filepath.Join(childDir, invowkmod.VendoredModulesDir, "io.example.nested.invowkmod")
	createAuditTestModule(t, nestedDir, "io.example.nested", "nested-cmd")

	sc, err := BuildScanContext(types.FilesystemPath(root), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	var foundDiagnostic bool
	for _, diagnostic := range sc.Diagnostics() {
		if diagnostic.Code() == diagnosticNestedVendoredIgnored && diagnostic.Path() == types.FilesystemPath(childDir) {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("missing %s diagnostic; diagnostics=%v", diagnosticNestedVendoredIgnored, sc.Diagnostics())
	}

	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.nested" {
			t.Fatal("nested vendored module was scanned as a first-class audit surface")
		}
	}
	for _, script := range sc.AllScripts() {
		if script.SurfaceID == "io.example.nested" {
			t.Fatal("nested vendored module script was exposed to audit checkers")
		}
	}
}

func TestScanContextModulesReturnsCheckerOwnedSnapshots(t *testing.T) {
	t.Parallel()

	sc := newModuleOnlyContext(&ScannedModule{
		SurfaceID: "root",
		Module: &invowkmod.Module{
			Metadata: &invowkmod.Invowkmod{
				Module:  "io.example.root",
				Version: "1.0.0",
				Requires: []invowkmod.ModuleRequirement{{
					GitURL:  "https://github.com/example/dep.git",
					Version: "1.0.0",
				}},
			},
		},
		Symlinks: []SymlinkRef{{RelPath: "link"}},
	})

	first := sc.Modules()
	first[0].SurfaceID = "mutated"
	first[0].Module.Metadata.Module = "io.example.mutated"
	first[0].Module.Metadata.Requires[0].GitURL = "https://github.com/example/other.git"
	first[0].Symlinks[0].RelPath = "mutated-link"

	second := sc.Modules()
	if second[0].SurfaceID != "root" {
		t.Fatalf("SurfaceID = %q, want root", second[0].SurfaceID)
	}
	if second[0].Module.Metadata.Module != "io.example.root" {
		t.Fatalf("Module = %q, want io.example.root", second[0].Module.Metadata.Module)
	}
	if second[0].Module.Metadata.Requires[0].GitURL != "https://github.com/example/dep.git" {
		t.Fatalf("GitURL = %q, want original dependency", second[0].Module.Metadata.Requires[0].GitURL)
	}
	if second[0].Symlinks[0].RelPath != "link" {
		t.Fatalf("Symlink RelPath = %q, want link", second[0].Symlinks[0].RelPath)
	}
}

func createAuditTestModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "`+moduleID+`"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(`cmds: [{
	name: "`+cmdName+`"
	implementations: [{
		script: "echo test"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}
}
