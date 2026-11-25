// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func includeConfig(path, alias string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(path), Alias: invowkmod.ModuleAlias(alias)}}
	return cfg
}

func requireLoadedOnce(t *testing.T, leg string, out cwdDiscoveryOutcome, name string) *DiscoveredFile {
	t.Helper()
	if out.err != nil {
		t.Fatalf("%s: discovery error: %v", leg, out.err)
	}
	if got := out.moduleNames(); !slices.Equal(got, []string{name}) {
		t.Fatalf("%s: modules = %v, want [%s] once", leg, got, name)
	}
	return out.module(name)
}

func TestDiscovery_ModuleDedupe_CurrentModuleAlsoIncluded(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	cfg := includeConfig(moduleDir, "")

	for leg, base := range map[string]string{"parent (sibling module)": parent, "module dir": moduleDir} {
		requireLoadedOnce(t, leg, discoverFromBase(t, cfg, base), "tools")
	}
}

func TestDiscovery_ModuleDedupe_CaseVariantCurrentModuleAlsoIncluded(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	variant := filepath.Join(parent, "TOOLS.INVOWKMOD")
	if _, err := os.Lstat(variant); err != nil {
		t.Skipf("case-sensitive volume: %v", err)
	}

	requireLoadedOnce(t, "case variant", discoverFromBase(t, includeConfig(moduleDir, ""), variant), "tools")
}

func TestDiscovery_ModuleDedupe_IncludeSpelledThroughLink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	root := fstree.TempRoot(t)
	parent := filepath.Join(root, "real")
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	linkedParent := filepath.Join(root, "link")
	fstree.MustSymlink(t, parent, linkedParent)
	linkedModule := filepath.Join(linkedParent, "tools.invowkmod")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		requireLoadedOnce(t, leg, discoverFromBase(t, includeConfig(linkedModule, ""), base), "tools")
	}

	// The include alias of the link spelling still applies to the module
	// loaded from the current directory.
	set := commandSetFromBase(t, includeConfig(linkedModule, "t"), moduleDir)
	if len(set.Commands) != 1 || set.Commands[0].SourceID != "t" {
		t.Fatalf("commands = %v, want hello from source t", set.Commands)
	}
}

func TestDiscovery_ModuleDedupe_IncludeAliasFromInside(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	cfg := includeConfig(moduleDir, "t")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		requireLoadedOnce(t, leg, discoverFromBase(t, cfg, base), "tools")
		set := commandSetFromBase(t, cfg, base)
		if len(set.Commands) != 1 || set.Commands[0].SourceID != "t" || set.Commands[0].Name != "t hello" {
			t.Fatalf("%s: commands = %v, want \"t hello\" from source t", leg, set.Commands)
		}
	}
}

func TestDiscovery_ModuleDedupe_CurrentModuleInsideCommandsDir(t *testing.T) {
	t.Parallel()

	cmdsDir := filepath.Join(t.TempDir(), "cmds")
	moduleDir := filepath.Join(cmdsDir, "g.invowkmod")
	createTestModule(t, moduleDir, "g", "ghello")
	createVendoredModule(t, moduleDir, "vend.invowkmod", "vend", "vhello")

	out := discoverFromBase(t, nil, moduleDir, WithCommandsDir(types.FilesystemPath(cmdsDir)))
	if out.err != nil {
		t.Fatalf("discovery error: %v", out.err)
	}
	if got := out.moduleNames(); !slices.Equal(got, []string{"g", "vend"}) {
		t.Fatalf("modules = %v, want [g vend] once each", got)
	}
	for _, name := range []string{"g", "vend"} {
		if !out.module(name).IsGlobalModule {
			t.Errorf("%s: IsGlobalModule = false, want true", name)
		}
	}
	if out.hasCode(CodeModuleShadowsGlobal) {
		t.Fatalf("unexpected %s: %v", CodeModuleShadowsGlobal, out.diags)
	}
}

func TestDiscovery_ModuleDedupe_DifferentDirectoriesStillCollide(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	cmdsDir := filepath.Join(t.TempDir(), "cmds")
	createTestModule(t, filepath.Join(cmdsDir, "tools.invowkmod"), "tools", "ghello")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		out := discoverFromBase(t, nil, base, WithCommandsDir(types.FilesystemPath(cmdsDir)))
		if _, ok := errors.AsType[*ModuleCollisionError](out.err); !ok {
			t.Fatalf("%s: error = %v, want ModuleCollisionError", leg, out.err)
		}
	}
}
