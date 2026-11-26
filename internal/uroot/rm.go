// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/u-root/u-root/pkg/core/rm"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

var errRmRefuseDot = errors.New("refusing to remove '.' or '..' directory")

// rmCommand wraps the u-root rm implementation.
type rmCommand struct {
	baseWrapper
}

// newRmCommand creates a new rm command wrapper.
func newRmCommand() *rmCommand {
	return &rmCommand{
		name: "rm",
		flags: []FlagInfo{
			{Name: "r", Description: "remove directories and their contents recursively"},
			{Name: "R", Description: "same as -r"},
			{Name: "f", Description: "ignore nonexistent files, never prompt"},
			{Name: "i", Description: "prompt before every removal"},
			{Name: "v", Description: "explain what is being done"},
		},
	}
}

// Run executes the rm command.
//
//goplint:ignore -- Command interface uses raw argv strings.
func (c *rmCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 1 {
		if err := checkRmStandaloneDotOperands(args[1:]); err != nil {
			return wrapError(c.name, err)
		}
	}
	return c.runUpstream(ctx, rm.New(), args)
}

//goplint:ignore -- raw CLI operand strings.
func checkRmStandaloneDotOperands(raw []string) error {
	for _, op := range splitRmOperands(raw) {
		if isRmDotOperand(op) {
			return fmt.Errorf("%w: %q", errRmRefuseDot, op)
		}
	}
	return nil
}

//goplint:ignore -- raw CLI operand strings.
func checkRmDotOperands(operands []string) error {
	for _, op := range operands {
		if isRmDotOperand(op) {
			return fmt.Errorf("%w: %q", errRmRefuseDot, op)
		}
	}
	return nil
}

//goplint:ignore -- raw CLI operand string.
func isRmDotOperand(raw string) bool {
	trimmed := strings.TrimRight(raw, "/")
	if goruntime.GOOS == "windows" {
		trimmed = strings.TrimRight(trimmed, `/\`)
	}
	if trimmed == "" {
		return false
	}
	sep := strings.LastIndexByte(trimmed, '/')
	if goruntime.GOOS == "windows" {
		if backslash := strings.LastIndexByte(trimmed, '\\'); backslash > sep {
			sep = backslash
		}
	}
	last := trimmed[sep+1:]
	return last == "." || last == ".."
}

//goplint:ignore -- raw CLI flag and operand strings.
func splitRmOperands(raw []string) []string {
	var tail []string
	if idx := leadingDashDash(raw); idx >= 0 {
		raw, tail = raw[:idx], raw[idx+1:]
	}
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, f := range []string{"r", "R", "f", "i", "v"} {
		fs.Bool(f, false, "")
	}
	if err := fs.Parse(unixflag.ArgsToGoArgs(raw)); err != nil {
		return nil
	}
	return append(slices.Clone(fs.Args()), tail...)
}
