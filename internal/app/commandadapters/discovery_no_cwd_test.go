// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestNoCwdKeepsProvisionedModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	invowkfileContent := `cmds: [{name: "local", implementations: [{script: {content: "echo local"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`
	if err := os.WriteFile(filepath.Join(cwd, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}
	provisioned := createCommandAdapterTestModule(t, filepath.Join(root, "prov"), "shared.invowkmod", "shared", "run")
	globalProvisioned := createCommandAdapterTestModule(t, filepath.Join(root, "global"), "glob.invowkmod", "glob", "run")

	baseDir := types.FilesystemPath(cwd)
	commandsDir := types.FilesystemPath("")
	svc := &DiscoveryService{
		config:      &staticDiscoveryConfigLoader{cfg: config.DefaultConfig()},
		baseDir:     &baseDir,
		commandsDir: &commandsDir,
		provisionedModules: discovery.ProvisionedModuleEntries{{
			Path:             types.FilesystemPath(provisioned),
			CommandNamespace: invowkmod.ModuleNamespace("shared"),
		}},
		provisionedGlobalModules: discovery.ProvisionedModuleEntries{{
			Path:             types.FilesystemPath(globalProvisioned),
			CommandNamespace: invowkmod.ModuleNamespace("glob"),
		}},
	}

	withCwd, err := svc.DiscoverCommandSet(ContextWithConfigPath(t.Context(), ""))
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}
	if _, ok := withCwd.Set.ByName[invowkfile.CommandName("local")]; !ok {
		t.Fatalf("cwd command missing without the switch: %v", withCwd.Set.ByName)
	}

	noCwd, err := svc.DiscoverCommandSet(ContextWithConfigPath(ContextWithNoCwd(t.Context(), true), ""))
	if err != nil {
		t.Fatalf("DiscoverCommandSet() under no-cwd error = %v", err)
	}
	if _, ok := noCwd.Set.ByName[invowkfile.CommandName("local")]; ok {
		t.Fatal("no-cwd discovery still found the cwd invowkfile command")
	}
	for _, name := range []invowkfile.CommandName{"shared run", "glob run"} {
		if _, ok := noCwd.Set.ByName[name]; !ok {
			t.Fatalf("no-cwd discovery lost provisioned command %q: %v", name, noCwd.Set.ByName)
		}
	}
}

func TestDiscoveryRequestCacheReuseMatchesNoCwd(t *testing.T) {
	t.Parallel()

	cwdCtx := ContextWithDiscoveryRequestCache(ContextWithNoCwd(t.Context(), false))
	cwdCache := discoveryCacheFromContext(cwdCtx)

	if discoveryCacheFromContext(ContextWithDiscoveryRequestCache(cwdCtx)) != cwdCache {
		t.Fatal("same no-cwd value must reuse the parent cache")
	}
	noCwdCtx := ContextWithDiscoveryRequestCache(ContextWithNoCwd(cwdCtx, true))
	noCwdCache := discoveryCacheFromContext(noCwdCtx)
	if noCwdCache == cwdCache || !noCwdCache.noCwd {
		t.Fatal("a no-cwd context must get a fresh cache, not the cwd-scoped parent cache")
	}
	if !NoCwdFromContext(noCwdCtx) || NoCwdFromContext(cwdCtx) || NoCwdFromContext(t.Context()) {
		t.Fatal("NoCwdFromContext() must reflect the attached value and default to false")
	}
}
