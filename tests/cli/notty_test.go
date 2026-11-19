// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
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
// captured output reach the child unchanged. Interrupt and termination
// signals are relayed to the child, which no longer shares notty's session.
// A child killed by a signal exits notty with 128+signal; a command that
// cannot be found exits 127 and any other start failure 126.
//
// On Windows only the direct child loses its console: a console program it
// starts gets a new console. Run invowk itself as the direct child.
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
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "notty: %v\n", err)
		if errors.Is(err, exec.ErrNotFound) {
			os.Exit(127)
		}
		os.Exit(126)
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		for s := range sigs {
			_ = cmd.Process.Signal(s) // best effort: Windows supports only Kill
		}
	}()
	err := cmd.Wait()
	signal.Stop(sigs)
	if err == nil {
		os.Exit(0)
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		fmt.Fprintf(os.Stderr, "notty: %v\n", err)
		os.Exit(126)
	}
	if ws, isWait := exitErr.Sys().(syscall.WaitStatus); isWait && ws.Signaled() {
		os.Exit(128 + int(ws.Signal()))
	}
	os.Exit(exitErr.ExitCode())
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
