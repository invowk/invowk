// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestContainerMountChecker_Identity(t *testing.T) {
	t.Parallel()

	checker := NewContainerMountChecker()
	if checker.Name() != containerMountCheckerName {
		t.Errorf("Name() = %q, want %q", checker.Name(), containerMountCheckerName)
	}
	if checker.Category() != CategoryPathTraversal {
		t.Errorf("Category() = %q, want %q", checker.Category(), CategoryPathTraversal)
	}
	if !slices.ContainsFunc(DefaultCheckers(), func(c Checker) bool {
		return c.Name() == containerMountCheckerName
	}) {
		t.Error("DefaultCheckers() should include ContainerMountChecker")
	}
}

func TestContainerMountChecker_DetectsModuleMountAndSELinuxRisks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modDir := filepath.Join(root, "io.example.risky.invowkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkmod.cue"), []byte(`module: "io.example.risky"
version: "1.0.0"`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue): %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkfile.cue"), []byte(`cmds: [
	{
		name: "escape"
		workdir: "../../outside"
		implementations: [{
			script: { content: "pwd" }
			runtimes: [{
				name: "container"
				image: "debian:stable-slim"
				volumes: ["/host/secret:/secret:ro", "../sibling:/sib", "named-vol:/cache"]
				workspace_mount: {
					root: "workdir"
					selinux_label: "none"
				}
			}]
			platforms: [{name: "linux"}]
		}]
	},
]`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue): %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(root), nil, false)
	if err != nil {
		t.Fatalf("BuildScanContext(): %v", err)
	}

	findings, err := NewContainerMountChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}

	gotCodes := make(map[FindingCode]bool, len(findings))
	for _, f := range findings {
		gotCodes[f.Code] = true
	}

	for _, want := range []FindingCode{
		codeContainerModuleAbsoluteVolume,
		codeContainerMountParentTraversal,
		codeContainerWorkdirRootTraversal,
		codeContainerSELinuxLabelNone,
	} {
		if !gotCodes[want] {
			t.Errorf("missing expected finding code %q in %+v", want, findings)
		}
	}
}

func TestContainerMountChecker_StandaloneRootAbsoluteVolumeNotFlaggedAsModuleError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invowkfile.cue"), []byte(`cmds: [
	{
		name: "host-vol"
		implementations: [{
			script: { content: "ls /data" }
			runtimes: [{
				name: "container"
				image: "debian:stable-slim"
				volumes: ["/tmp/data:/data:ro", "cache-vol:/cache"]
			}]
			platforms: [{name: "linux"}]
		}]
	},
]`), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(root), nil, false)
	if err != nil {
		t.Fatalf("BuildScanContext(): %v", err)
	}

	findings, err := NewContainerMountChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("unexpected findings for standalone invowkfile: %+v", findings)
	}
}

func TestContainerMountChecker_StandaloneSensitiveHostMounts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invowkfile.cue"), []byte(`cmds: [
	{
		name: "sensitive"
		implementations: [{
			script: {content: "ls"}
			runtimes: [{
				name: "container"
				image: "debian:stable-slim"
				volumes: [
					"/:/host:ro",
					"/etc:/host-etc:ro",
					"/var/run:/host-var-run:ro",
					"/run/podman:/run/podman",
					"/root:/host-root",
					"~/.ssh:/root/.ssh:ro",
					"/tmp/benign:/benign:ro",
					"named-cache:/cache",
				]
			}]
			platforms: [{name: "linux"}]
		}]
	},
]`), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(root), nil, false)
	if err != nil {
		t.Fatalf("BuildScanContext(): %v", err)
	}
	if invs := sc.Invowkfiles(); len(invs) != 1 || invs[0].ParseErr != nil {
		t.Fatalf("expected valid parsed standalone invowkfile, got invs=%+v diags=%+v", invs, sc.Diagnostics())
	}

	findings, err := NewContainerMountChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	if len(findings) != 6 {
		t.Fatalf("findings count = %d, want 6 (excluding /tmp/benign and named-cache): %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Code != codeContainerStandaloneSensitiveMount {
			t.Errorf("finding code = %q, want %q", f.Code, codeContainerStandaloneSensitiveMount)
		}
		if f.Severity != SeverityHigh {
			t.Errorf("finding severity = %q, want %q", f.Severity, SeverityHigh)
		}
	}

	// Also verify helper coverage on sensitive system paths that CUE schema rejects directly.
	for _, src := range []string{"/proc", "/sys", "/dev", "/var/run/docker.sock", "/home/alice/.ssh"} {
		if !isStandaloneSensitiveVolumeSource(src) {
			t.Errorf("isStandaloneSensitiveVolumeSource(%q) = false, want true", src)
		}
	}
	for _, benign := range []string{"/tmp/benign", "./data", "named-cache", "/opt/project"} {
		if isStandaloneSensitiveVolumeSource(benign) {
			t.Errorf("isStandaloneSensitiveVolumeSource(%q) = true, want false", benign)
		}
	}
}
