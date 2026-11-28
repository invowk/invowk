// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestPrivilegeChecker_Identity(t *testing.T) {
	t.Parallel()

	checker := NewPrivilegeChecker()
	if checker.Name() != privilegeCheckerName {
		t.Errorf("Name() = %q, want %q", checker.Name(), privilegeCheckerName)
	}
	if checker.Category() != CategoryIntegrity {
		t.Errorf("Category() = %q, want %q", checker.Category(), CategoryIntegrity)
	}
	if !slices.ContainsFunc(DefaultCheckers(), func(c Checker) bool {
		return c.Name() == privilegeCheckerName
	}) {
		t.Error("DefaultCheckers() should include PrivilegeChecker")
	}
}

func TestPrivilegeChecker_StandaloneInvowkfileScopesAndEnvFiles(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping POSIX file mode bit test on Windows")
	}

	dir := t.TempDir()
	invPath := filepath.Join(dir, "invowkfile.cue")
	rootEnvPath := filepath.Join(dir, "root.env")
	cmdEnvPath := filepath.Join(dir, "cmd.env")
	implEnvPath := filepath.Join(dir, "impl.env")
	unprivEnvPath := filepath.Join(dir, "unpriv.env")

	for _, p := range []string{rootEnvPath, cmdEnvPath, implEnvPath, unprivEnvPath} {
		if err := os.WriteFile(p, []byte("FOO=bar\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", p, err)
		}
	}
	// Make root.env, cmd.env, impl.env world-writable (0o666), and unpriv.env world-writable too.
	for _, p := range []string{rootEnvPath, cmdEnvPath, implEnvPath, unprivEnvPath} {
		if err := os.Chmod(p, 0o666); err != nil {
			t.Fatalf("Chmod(%s): %v", p, err)
		}
	}

	cueContent := `
env: {
	files: ["root.env", "missing.env?"]
}
cmds: [
	{
		name: "cmd-elevated"
		depends_on: {
			capabilities: [{alternatives: ["elevated"]}]
		}
		env: {files: ["cmd.env"]}
		implementations: [{
			script: {content: "echo cmd"}
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
		}]
	},
	{
		name: "impl-elevated"
		implementations: [{
			script: {content: "echo impl"}
			depends_on: {
				capabilities: [{alternatives: ["elevated"]}]
			}
			env: {files: ["impl.env"]}
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
		}]
	},
	{
		name: "unpriv-cmd"
		depends_on: {
			capabilities: [{alternatives: ["unprivileged"]}]
		}
		env: {files: ["unpriv.env"]}
		implementations: [{
			script: {content: "echo unpriv"}
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
		}]
	},
]
`
	if err := os.WriteFile(invPath, []byte(cueContent), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue): %v", err)
	}
	if err := os.Chmod(invPath, 0o666); err != nil {
		t.Fatalf("Chmod(invowkfile.cue): %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(invPath), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	findings, err := NewPrivilegeChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}

	gotFiles := make(map[string]Finding)
	for _, f := range findings {
		gotFiles[string(f.FilePath)] = f
	}

	for _, expectedPath := range []string{invPath, rootEnvPath, cmdEnvPath, implEnvPath} {
		f, ok := gotFiles[expectedPath]
		if !ok {
			t.Fatalf("expected finding for %s, got findings: %+v", expectedPath, findings)
		}
		if f.Severity != SeverityHigh {
			t.Errorf("%s severity = %s, want high", expectedPath, f.Severity)
		}
		if f.Code != codePrivilegeWorldWritableFile {
			t.Errorf("%s code = %s, want %s", expectedPath, f.Code, codePrivilegeWorldWritableFile)
		}
	}
	if _, ok := gotFiles[unprivEnvPath]; ok {
		t.Errorf("unexpected finding for unprivileged-only env file %s", unprivEnvPath)
	}
	if len(findings) != 4 {
		t.Errorf("findings len = %d, want 4 (deduplicated)", len(findings))
	}
}

func TestPrivilegeChecker_ModuleFilesAndGroupWritabilityUPG(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping POSIX file mode bit test on Windows")
	}

	root := t.TempDir()
	modDir := filepath.Join(root, "io.example.priv.invowkmod")
	scriptsDir := filepath.Join(modDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	invowkmodPath := filepath.Join(modDir, "invowkmod.cue")
	if err := os.WriteFile(invowkmodPath, []byte("module: \"io.example.priv\"\nversion: \"1.0.0\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue): %v", err)
	}

	scriptPath := filepath.Join(scriptsDir, "install.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho install\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(script): %v", err)
	}
	if err := os.Chmod(scriptPath, 0o777); err != nil {
		t.Fatalf("Chmod(script): %v", err)
	}

	envPath := filepath.Join(modDir, ".env")
	if err := os.WriteFile(envPath, []byte("TOKEN=1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(.env): %v", err)
	}
	if err := os.Chmod(envPath, 0o664); err != nil {
		t.Fatalf("Chmod(.env): %v", err)
	}

	invowkfilePath := filepath.Join(modDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(`
depends_on: {
	capabilities: [{alternatives: ["elevated"]}]
}
cmds: [{
	name: "install"
	env: {files: [".env"]}
	implementations: [{
		script: {file: "scripts/install.sh"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue): %v", err)
	}
	if err := os.Chmod(invowkmodPath, 0o664); err != nil {
		t.Fatalf("Chmod(invowkmod.cue): %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(modDir), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	// Case 1: UPG (isSharedGroup = false) — group-writable 0o664 files emit no findings;
	// world-writable 0o777 script emits HIGH.
	upgChecker := newPrivilegeCheckerWithGroupCheck(func(os.FileInfo) bool { return false })
	upgFindings, err := upgChecker.Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("upgChecker.Check() = %v", err)
	}
	if len(upgFindings) != 1 {
		t.Fatalf("UPG findings len = %d, want 1: %+v", len(upgFindings), upgFindings)
	}
	if upgFindings[0].FilePath != types.FilesystemPath(scriptPath) || upgFindings[0].Severity != SeverityHigh {
		t.Fatalf("UPG finding = %+v, want HIGH for %s", upgFindings[0], scriptPath)
	}

	// Case 2: Shared group (isSharedGroup = true) — group-writable 0o664 invowkmod.cue and .env emit MEDIUM,
	// world-writable script emits HIGH.
	sharedChecker := newPrivilegeCheckerWithGroupCheck(func(os.FileInfo) bool { return true })
	sharedFindings, err := sharedChecker.Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("sharedChecker.Check() = %v", err)
	}
	if len(sharedFindings) != 3 {
		t.Fatalf("shared-group findings len = %d, want 3: %+v", len(sharedFindings), sharedFindings)
	}
	byPath := make(map[string]Finding, len(sharedFindings))
	for _, f := range sharedFindings {
		byPath[string(f.FilePath)] = f
	}
	if f, ok := byPath[invowkmodPath]; !ok || f.Severity != SeverityMedium || f.Code != codePrivilegeGroupWritableFile {
		t.Errorf("invowkmod finding = %+v, want MEDIUM %s", f, codePrivilegeGroupWritableFile)
	}
	if f, ok := byPath[envPath]; !ok || f.Severity != SeverityMedium || f.Code != codePrivilegeGroupWritableFile {
		t.Errorf(".env finding = %+v, want MEDIUM %s", f, codePrivilegeGroupWritableFile)
	}
	if f, ok := byPath[scriptPath]; !ok || f.Severity != SeverityHigh || f.Code != codePrivilegeWorldWritableFile {
		t.Errorf("script finding = %+v, want HIGH %s", f, codePrivilegeWorldWritableFile)
	}
	for _, f := range sharedFindings {
		if !strings.Contains(f.Description, "install") {
			t.Errorf("finding description should name elevated command 'install': %q", f.Description)
		}
	}
}

func TestPrivilegeChecker_IgnoresNonElevatedCommands(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping POSIX file mode bit test on Windows")
	}

	dir := t.TempDir()
	invPath := filepath.Join(dir, "invowkfile.cue")
	if err := os.WriteFile(invPath, []byte(`
cmds: [{
	name: "status"
	depends_on: {
		capabilities: [{alternatives: ["unprivileged"]}]
	}
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chmod(invPath, 0o666); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(invPath), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}
	findings, err := NewPrivilegeChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-elevated command, got %+v", findings)
	}
}

func TestPrivilegeChecker_DependsOnHasCapability(t *testing.T) {
	t.Parallel()

	if dependsOnHasCapability(nil, invowkfile.CapabilityElevated) {
		t.Error("nil DependsOn should return false")
	}
	deps := &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY, invowkfile.CapabilityElevated}},
		},
	}
	if !dependsOnHasCapability(deps, invowkfile.CapabilityElevated) {
		t.Error("expected elevated to match when listed in alternatives")
	}
	if dependsOnHasCapability(deps, invowkfile.CapabilityUnprivileged) {
		t.Error("unexpected match for unprivileged")
	}
	// Ensure mod helper is exercised cleanly.
	_ = invowkmod.ModuleSuffix
}

func TestPrivilegeChecker_RootScopeWithoutCommandsFallbackValidate(t *testing.T) {
	t.Parallel()

	invPath := filepath.Join(t.TempDir(), "invowkfile.cue")
	targets := collectElevatedFileTargets(privilegeSurface{
		surfaceID:   invPath,
		surfaceKey:  scanSurfaceKey(SurfaceKindRootInvowkfile, types.FilesystemPath(invPath)),
		surfaceKind: SurfaceKindRootInvowkfile,
		invPath:     types.FilesystemPath(invPath),
		invowkfile: &invowkfile.Invowkfile{
			DependsOn: &invowkfile.DependsOn{
				Capabilities: []invowkfile.CapabilityDependency{
					{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityElevated}},
				},
			},
		},
	})
	if len(targets) != 1 {
		t.Fatalf("targets len = %d, want 1: %+v", len(targets), targets)
	}
	if targets[0].commandLabel != "(root)" {
		t.Errorf("commandLabel = %q, want %q", targets[0].commandLabel, "(root)")
	}
	if err := targets[0].Validate(); err != nil {
		t.Errorf("targets[0].Validate() = %v, want nil", err)
	}
}
