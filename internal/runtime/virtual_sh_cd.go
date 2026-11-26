// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"

	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	virtualShCdBuiltin    = "cd"
	virtualShPushdBuiltin = "pushd"
	virtualShPopdBuiltin  = "popd"
	virtualShCdPrev       = "-"

	// NUL-prefixed sentinels cannot be entered as shell command words, so a
	// script function or external command can never collide with them.
	virtualShDirDeniedSentinel = "\x00invowk-virtual-sh-dir-denied"
	virtualShDirStackSentinel  = "\x00invowk-virtual-sh-dir-stack"
)

type (
	virtualShDirDenialContextKey struct{}

	virtualShDirDenial struct {
		err error
	}
)

// cdCallHandler returns the call handler that guards restricted `cd`, `pushd`,
// and `popd` calls (including via `builtin` and `command`). Direct-target
// denials rewrite args to an unforgeable exec sentinel handled in execHandler
// so stderr reports one "<cmd>: <err>" line without mvdan's misleading "no such
// file or directory" line. Stack-dependent 0-arg pushd/popd calls rewrite to
// virtualShDirStackSentinel so execHandler can run hc.Builtin with a context
// that records any restricted-root denial from statHandler.
func (v virtualPathValidator) cdCallHandler() interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if v.resolver.access == invowkfile.VirtualFilesystemAccessFull {
			return args, nil
		}
		cmdName, operands, ok := parseVirtualShDirCall(args)
		if !ok {
			return args, nil
		}
		hc := interp.HandlerCtx(ctx)
		target, direct := virtualShDirDirectTarget(cmdName, hc.Env, operands)
		if !direct {
			return append([]string{virtualShDirStackSentinel, cmdName}, args...), nil
		}
		if target == "" {
			return args, nil
		}
		if _, err := v.validate(hc.Dir, target); err != nil {
			//nolint:nilerr // CallHandler rewrites args to exec sentinel so execHandler reports the denial and exits 1 without aborting the runner.
			return []string{virtualShDirDeniedSentinel, cmdName, err.Error()}, nil
		}
		return args, nil
	}
}

//goplint:ignore -- raw shell command/operand strings.
func handleVirtualShDirBuiltin(ctx context.Context, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	hc := interp.HandlerCtx(ctx)
	switch args[0] {
	case virtualShDirDeniedSentinel:
		if len(args) >= 3 && hc.Stderr != nil {
			fmt.Fprintf(hc.Stderr, "%s: %s\n", args[1], args[2])
		}
		return true, interp.ExitStatus(1)
	case virtualShDirStackSentinel:
		if len(args) < 3 {
			return true, interp.ExitStatus(1)
		}
		cmdName := args[1]
		origArgs := args[2:]
		holder := &virtualShDirDenial{}
		subCtx := context.WithValue(ctx, virtualShDirDenialContextKey{}, holder)
		err := hc.Builtin(subCtx, origArgs)
		if holder.err != nil && hc.Stderr != nil {
			fmt.Fprintf(hc.Stderr, "%s: %v\n", cmdName, holder.err)
		}
		return true, err //nolint:wrapcheck // mvdan/sh execHandler interprets hc.Builtin errors (including interp.ExitStatus) directly.
	default:
		return false, nil
	}
}

func recordVirtualShDirDenial(ctx context.Context, err error) {
	if err == nil {
		return
	}
	if holder, ok := ctx.Value(virtualShDirDenialContextKey{}).(*virtualShDirDenial); ok && holder != nil && holder.err == nil {
		holder.err = err
	}
}

//goplint:ignore -- raw shell command/operand strings.
func unwrapDirCallPrefix(args []string) []string {
	for len(args) > 0 {
		switch args[0] {
		case "builtin":
			args = args[1:]
		case "command":
			rest, ok := unwrapCommandBuiltinPrefix(args[1:])
			if !ok {
				return nil
			}
			args = rest
		default:
			return args
		}
	}
	return nil
}

//goplint:ignore -- raw shell command/operand strings.
func unwrapCommandBuiltinPrefix(rest []string) ([]string, bool) {
	for len(rest) > 0 {
		arg := rest[0]
		if arg == "--" {
			return rest[1:], true
		}
		if len(arg) < 2 || arg[0] != '-' {
			return rest, true
		}
		// Only -p is compatible with invoking the directory builtin; -v/-V or
		// unknown flags must pass straight through to mvdan.
		if strings.Trim(arg[1:], "p") != "" {
			return nil, false
		}
		rest = rest[1:]
	}
	return nil, false
}

//goplint:ignore -- raw shell command/operand strings.
func parseVirtualShDirCall(args []string) (cmdName string, operands []string, ok bool) {
	unwrapped := unwrapDirCallPrefix(args)
	if len(unwrapped) == 0 {
		return "", nil, false
	}
	switch unwrapped[0] {
	case virtualShCdBuiltin, virtualShPushdBuiltin, virtualShPopdBuiltin:
		return unwrapped[0], unwrapped[1:], true
	default:
		return "", nil, false
	}
}

//goplint:ignore -- raw shell command/operand strings.
func virtualShDirDirectTarget(cmdName string, env expand.Environ, operands []string) (target string, direct bool) {
	switch cmdName {
	case virtualShCdBuiltin:
		stripped := stripVirtualShCdFlags(operands)
		if len(stripped) > 1 {
			return "", true
		}
		return virtualShCdTarget(env, stripped), true
	case virtualShPushdBuiltin:
		if slices.Contains(operands, "-n") {
			// pushd -n pushes without changing directories; mvdan checks
			// stat only when a later popd/pushd enters that directory.
			return "", true
		}
		if len(operands) == 0 {
			return "", false
		}
		if len(operands) == 1 {
			return operands[0], true
		}
		return "", true
	case virtualShPopdBuiltin:
		if slices.Contains(operands, "-n") {
			return "", true
		}
		if len(operands) == 0 {
			return "", false
		}
		return "", true
	default:
		return "", true
	}
}

//goplint:ignore -- raw shell operand strings.
func stripVirtualShCdFlags(operands []string) []string {
	for len(operands) > 0 {
		op := operands[0]
		if op == "--" {
			return operands[1:]
		}
		if len(op) < 2 || op[0] != '-' || strings.Trim(op[1:], "LP") != "" {
			return operands
		}
		operands = operands[1:]
	}
	return operands
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
