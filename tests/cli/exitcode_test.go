// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"os/exec"
	"strconv"

	"github.com/rogpeppe/go-internal/testscript"
)

// testscriptCmds returns the custom testscript commands shared by the CLI and
// container runners.
func testscriptCmds() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"exitcode": cmdExitCode,
	}
}

// cmdExitCode implements `exitcode <want> <cmd> <args...>`: it runs the
// command like `exec` (stdout and stderr stay available to later assertions)
// and fails unless the process exits with exactly <want>. Negation is not
// supported because the expected status is explicit.
func cmdExitCode(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("exitcode does not support negation")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: exitcode <want> <cmd> [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil || want < 0 || want > 255 {
		ts.Fatalf("exitcode: invalid expected status %q", args[0])
	}

	got := 0
	if runErr := ts.Exec(args[1], args[2:]...); runErr != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](runErr)
		if !ok {
			ts.Fatalf("exitcode: %v", runErr)
		}
		got = exitErr.ExitCode()
	}
	if got != want {
		ts.Fatalf("exitcode: %s exited with status %d, want %d\n[stderr]\n%s", args[1], got, want, ts.ReadFile("stderr"))
	}
}
