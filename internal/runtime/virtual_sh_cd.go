// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"fmt"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"

	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	virtualShCdBuiltin = "cd"
	virtualShCdPrev    = "-"
)

// cdCallHandler returns the call handler that names the reason a restricted
// `cd` is refused. mvdan's `cd` reports every stat error as "no such file or
// directory", so a policy denial would read as a missing directory.
//
// For a call `cd` with at most one argument, under restricted access, it
// computes the directory as mvdan's `cd` does ($HOME with no argument, $OLDPWD
// for "-", otherwise the argument) and validates it. When validation fails it
// writes "cd: <error>" to the command's standard error. It never changes the
// arguments and never returns an error (an error would halt the runner): `cd`
// then runs as usual, the stat handler refuses the same path, and `cd` exits 1
// with its own message. An empty directory gets no line: mvdan already prints
// "cd: empty directory path" for it. Under full access it does nothing.
func (v virtualPathValidator) cdCallHandler() interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if v.resolver.access == invowkfile.VirtualFilesystemAccessFull ||
			args[0] != virtualShCdBuiltin || len(args) > 2 {
			return args, nil
		}
		hc := interp.HandlerCtx(ctx)
		dir := virtualShCdTarget(hc.Env, args[1:])
		if dir == "" {
			return args, nil
		}
		if _, err := v.validate(hc.Dir, dir); err != nil && hc.Stderr != nil {
			fmt.Fprintf(hc.Stderr, "%s: %v\n", virtualShCdBuiltin, err)
		}
		return args, nil
	}
}

// virtualShCdTarget returns the directory mvdan's `cd` changes into for the
// given operands (at most one).
//
//goplint:ignore -- raw shell operand and environment strings.
func virtualShCdTarget(env expand.Environ, operands []string) string {
	switch {
	case len(operands) == 0:
		return env.Get("HOME").String()
	case operands[0] == virtualShCdPrev:
		return env.Get("OLDPWD").String()
	default:
		return operands[0]
	}
}
