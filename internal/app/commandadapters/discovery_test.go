// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/provisionenv"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

type staticDiscoveryConfigLoader struct {
	cfg *config.Config
}

func (l *staticDiscoveryConfigLoader) Load(context.Context, config.LoadOptions) (*config.Config, error) {
	return l.cfg, nil
}

func TestDiscoveryServiceReadsProvisionedModuleManifestAtAdapterBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	t.Chdir(workDir)

	sharedModule := createCommandAdapterTestModule(t, filepath.Join(tmpDir, "first"), "shared.invowkmod", "shared", "run")
	aliasedModule := createCommandAdapterTestModule(t, filepath.Join(tmpDir, "second"), "shared.invowkmod", "shared", "run")

	manifest := provisionenv.Entries{
		{
			Path:             container.MountTargetPath(sharedModule),
			CommandNamespace: invowkmod.ModuleNamespace("shared"),
		},
		{
			Path:             container.MountTargetPath(aliasedModule),
			CommandNamespace: invowkmod.ModuleNamespace("aliased"),
		},
	}
	manifestValue, err := provisionenv.MarshalManifest(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	t.Setenv(provisionenv.ModuleManifestName.String(), manifestValue.String())

	svc := NewDiscoveryService(&staticDiscoveryConfigLoader{cfg: config.DefaultConfig()})
	result, err := svc.DiscoverCommandSet(ContextWithDiscoveryRequestCache(t.Context()))
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}
	if result.Set == nil {
		t.Fatal("DiscoverCommandSet() returned nil set")
	}
	if _, ok := result.Set.ByName[invowkfile.CommandName("shared run")]; !ok {
		t.Fatalf("missing command from first provisioned module; names: %v", result.Set.ByName)
	}
	if _, ok := result.Set.ByName[invowkfile.CommandName("aliased run")]; !ok {
		t.Fatalf("missing command from aliased provisioned module; names: %v", result.Set.ByName)
	}
}

func TestDiscoveryServiceDoesNotFallbackWhenProvisionedManifestIsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	t.Chdir(workDir)

	moduleRoot := filepath.Join(tmpDir, "modules")
	createCommandAdapterTestModule(t, moduleRoot, "fallback.invowkmod", "fallback", "run")
	t.Setenv(provisionenv.ModuleManifestName.String(), "{not-json")
	t.Setenv(provisionenv.ModulePathName.String(), moduleRoot)

	svc := NewDiscoveryService(&staticDiscoveryConfigLoader{cfg: config.DefaultConfig()})
	result, err := svc.DiscoverCommandSet(ContextWithDiscoveryRequestCache(t.Context()))
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}
	if result.Set != nil {
		if _, ok := result.Set.ByName[invowkfile.CommandName("fallback run")]; ok {
			t.Fatalf("invalid manifest fell back to path discovery; names: %v", result.Set.ByName)
		}
	}
	for _, diag := range result.Diagnostics {
		if diag.Code() == discovery.CodeProvisionedModuleManifestInvalid {
			return
		}
	}
	t.Fatalf("missing %s diagnostic; got %#v", discovery.CodeProvisionedModuleManifestInvalid, result.Diagnostics)
}

func createCommandAdapterTestModule(t *testing.T, parentDir, folderName, moduleID, cmdName string) string {
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
