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
