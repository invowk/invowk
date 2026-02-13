// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"

	"github.com/u-root/u-root/pkg/core"
)

// baseWrapper provides common functionality for pkg/core wrappers.
type baseWrapper struct {
	name  string
	flags []FlagInfo
}

// Name returns the command name.
func (w *baseWrapper) Name() string {
	return w.name
}

// SupportedFlags returns the flags supported by this command.
func (w *baseWrapper) SupportedFlags() []FlagInfo {
	return w.flags
}

// nativePreprocessor marks baseWrapper (and all upstream wrappers that embed it)
// as commands handling POSIX flag preprocessing internally via unixflag.ArgsToGoArgs
// in their RunContext method. Registry.Run() skips centralized preprocessing for
// these commands to avoid double-splitting (e.g., --recursive → -r -e -c -u ...).
func (w *baseWrapper) nativePreprocessor() {}

// configureCommand configures a u-root core.Command with the handler context.
// This is the common setup for all pkg/core wrapper commands.
func configureCommand(ctx context.Context, cmd core.Command) {
	hc := GetHandlerContext(ctx)
	cmd.SetIO(hc.Stdin, hc.Stdout, hc.Stderr)
	cmd.SetWorkingDir(hc.Dir)
	cmd.SetLookupEnv(hc.LookupEnv)
}

// wrapError wraps an error with the [uroot] prefix format.
// The [uroot] prefix identifies errors from virtual shell built-in utilities,
// distinguishing them from host shell errors and system-level failures in
// mixed execution output. Returns nil if err is nil.
func wrapError(cmdName string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("[uroot] %s: %w", cmdName, err)
}
