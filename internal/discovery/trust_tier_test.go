// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

func TestSourceTrustTierValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []SourceTrustTier{"", SourceTrustTierContained, SourceTrustTierLocalCwd, SourceTrustTierInclude} {
		if err := valid.Validate(); err != nil {
			t.Errorf("SourceTrustTier(%q).Validate() = %v, want nil", valid, err)
		}
	}
	if err := SourceTrustTier("untrusted").Validate(); !errors.Is(err, ErrInvalidSourceTrustTier) {
		t.Fatalf("Validate(untrusted) = %v, want ErrInvalidSourceTrustTier", err)
	}
}

func TestDiscoverCommandsAssignsThreeTierTrustAndCeilings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "project")
	includeBase := filepath.Join(root, "shared")
	includeModDir := filepath.Join(includeBase, "mods", "org.include.invowkmod")
	cwdModDir := filepath.Join(cwd, "local.mod.invowkmod")
	vendoredModDir := filepath.Join(cwdModDir, "invowk_modules", "org.vendor.invowkmod")

	globalDir := t.TempDir()
	globalModDir := filepath.Join(globalDir, "org.global.invowkmod")

	for _, dir := range []string{cwd, includeModDir, cwdModDir, vendoredModDir, globalModDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	writeTestFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	writeTestFile(filepath.Join(cwd, "invowkfile.cue"), `cmds: [{ name: "rootcmd", implementations: [{ script: { content: "echo root" }, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}] }] }]`)
	writeTestFile(filepath.Join(cwdModDir, "invowkmod.cue"), `
module: "local.mod"
version: "1.0.0"
`)
	writeTestFile(filepath.Join(cwdModDir, "invowkfile.cue"), `cmds: [{ name: "localcmd", implementations: [{ script: { content: "echo local" }, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}] }] }]`)
	writeTestFile(filepath.Join(vendoredModDir, "invowkmod.cue"), `module: "org.vendor"
version: "1.0.0"`)
	writeTestFile(filepath.Join(vendoredModDir, "invowkfile.cue"), `cmds: [{ name: "vendorcmd", implementations: [{ script: { content: "echo vendor" }, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}] }] }]`)
	declareVendoredModule(t, cwdModDir, vendoredModDir, "org.vendor")
	writeTestFile(filepath.Join(includeModDir, "invowkmod.cue"), `module: "org.include"
version: "1.0.0"`)
	writeTestFile(filepath.Join(includeModDir, "invowkfile.cue"), `cmds: [{ name: "includecmd", implementations: [{ script: { content: "echo inc" }, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}] }] }]`)
	writeTestFile(filepath.Join(globalModDir, "invowkmod.cue"), `module: "org.global"
version: "1.0.0"`)
	writeTestFile(filepath.Join(globalModDir, "invowkfile.cue"), `cmds: [{ name: "globalcmd", implementations: [{ script: { content: "echo global" }, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}] }] }]`)

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{
		Path:          config.ModuleIncludePath(includeModDir),
		WorkspaceRoot: config.IncludeWorkspaceRootPath(includeBase),
	}}

	disc := New(cfg, WithBaseDir(types.FilesystemPath(cwd)), WithCommandsDir(types.FilesystemPath(globalDir)))
	files, err := disc.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll(): %v", err)
	}
	var sawVendored bool
	for _, f := range files {
		if f.ParentModule != nil && f.Module != nil && f.Module.Metadata.Module == "org.vendor" {
			sawVendored = true
			if f.TrustTier != SourceTrustTierContained || f.MountCeiling != "" {
				t.Fatalf("vendored file trust=(%q, %q), want (%q, \"\")", f.TrustTier, f.MountCeiling, SourceTrustTierContained)
			}
		}
	}
	if !sawVendored {
		t.Fatal("DiscoverAll() did not discover vendored module")
	}

	result, err := disc.DiscoverAndValidateCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverAndValidateCommandSet(): %v", err)
	}

	byName := make(map[string]*CommandInfo, len(result.Set.Commands))
	for _, cmd := range result.Set.Commands {
		byName[string(cmd.SimpleName)] = cmd
	}

	rootCmd := byName["rootcmd"]
	if rootCmd == nil {
		t.Fatalf("missing rootcmd in discovered commands (%d), diags=%v", len(result.Set.Commands), result.Diagnostics)
	}
	if rootCmd.TrustTier != SourceTrustTierLocalCwd || string(rootCmd.MountCeiling) != cwd {
		t.Fatalf("rootcmd trust=(%q, %q), want (%q, %q)", rootCmd.TrustTier, rootCmd.MountCeiling, SourceTrustTierLocalCwd, cwd)
	}

	localCmd := byName["localcmd"]
	if localCmd == nil {
		t.Fatalf("missing localcmd in discovered commands (%d)", len(result.Set.Commands))
	}
	if localCmd.TrustTier != SourceTrustTierLocalCwd || string(localCmd.MountCeiling) != cwd {
		t.Fatalf("localcmd trust=(%q, %q), want (%q, %q)", localCmd.TrustTier, localCmd.MountCeiling, SourceTrustTierLocalCwd, cwd)
	}

	globalCmd := byName["globalcmd"]
	if globalCmd == nil {
		t.Fatalf("missing globalcmd in discovered commands (%d)", len(result.Set.Commands))
	}
	if globalCmd.TrustTier != SourceTrustTierContained || globalCmd.MountCeiling != "" {
		t.Fatalf("globalcmd trust=(%q, %q), want (%q, \"\")", globalCmd.TrustTier, globalCmd.MountCeiling, SourceTrustTierContained)
	}

	includeCmd := byName["includecmd"]
	if includeCmd == nil {
		t.Fatalf("missing includecmd in discovered commands (%d)", len(result.Set.Commands))
	}
	if includeCmd.TrustTier != SourceTrustTierInclude || string(includeCmd.MountCeiling) != includeBase {
		t.Fatalf("includecmd trust=(%q, %q), want (%q, %q)", includeCmd.TrustTier, includeCmd.MountCeiling, SourceTrustTierInclude, includeBase)
	}
}
