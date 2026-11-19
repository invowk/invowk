// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// ErrNoTerminal is returned when a directly rendered component cannot find a
// terminal to draw on: the controlling terminal cannot be opened (or, on Windows,
// is not a usable console) and stdin and stderr are not both terminals.
var ErrNoTerminal = errors.New("needs a terminal")

type (
	// NoTerminalError names the component that could not find a terminal and
	// carries the reason the controlling terminal could not be opened.
	// It wraps ErrNoTerminal for errors.Is() compatibility.
	NoTerminalError struct {
		Component ComponentType
		Cause     error
	}

	// componentTerminal is the terminal a directly rendered component draws on
	// and reads keys from. The files are passed to Bubble Tea unwrapped so that
	// it can enter raw mode and read the terminal size.
	componentTerminal struct {
		in    *os.File
		out   *os.File
		close func() error
	}

	// terminalOpener opens the controlling terminal: /dev/tty on Unix,
	// CONIN$ and CONOUT$ on Windows.
	terminalOpener func() (in, out *os.File, err error)

	// consoleProbe reports an error when an opened handle is not a usable
	// console. Unix needs none (nil): an opened /dev/tty is the terminal.
	consoleProbe func(f *os.File) error

	// terminalResolver holds the inputs of the terminal-selection rule, so the
	// rule can be unit-tested without a real terminal.
	terminalResolver struct {
		open       terminalOpener
		probe      consoleProbe
		isTerminal func(f *os.File) bool
		stdin      *os.File
		stderr     *os.File
	}

	// resolveTerminalFunc resolves the terminal for one component.
	resolveTerminalFunc func(component ComponentType) (componentTerminal, error)
)

// Error implements error.
func (e *NoTerminalError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Component, ErrNoTerminal, e.Cause)
}

// Unwrap returns ErrNoTerminal and the cause, for errors.Is() and errors.As().
func (e *NoTerminalError) Unwrap() []error {
	return []error{ErrNoTerminal, e.Cause}
}

// newTerminalResolver returns the resolver for the current process: the
// platform's controlling-terminal opener and console probe, and the process
// stdin and stderr for the fallback.
func newTerminalResolver() terminalResolver {
	return terminalResolver{
		open:       platformTerminalOpener,
		probe:      platformConsoleProbe(),
		isTerminal: isTerminalFile,
		stdin:      os.Stdin,
		stderr:     os.Stderr,
	}
}

// resolveComponentTerminal resolves the terminal of the current process.
func resolveComponentTerminal(component ComponentType) (componentTerminal, error) {
	return newTerminalResolver().resolve(component)
}

// resolve applies the terminal-selection rule:
//  1. the controlling terminal, after the console probe;
//  2. stdin and stderr, when both are terminals;
//  3. otherwise a NoTerminalError wrapping the open (or probe) error.
//
// The result never involves stdout, which carries only the component's result.
func (r terminalResolver) resolve(component ComponentType) (componentTerminal, error) {
	in, out, err := openProbedTerminal(r.open, r.probe)
	if err == nil {
		return componentTerminal{in: in, out: out, close: closeTerminalFiles(in, out)}, nil
	}

	if r.stdin != nil && r.stderr != nil && r.isTerminal(r.stdin) && r.isTerminal(r.stderr) {
		// The process owns stdin and stderr: never close them.
		return componentTerminal{in: r.stdin, out: r.stderr, close: func() error { return nil }}, nil
	}

	return componentTerminal{}, &NoTerminalError{Component: component, Cause: err}
}

// Close releases the terminal handles the resolver opened.
func (t componentTerminal) Close() error {
	if t.close == nil {
		return nil
	}
	return t.close()
}

// programOptions returns the Bubble Tea options that bind a program to t.
func (t componentTerminal) programOptions() []tea.ProgramOption {
	return []tea.ProgramOption{tea.WithInput(t.in), tea.WithOutput(t.out)}
}

// openProbedTerminal opens the controlling terminal and, when a probe is given,
// checks both handles. A handle that fails the probe closes both and counts as
// a terminal that could not be opened.
func openProbedTerminal(open terminalOpener, probe consoleProbe) (in, out *os.File, err error) {
	in, out, err = open()
	if err != nil {
		return nil, nil, err
	}
	if probe == nil {
		return in, out, nil
	}
	for _, f := range []*os.File{in, out} {
		if probeErr := probe(f); probeErr != nil {
			probeErr = fmt.Errorf("%s is not a usable console: %w", f.Name(), probeErr)
			return nil, nil, errors.Join(probeErr, closeTerminalFiles(in, out)())
		}
	}
	return in, out, nil
}

// closeTerminalFiles closes in and out, once when they are the same file
// (/dev/tty on Unix).
func closeTerminalFiles(in, out *os.File) func() error {
	return func() error {
		err := in.Close()
		if out != in {
			err = errors.Join(err, out.Close())
		}
		return err
	}
}

// isTerminalFile reports whether f is a terminal.
func isTerminalFile(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// runComponentProgram runs model on the terminal of the current process.
func runComponentProgram(component ComponentType, model tea.Model, opts ...tea.ProgramOption) (tea.Model, error) {
	return runComponentProgramWith(resolveComponentTerminal, component, model, opts...)
}

// runComponentProgramWith runs model on the terminal that resolve selects and
// closes that terminal after the program exits.
func runComponentProgramWith(resolve resolveTerminalFunc, component ComponentType, model tea.Model, opts ...tea.ProgramOption) (finalModel tea.Model, err error) {
	terminal, err := resolve(component)
	if err != nil {
		return model, err
	}
	defer func() {
		err = errors.Join(err, terminal.Close())
	}()

	p := tea.NewProgram(model, append(terminal.programOptions(), opts...)...)
	return p.Run()
}
