// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/invowk/invowk/internal/runtime"
)

func virtualInteractiveCommand(ctx context.Context, spec runtime.VirtualInteractiveCommandSpec) (*exec.Cmd, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	invowkPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get invowk executable path: %w", err)
	}

	args := []string{
		"internal", "exec-virtual",
		"--script-file", string(*spec.ScriptFile),
		"--workdir", string(*spec.WorkDir),
		"--env-json", string(spec.EnvJSON),
	}
	if spec.EnableUroot {
		args = append(args, "--enable-uroot")
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
