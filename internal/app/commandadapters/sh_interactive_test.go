// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestShInteractiveCommandPassesVirtualPolicyFlags(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptFile := types.FilesystemPath(filepath.Join(tmpDir, "invowk-script.sh"))
	workDir := types.FilesystemPath(tmpDir)
	scriptBasePath := types.FilesystemPath(filepath.Join(tmpDir, "module"))
	cmd, err := shInteractiveCommand(t.Context(), runtime.ShInteractiveCommandSpec{
		ScriptFile:       &scriptFile,
		WorkDir:          &workDir,
		ScriptBasePath:   &scriptBasePath,
		EnvJSON:          runtime.ShInteractiveEnvJSON("{}"),
		Args:             runtime.ShInteractiveArgs{"positional"},
		AllowedBinaries:  []string{"tool", "/bin/echo"},
		BinaryLookupMode: invowkfile.BinaryLookupModeStrict,
		FilesystemAccess: invowkfile.VirtualFilesystemAccessFull,
		FilesystemPaths:  invowkfile.VirtualFilesystemPaths{"CACHE": "@cache/reports"},
		EnableUroot:      true,
	})
	if err != nil {
		t.Fatalf("shInteractiveCommand() error = %v", err)
	}

	args := cmd.Args
	for _, want := range []string{
		"internal",
		"exec-virtual-sh",
		"--script-base-path",
		string(scriptBasePath),
		"--binary-lookup-mode",
		invowkfile.BinaryLookupModeStrict.String(),
		"--filesystem-access",
		invowkfile.VirtualFilesystemAccessFull.String(),
		"--filesystem-paths-json",
		`{"CACHE":"@cache/reports"}`,
		"--allowed-binary",
		"tool",
		"/bin/echo",
		"--enable-uroot",
		"--args",
		"positional",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("args = %v, missing %q", args, want)
		}
	}
}

func TestLuaInteractiveCommandPassesVirtualPolicyFlags(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptFile := types.FilesystemPath(filepath.Join(tmpDir, "invowk-script.lua"))
	workDir := types.FilesystemPath(tmpDir)
	scriptBasePath := types.FilesystemPath(filepath.Join(tmpDir, "module"))
	cmd, err := luaInteractiveCommand(t.Context(), runtime.LuaInteractiveCommandSpec{
		ScriptFile:       &scriptFile,
		WorkDir:          &workDir,
		ScriptBasePath:   &scriptBasePath,
		EnvJSON:          runtime.LuaInteractiveEnvJSON("{}"),
		Args:             runtime.LuaInteractiveArgs{"positional"},
		AllowedBinaries:  []string{"tool", "/bin/echo"},
		BinaryLookupMode: invowkfile.BinaryLookupModeStrict,
		FilesystemAccess: invowkfile.VirtualFilesystemAccessFull,
		FilesystemPaths:  invowkfile.VirtualFilesystemPaths{"CACHE": "@cache/reports"},
		CPULimit:         123,
		MemoryLimit:      "1M",
		EnableUroot:      true,
	})
	if err != nil {
		t.Fatalf("luaInteractiveCommand() error = %v", err)
	}

	args := cmd.Args
	for _, want := range []string{
		"internal",
		"exec-virtual-lua",
		"--script-base-path",
		string(scriptBasePath),
		"--binary-lookup-mode",
		invowkfile.BinaryLookupModeStrict.String(),
		"--filesystem-access",
		invowkfile.VirtualFilesystemAccessFull.String(),
		"--filesystem-paths-json",
		`{"CACHE":"@cache/reports"}`,
		"--cpu-limit",
		"123",
		"--memory-limit",
		"1M",
		"--allowed-binary",
		"tool",
		"/bin/echo",
		"--enable-uroot",
		"--args",
		"positional",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("args = %v, missing %q", args, want)
		}
	}
}

// TestInteractiveCommandsPassWorkdirModuleRootOnlyWhenSet checks that both
// adapters pass the hidden --workdir-module-root flag exactly when the spec
// carries a module root (nil means a trusted workdir).
func TestInteractiveCommandsPassWorkdirModuleRootOnlyWhenSet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptFile := types.FilesystemPath(filepath.Join(tmpDir, "invowk-script"))
	workDir := types.FilesystemPath(filepath.Join(tmpDir, "shared"))
	moduleRoot := types.FilesystemPath(filepath.Join(tmpDir, "mod.invowkmod"))
	for _, root := range []*types.FilesystemPath{&moduleRoot, nil} {
		shCmd, err := shInteractiveCommand(t.Context(), runtime.ShInteractiveCommandSpec{
			ScriptFile:        &scriptFile,
			WorkDir:           &workDir,
			ScriptBasePath:    &moduleRoot,
			WorkDirModuleRoot: root,
			EnvJSON:           runtime.ShInteractiveEnvJSON("{}"),
		})
		if err != nil {
			t.Fatalf("shInteractiveCommand() error = %v", err)
		}
		luaCmd, err := luaInteractiveCommand(t.Context(), runtime.LuaInteractiveCommandSpec{
			ScriptFile:        &scriptFile,
			WorkDir:           &workDir,
			ScriptBasePath:    &moduleRoot,
			WorkDirModuleRoot: root,
			EnvJSON:           runtime.LuaInteractiveEnvJSON("{}"),
		})
		if err != nil {
			t.Fatalf("luaInteractiveCommand() error = %v", err)
		}
		for _, args := range [][]string{shCmd.Args, luaCmd.Args} {
			i := slices.Index(args, "--workdir-module-root")
			switch {
			case root == nil && i >= 0:
				t.Fatalf("args = %v, want no --workdir-module-root for a trusted workdir", args)
			case root != nil && (i < 0 || i+1 >= len(args) || args[i+1] != string(moduleRoot)):
				t.Fatalf("args = %v, want --workdir-module-root %s", args, moduleRoot)
			}
		}
	}
}
