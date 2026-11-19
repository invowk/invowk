// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

var (
	errTestOpenTTY = errors.New("open /dev/tty: no such device or address")
	errTestProbe   = errors.New("the handle is invalid")
)

// tempTerminalFile returns a regular file that stands in for a terminal handle.
func tempTerminalFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// assertClosed fails when f is still open.
func assertClosed(t *testing.T, f *os.File) {
	t.Helper()
	if _, err := f.WriteString("x"); !errors.Is(err, os.ErrClosed) {
		t.Errorf("%s: write after close = %v, want os.ErrClosed", f.Name(), err)
	}
}

// assertOpen fails when f has been closed.
func assertOpen(t *testing.T, f *os.File) {
	t.Helper()
	if _, err := f.WriteString("x"); err != nil {
		t.Errorf("%s: write = %v, want the file still open", f.Name(), err)
	}
}

func failingOpener() (in, out *os.File, err error) {
	return nil, nil, errTestOpenTTY
}

func allTerminals(*os.File) bool { return true }

func noTerminals(*os.File) bool { return false }

func TestResolveComponentTerminal(t *testing.T) {
	t.Parallel()

	t.Run("controlling terminal opens", func(t *testing.T) {
		t.Parallel()
		in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
		stdin, stderr := tempTerminalFile(t, "stdin"), tempTerminalFile(t, "stderr")
		r := terminalResolver{
			open:       func() (*os.File, *os.File, error) { return in, out, nil },
			isTerminal: allTerminals,
			stdin:      stdin,
			stderr:     stderr,
		}

		got, err := r.resolve(ComponentTypeInput)
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		if got.in != in || got.out != out {
			t.Fatalf("resolve() = (%s, %s), want the controlling terminal", got.in.Name(), got.out.Name())
		}
		if err := got.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		assertClosed(t, in)
		assertClosed(t, out)
		assertOpen(t, stdin)
		assertOpen(t, stderr)
	})

	t.Run("one file for input and output closes once", func(t *testing.T) {
		t.Parallel()
		tty := tempTerminalFile(t, "tty")
		r := terminalResolver{
			open:       func() (*os.File, *os.File, error) { return tty, tty, nil },
			isTerminal: noTerminals,
		}

		got, err := r.resolve(ComponentTypeChoose)
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		if err := got.Close(); err != nil {
			t.Fatalf("Close() error = %v, want a single close", err)
		}
		assertClosed(t, tty)
	})

	t.Run("no controlling terminal, terminal stdin and stderr", func(t *testing.T) {
		t.Parallel()
		stdin, stderr := tempTerminalFile(t, "stdin"), tempTerminalFile(t, "stderr")
		r := terminalResolver{open: failingOpener, isTerminal: allTerminals, stdin: stdin, stderr: stderr}

		got, err := r.resolve(ComponentTypeFilter)
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		if got.in != stdin || got.out != stderr {
			t.Fatalf("resolve() = (%s, %s), want stdin and stderr", got.in.Name(), got.out.Name())
		}
		if err := got.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		assertOpen(t, stdin)
		assertOpen(t, stderr)
	})

	t.Run("no controlling terminal, only stdin a terminal", func(t *testing.T) {
		t.Parallel()
		stdin, stderr := tempTerminalFile(t, "stdin"), tempTerminalFile(t, "stderr")
		r := terminalResolver{
			open:       failingOpener,
			isTerminal: func(f *os.File) bool { return f == stdin },
			stdin:      stdin,
			stderr:     stderr,
		}

		_, err := r.resolve(ComponentTypeWrite)
		if !errors.Is(err, ErrNoTerminal) {
			t.Fatalf("resolve() error = %v, want ErrNoTerminal", err)
		}
	})

	t.Run("no terminal at all", func(t *testing.T) {
		t.Parallel()
		r := terminalResolver{
			open:       failingOpener,
			isTerminal: noTerminals,
			stdin:      tempTerminalFile(t, "stdin"),
			stderr:     tempTerminalFile(t, "stderr"),
		}

		_, err := r.resolve(ComponentTypeInput)
		if !errors.Is(err, ErrNoTerminal) {
			t.Fatalf("resolve() error = %v, want ErrNoTerminal", err)
		}
		if !errors.Is(err, errTestOpenTTY) {
			t.Errorf("resolve() error = %v, want it to wrap the open error", err)
		}
		var noTerm *NoTerminalError
		if !errors.As(err, &noTerm) || noTerm.Component != ComponentTypeInput {
			t.Fatalf("resolve() error = %#v, want a NoTerminalError for input", err)
		}
		if !strings.HasPrefix(err.Error(), "input needs a terminal: ") {
			t.Errorf("Error() = %q, want it to start with %q", err.Error(), "input needs a terminal: ")
		}
	})

	t.Run("console probe fails, terminal stdio", func(t *testing.T) {
		t.Parallel()
		in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
		stdin, stderr := tempTerminalFile(t, "stdin"), tempTerminalFile(t, "stderr")
		r := terminalResolver{
			open:       func() (*os.File, *os.File, error) { return in, out, nil },
			probe:      func(*os.File) error { return errTestProbe },
			isTerminal: allTerminals,
			stdin:      stdin,
			stderr:     stderr,
		}

		got, err := r.resolve(ComponentTypeTable)
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		if got.in != stdin || got.out != stderr {
			t.Fatalf("resolve() = (%s, %s), want stdin and stderr", got.in.Name(), got.out.Name())
		}
		assertClosed(t, in)
		assertClosed(t, out)
	})

	t.Run("console probe fails on the output handle only, no terminal", func(t *testing.T) {
		t.Parallel()
		in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
		r := terminalResolver{
			open: func() (*os.File, *os.File, error) { return in, out, nil },
			probe: func(f *os.File) error {
				if f == out {
					return errTestProbe
				}
				return nil
			},
			isTerminal: noTerminals,
		}

		_, err := r.resolve(ComponentTypeConfirm)
		if !errors.Is(err, ErrNoTerminal) || !errors.Is(err, errTestProbe) {
			t.Fatalf("resolve() error = %v, want ErrNoTerminal wrapping the probe error", err)
		}
		if !strings.Contains(err.Error(), "is not a usable console") {
			t.Errorf("Error() = %q, want it to say the console is not usable", err.Error())
		}
		assertClosed(t, in)
		assertClosed(t, out)
	})

	t.Run("console probe passes", func(t *testing.T) {
		t.Parallel()
		in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
		var probed atomic.Int32
		r := terminalResolver{
			open: func() (*os.File, *os.File, error) { return in, out, nil },
			probe: func(*os.File) error {
				probed.Add(1)
				return nil
			},
			isTerminal: noTerminals,
		}

		got, err := r.resolve(ComponentTypeFile)
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		defer func() { _ = got.Close() }()
		if got.in != in || got.out != out {
			t.Fatalf("resolve() = (%s, %s), want the console handles", got.in.Name(), got.out.Name())
		}
		if n := probed.Load(); n != 2 {
			t.Errorf("probe calls = %d, want 2 (input and output)", n)
		}
	})
}

func noTerminalResolver(component ComponentType) (componentTerminal, error) {
	return componentTerminal{}, &NoTerminalError{Component: component, Cause: errTestOpenTTY}
}

func TestRunComponentProgramWith_NoTerminal(t *testing.T) {
	t.Parallel()

	model := NewInputModel(InputOptions{Title: "Name"})
	got, err := runComponentProgramWith(noTerminalResolver, ComponentTypeInput, model)
	if !errors.Is(err, ErrNoTerminal) {
		t.Fatalf("runComponentProgramWith() error = %v, want ErrNoTerminal", err)
	}
	if got != model {
		t.Errorf("runComponentProgramWith() model = %T, want the initial model", got)
	}
}

func TestSpinWithAction_NoTerminalRunsActionOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	err := spinWithAction(SpinOptions{Title: "Wait", Type: SpinnerLine}, func() { calls.Add(1) }, noTerminalResolver)
	if err != nil {
		t.Fatalf("spinWithAction() error = %v, want nil without a terminal", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("action calls = %d, want exactly 1", n)
	}
}

func TestSpinWithAction_ResolverErrorSkipsAction(t *testing.T) {
	t.Parallel()

	errResolve := errors.New("resolver broke")
	var calls atomic.Int32
	err := spinWithAction(SpinOptions{Title: "Wait", Type: SpinnerLine}, func() { calls.Add(1) },
		func(ComponentType) (componentTerminal, error) { return componentTerminal{}, errResolve })
	if !errors.Is(err, errResolve) {
		t.Fatalf("spinWithAction() error = %v, want the resolver error", err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("action calls = %d, want 0 when the terminal cannot be resolved", n)
	}
}

func TestSpinWithContext_NoTerminalWaitsForCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := spinWithContext(ctx, SpinOptions{Title: "Wait", Type: SpinnerLine}, noTerminalResolver); err != nil {
		t.Fatalf("spinWithContext() error = %v, want nil without a terminal", err)
	}
}

func TestPager_NoTerminalWritesContent(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	opts := PagerOptions{Title: "Log", Content: "x\n", ShowLineNumbers: true, Config: DefaultConfig()}
	if err := pagerWith(opts, noTerminalResolver, &stdout); err != nil {
		t.Fatalf("pagerWith() error = %v, want nil without a terminal", err)
	}
	if got := stdout.String(); got != "x\n" {
		t.Fatalf("stdout = %q, want the content verbatim %q", got, "x\n")
	}
}
