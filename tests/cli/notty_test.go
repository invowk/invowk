// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// cliTestM builds the invowk binary before it runs the tests.
type cliTestM struct{ m *testing.M }

// Run builds bin/invowk and then runs the tests.
func (c cliTestM) Run() int {
	buildCLIBinary()
	return c.m.Run()
}

// testscriptPrograms returns the helper programs that testscript.Main installs
// on the PATH of every script. They are real executables, so they work as the
// command of `exec`, `exitcode` or another program, on every OS.
func testscriptPrograms() map[string]func() {
	return map[string]func(){
		"notty":     nottyMain,
		"tuiaction": tuiactionMain,
	}
}

// nottyMain implements `notty <cmd> [args...]`: it runs the command without a
// controlling terminal and exits with its status. On Unix the child starts a
// new session (like `setsid -w`); on Windows it starts with no console at all.
// stdin, stdout and stderr are inherited, so testscript's `stdin` file and its
// captured output reach the child unchanged.
func nottyMain() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: notty <cmd> [args...]")
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...) //nolint:noctx // test helper: runs the command the script names, no context in a program main
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = noTerminalProcAttr()
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "notty: %v\n", err)
		os.Exit(127)
	}
	os.Exit(0)
}

// tuiactionMain implements `tuiaction <file> <status>`: it appends "run" to
// <file>, prints "ok" and exits with <status>. It is a portable stand-in for
// `sh -c 'echo run >>file; echo ok; exit N'` as the command of `tui spin`.
func tuiactionMain() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: tuiaction <file> <status>")
		os.Exit(2)
	}
	status, err := strconv.Atoi(os.Args[2])
	if err != nil || status < 0 || status > 255 {
		fmt.Fprintf(os.Stderr, "tuiaction: invalid status %q\n", os.Args[2])
		os.Exit(2)
	}
	f, err := os.OpenFile(os.Args[1], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tuiaction: %v\n", err)
		os.Exit(2)
	}
	if _, err := fmt.Fprintln(f, "run"); err != nil {
		fmt.Fprintf(os.Stderr, "tuiaction: %v\n", err)
		os.Exit(2)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "tuiaction: %v\n", err)
		os.Exit(2)
	}
	fmt.Println("ok")
	os.Exit(status)
}
