// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/types"
)

func TestSELinuxLabelingValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []SELinuxLabeling{"", SELinuxLabelingEnabled, SELinuxLabelingDisabled} {
		if err := valid.Validate(); err != nil {
			t.Errorf("SELinuxLabeling(%q).Validate() = %v, want nil", valid, err)
		}
	}
	if got := SELinuxLabeling("").Effective(); got != SELinuxLabelingEnabled {
		t.Errorf("SELinuxLabeling(\"\").Effective() = %q, want %q", got, SELinuxLabelingEnabled)
	}
	if !SELinuxLabelingDisabled.IsDisabled() || SELinuxLabelingEnabled.IsDisabled() || SELinuxLabeling("").IsDisabled() {
		t.Fatal("IsDisabled() must hold only for disabled")
	}
	err := SELinuxLabeling("permissive").Validate()
	if !errors.Is(err, ErrInvalidSELinuxLabeling) {
		t.Fatalf("Validate(permissive) = %v, want ErrInvalidSELinuxLabeling", err)
	}
}

func TestDefaultConfigSELinuxLabelingIsEnabled(t *testing.T) {
	t.Parallel()

	if got := DefaultConfig().Container.SELinuxLabeling; got != SELinuxLabelingEnabled {
		t.Fatalf("DefaultConfig().Container.SELinuxLabeling = %q, want enabled", got)
	}
}

func TestSELinuxLabelingFromInvowkConfigSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := writeSourceTestFile(t, dir, "env.cue", "container: selinux_labeling: \"disabled\"\n")
	provider := NewProviderWithEnv(envWith(envPath))
	cfg, err := provider.Load(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Container.SELinuxLabeling != SELinuxLabelingDisabled {
		t.Fatalf("SELinuxLabeling = %q, want disabled", cfg.Container.SELinuxLabeling)
	}

	generated := GenerateCUE(cfg)
	if !strings.Contains(generated, `selinux_labeling: "disabled"`) {
		t.Fatalf("GenerateCUE() does not carry selinux_labeling:\n%s", generated)
	}
	roundTrip := writeSourceTestFile(t, dir, "roundtrip.cue", generated)
	cfg2, err := NewProviderWithEnv(envWith(roundTrip)).Load(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err != nil {
		t.Fatalf("Load(round trip): %v", err)
	}
	if cfg2.Container.SELinuxLabeling != SELinuxLabelingDisabled {
		t.Fatalf("round-trip SELinuxLabeling = %q, want disabled", cfg2.Container.SELinuxLabeling)
	}
}

func TestIncludeWorkspaceRootPathValidateAndRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modPath := filepath.Join(tmpDir, "repo", "cst.invowkmod")
	wsRoot := filepath.Join(tmpDir, "repo")

	for _, valid := range []IncludeWorkspaceRootPath{"", IncludeWorkspaceRootPath(wsRoot)} {
		if err := valid.Validate(); err != nil {
			t.Errorf("IncludeWorkspaceRootPath(%q).Validate() = %v, want nil", valid, err)
		}
	}
	for _, invalid := range []IncludeWorkspaceRootPath{"   ", "relative/path", IncludeWorkspaceRootPath(strings.Repeat("a", 4097))} {
		err := invalid.Validate()
		if !errors.Is(err, ErrInvalidIncludeWorkspaceRootPath) {
			t.Errorf("IncludeWorkspaceRootPath(%q).Validate() = %v, want ErrInvalidIncludeWorkspaceRootPath", invalid, err)
		}
	}

	entry := IncludeEntry{
		Path:          ModuleIncludePath(modPath),
		WorkspaceRoot: IncludeWorkspaceRootPath(wsRoot),
	}
	if err := entry.Validate(); err != nil {
		t.Fatalf("IncludeEntry.Validate() = %v, want nil", err)
	}

	cfg := DefaultConfig()
	cfg.Includes = []IncludeEntry{entry}
	generated := GenerateCUE(cfg)
	if !strings.Contains(generated, "workspace_root:") {
		t.Fatalf("GenerateCUE() missing workspace_root:\n%s", generated)
	}
}

func TestIncludeWorkspaceRootPath_Validate_Matrix(t *testing.T) {
	t.Parallel()

	hostAbsolute := func(input string) pathmatrix.Outcome {
		return pathmatrix.Custom(func(t testing.TB, _ string, gotErr error) {
			t.Helper()
			if filepath.IsAbs(input) && gotErr != nil {
				t.Fatalf("host-absolute path %q rejected: %v", input, gotErr)
			}
			if !filepath.IsAbs(input) && !errors.Is(gotErr, ErrInvalidIncludeWorkspaceRootPath) {
				t.Fatalf("host-relative path %q error = %v, want ErrInvalidIncludeWorkspaceRootPath", input, gotErr)
			}
		})
	}
	rejectRelative := pathmatrix.RejectIs(ErrInvalidIncludeWorkspaceRootPath)
	pathmatrix.Validator(t, func(input string) error {
		return IncludeWorkspaceRootPath(input).Validate()
	}, pathmatrix.Expectations{
		UnixAbsolute:       hostAbsolute(pathmatrix.InputUnixAbsolute),
		WindowsDriveAbs:    hostAbsolute(pathmatrix.InputWindowsDriveAbs),
		WindowsRooted:      hostAbsolute(pathmatrix.InputWindowsRooted),
		UNC:                hostAbsolute(pathmatrix.InputUNC),
		SlashTraversal:     rejectRelative,
		BackslashTraversal: rejectRelative,
		ValidRelative:      rejectRelative,
	})
}
