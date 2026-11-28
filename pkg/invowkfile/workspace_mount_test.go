// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestWorkspaceMountRootValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []WorkspaceMountRoot{"", WorkspaceMountRootInvowkfile, WorkspaceMountRootWorkdir} {
		if err := valid.Validate(); err != nil {
			t.Errorf("WorkspaceMountRoot(%q).Validate() = %v, want nil", valid, err)
		}
	}
	if got := WorkspaceMountRoot("").Effective(); got != WorkspaceMountRootInvowkfile {
		t.Errorf("WorkspaceMountRoot(\"\").Effective() = %q, want %q", got, WorkspaceMountRootInvowkfile)
	}
	err := WorkspaceMountRoot("parent").Validate()
	if !errors.Is(err, ErrInvalidWorkspaceMountRoot) {
		t.Fatalf("Validate(parent) = %v, want ErrInvalidWorkspaceMountRoot", err)
	}
	if _, ok := errors.AsType[*InvalidWorkspaceMountRootError](err); !ok {
		t.Fatalf("error type = %T, want *InvalidWorkspaceMountRootError", err)
	}
}

func TestWorkspaceMountSELinuxLabelValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []WorkspaceMountSELinuxLabel{
		"",
		WorkspaceMountSELinuxAuto,
		WorkspaceMountSELinuxShared,
		WorkspaceMountSELinuxPrivate,
		WorkspaceMountSELinuxNone,
	} {
		if err := valid.Validate(); err != nil {
			t.Errorf("WorkspaceMountSELinuxLabel(%q).Validate() = %v, want nil", valid, err)
		}
	}
	if got := WorkspaceMountSELinuxLabel("").Effective(); got != WorkspaceMountSELinuxAuto {
		t.Errorf("WorkspaceMountSELinuxLabel(\"\").Effective() = %q, want %q", got, WorkspaceMountSELinuxAuto)
	}
	err := WorkspaceMountSELinuxLabel("z").Validate()
	if !errors.Is(err, ErrInvalidWorkspaceMountSELinuxLabel) {
		t.Fatalf("Validate(z) = %v, want ErrInvalidWorkspaceMountSELinuxLabel", err)
	}
	if _, ok := errors.AsType[*InvalidWorkspaceMountSELinuxLabelError](err); !ok {
		t.Fatalf("error type = %T, want *InvalidWorkspaceMountSELinuxLabelError", err)
	}
}

func TestWorkspaceMountConfigValidateAndDefaults(t *testing.T) {
	t.Parallel()

	var nilCfg *WorkspaceMountConfig
	if !nilCfg.IsEnabled() {
		t.Error("nil.IsEnabled() = false, want true")
	}
	if got := nilCfg.EffectiveRoot(); got != WorkspaceMountRootInvowkfile {
		t.Errorf("nil.EffectiveRoot() = %q, want %q", got, WorkspaceMountRootInvowkfile)
	}
	if got := nilCfg.EffectiveSELinuxLabel(); got != WorkspaceMountSELinuxAuto {
		t.Errorf("nil.EffectiveSELinuxLabel() = %q, want %q", got, WorkspaceMountSELinuxAuto)
	}

	falseVal := false
	disabledCfg := &WorkspaceMountConfig{Enabled: &falseVal}
	if disabledCfg.IsEnabled() {
		t.Error("disabled.IsEnabled() = true, want false")
	}

	invalidCfg := WorkspaceMountConfig{
		Root:         "bad-root",
		SELinuxLabel: "bad-label",
	}
	err := invalidCfg.Validate()
	if !errors.Is(err, ErrInvalidWorkspaceMountConfig) {
		t.Fatalf("Validate() = %v, want ErrInvalidWorkspaceMountConfig", err)
	}
	if !errors.Is(err, ErrInvalidWorkspaceMountRoot) || !errors.Is(err, ErrInvalidWorkspaceMountSELinuxLabel) {
		t.Fatalf("Validate() = %v, want both root and selinux_label sentinels", err)
	}

	nonContainerRC := RuntimeConfig{
		Name:           RuntimeNative,
		WorkspaceMount: &WorkspaceMountConfig{Readonly: true},
	}
	nonContainerErr := nonContainerRC.Validate()
	var invalidRCErr *InvalidRuntimeConfigError
	if !errors.As(nonContainerErr, &invalidRCErr) || len(invalidRCErr.FieldErrors) != 1 ||
		!strings.Contains(invalidRCErr.FieldErrors[0].Error(), "workspace_mount is only valid for container runtime") {
		t.Fatalf("non-container RuntimeConfig.Validate() = %v, want workspace_mount error", nonContainerErr)
	}

	containerRC := RuntimeConfig{
		Name:  RuntimeContainer,
		Image: "debian:stable-slim",
		WorkspaceMount: &WorkspaceMountConfig{
			Root:         WorkspaceMountRootWorkdir,
			Readonly:     true,
			SELinuxLabel: WorkspaceMountSELinuxNone,
		},
	}
	if err := containerRC.Validate(); err != nil {
		t.Fatalf("container RuntimeConfig.Validate() = %v, want nil", err)
	}
}

func TestParseAndGenerateCUE_WorkspaceMount(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [{
	name: "build"
	workdir: ".."
	implementations: [{
		script: {content: "pwd"}
		runtimes: [{
			name: "container"
			image: "debian:stable-slim"
			workspace_mount: {
				enabled: true
				root: "workdir"
				readonly: true
				selinux_label: "none"
			}
		}]
		platforms: [{name: "linux"}]
	}]
}]
`
	inv, err := ParseBytes([]byte(cueContent), "invowkfile.cue")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	rc := inv.Commands[0].Implementations[0].Runtimes[0]
	if rc.WorkspaceMount == nil {
		t.Fatal("WorkspaceMount = nil, want populated config")
	}
	if !rc.WorkspaceMount.IsEnabled() || rc.WorkspaceMount.EffectiveRoot() != WorkspaceMountRootWorkdir ||
		!rc.WorkspaceMount.Readonly || rc.WorkspaceMount.EffectiveSELinuxLabel() != WorkspaceMountSELinuxNone {
		t.Fatalf("unexpected decoded WorkspaceMount: %+v", rc.WorkspaceMount)
	}

	generated := GenerateCUE(inv)
	if !strings.Contains(generated, "workspace_mount:") ||
		!strings.Contains(generated, `root: "workdir"`) ||
		!strings.Contains(generated, "readonly: true") ||
		!strings.Contains(generated, `selinux_label: "none"`) {
		t.Fatalf("GenerateCUE() missing workspace_mount fields:\n%s", generated)
	}
}
