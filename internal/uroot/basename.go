// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"
	"path"
	"strings"
)

// basenameCommand implements the basename utility.
// It extracts the filename component from a path, optionally stripping a suffix.
type basenameCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newBasenameCommand())
}

// newBasenameCommand creates a new basename command.
func newBasenameCommand() *basenameCommand {
	return &basenameCommand{
		name:  "basename",
		flags: nil, // No flags, only positional args
	}
}

// Name returns the command name.
func (c *basenameCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *basenameCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the basename command.
// Usage: basename PATH [SUFFIX]
func (c *basenameCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	posArgs := args[1:]
	if len(posArgs) == 0 {
		return wrapError(c.name, fmt.Errorf("missing operand"))
	}

	base := path.Base(posArgs[0])

	// Strip suffix if provided and the result would be non-empty
	if len(posArgs) > 1 {
		suffix := posArgs[1]
		if suffix != "" && strings.HasSuffix(base, suffix) && base != suffix {
			base = base[:len(base)-len(suffix)]
		}
	}

	fmt.Fprintln(hc.Stdout, base)
	return nil
}
