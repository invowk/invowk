// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provisionenv"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestInternalCheckCmdUsesAppDiscoveryProvisionedManifest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Cleanup(testutil.SetHomeDir(t, tmpDir))

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	t.Chdir(workDir)

	sharedModule := createInternalCheckCmdTestModule(t, filepath.Join(tmpDir, "first"), "shared.invowkmod", "shared", "run")
	aliasedModule := createInternalCheckCmdTestModule(t, filepath.Join(tmpDir, "second"), "shared.invowkmod", "shared", "run")
	manifestValue, err := provisionenv.MarshalManifest(provisionenv.Entries{
		{
			Path:             container.MountTargetPath(sharedModule),
			CommandNamespace: invowkmod.ModuleNamespace("shared"),
		},
		{
			Path:             container.MountTargetPath(aliasedModule),
			CommandNamespace: invowkmod.ModuleNamespace("aliased"),
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	t.Setenv(provisionenv.ModuleManifestName.String(), manifestValue.String())

	provider := &fixedConfigProvider{cfg: config.DefaultConfig()}
	app := &App{
		Config:    provider,
		Discovery: commandadapters.NewDiscoveryService(provider),
		stdout:    io.Discard,
		stderr:    io.Discard,
	}

	cmd := newInternalCheckCmdCommand(app, &rootFlagValues{})
	cmd.SetArgs([]string{"aliased run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("internal check-cmd aliased run error = %v", err)
	}
}

func createInternalCheckCmdTestModule(t *testing.T, parentDir, folderName, moduleID, cmdName string) string {
	t.Helper()
	moduleDir := filepath.Join(parentDir, folderName)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module dir: %v", err)
	}
	invowkmodContent := `module: "` + moduleID + `"
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}
	invowkfileContent := `cmds: [{name: "` + cmdName + `", implementations: [{script: {content: "echo test"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}
	return moduleDir
}
