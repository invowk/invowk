// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type virtualFilesystemPathsJSON string

func (p virtualFilesystemPathsJSON) String() string { return string(p) }

func (p virtualFilesystemPathsJSON) Validate() error {
	if !json.Valid([]byte(p)) {
		return fmt.Errorf("invalid virtual filesystem paths JSON: %s", p)
	}
	return nil
}

func shInteractiveCommand(ctx context.Context, spec runtime.ShInteractiveCommandSpec) (*exec.Cmd, error) {
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
		"internal", "exec-virtual-sh",
		"--script-file", string(*spec.ScriptFile),
		"--workdir", string(*spec.WorkDir),
		"--script-base-path", string(*spec.ScriptBasePath),
		"--env-json", string(spec.EnvJSON),
		"--binary-lookup-mode", spec.BinaryLookupMode.String(),
		"--filesystem-access", spec.FilesystemAccess.String(),
		"--filesystem-paths-json", filesystemPathsJSON.String(),
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

func encodeVirtualFilesystemPathsJSON(paths invowkfile.VirtualFilesystemPaths) (virtualFilesystemPathsJSON, error) {
	if len(paths) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(paths.StringMap())
	if err != nil {
		return "", fmt.Errorf("serialize virtual filesystem paths: %w", err)
	}
	return virtualFilesystemPathsJSON(data), nil
}
