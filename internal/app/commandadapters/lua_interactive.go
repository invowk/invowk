// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/invowk/invowk/internal/runtime"
)

func luaInteractiveCommand(ctx context.Context, spec runtime.LuaInteractiveCommandSpec) (*exec.Cmd, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	invowkPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get invowk executable path: %w", err)
	}
	filesystemPathsJSON, err := encodeVirtualFilesystemPathsJSON(spec.FilesystemPaths)
	if err != nil {
		return nil, err
	}

	args := []string{
		"internal", "exec-virtual-lua",
		"--script-file", string(*spec.ScriptFile),
		"--workdir", string(*spec.WorkDir),
		"--script-base-path", string(*spec.ScriptBasePath),
		"--env-json", string(spec.EnvJSON),
		"--binary-lookup-mode", spec.BinaryLookupMode.String(),
		"--filesystem-access", spec.FilesystemAccess.String(),
		"--filesystem-paths-json", filesystemPathsJSON.String(),
		"--cpu-limit", fmt.Sprintf("%d", spec.CPULimit),
		"--memory-limit", spec.MemoryLimit.String(),
	}
	if spec.EnableUroot {
		args = append(args, "--enable-uroot")
	}
	for _, allowed := range spec.AllowedBinaries {
		args = append(args, "--allowed-binary", allowed)
	}
	for _, arg := range spec.Args {
		args = append(args, "--args", arg)
	}

	cmd := exec.CommandContext(ctx, invowkPath, args...)
	// Subprocess inherits filtered environment; script-visible TUI vars are passed
	// through the serialized environment above.
	cmd.Env = runtime.FilterInvowkEnvVars(os.Environ())
	return cmd, nil
}
