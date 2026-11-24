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
	"time"

	tea "charm.land/bubbletea/v2"
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

func fakeTerminalResolver(ComponentType) (componentTerminal, error) {
	return componentTerminal{close: func() error { return nil }}, nil
}

// spinOpts is the option set every spinWithAction test uses.
func spinOpts() SpinOptions { return SpinOptions{Title: "Wait", Type: SpinnerLine} }

// spinDeps returns test dependencies with a fake terminal and spinner.
func spinDeps(spinner actionSpinnerFunc, cancelWait time.Duration) spinActionDeps {
	return spinActionDeps{resolve: fakeTerminalResolver, spinner: spinner, cancelWait: cancelWait}
}

func TestSpinWithAction_NoTerminalRunsActionOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	deps := spinActionDeps{resolve: noTerminalResolver, spinner: runActionSpinner, cancelWait: SpinCancelWait}
	err := spinWithAction(t.Context(), spinOpts(), func(context.Context) { calls.Add(1) }, deps)
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
	deps := spinActionDeps{
		resolve:    func(ComponentType) (componentTerminal, error) { return componentTerminal{}, errResolve },
		spinner:    runActionSpinner,
		cancelWait: SpinCancelWait,
	}
	err := spinWithAction(t.Context(), spinOpts(), func(context.Context) { calls.Add(1) }, deps)
	if !errors.Is(err, errResolve) {
		t.Fatalf("spinWithAction() error = %v, want the resolver error", err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("action calls = %d, want 0 when the terminal cannot be resolved", n)
	}
}

func TestSpinWithAction_SpinnerFailureWaitsForAction(t *testing.T) {
	t.Parallel()

	errSpinner := errors.New("spinner program failed")
	release := make(chan struct{})
	var calls, finished, cancelled atomic.Int32
	action := func(ctx context.Context) {
		calls.Add(1)
		<-release
		if ctx.Err() != nil {
			cancelled.Add(1)
		}
		finished.Add(1)
	}
	// The spinner fails while the action is still blocked, and lets the action
	// finish only as it returns: spinWithAction must still wait for it, and
	// must not cancel it, since a drawing failure is not a cancellation.
	spinner := func(_ componentTerminal, _ SpinOptions, _ <-chan struct{}) error {
		close(release)
		return errSpinner
	}

	err := spinWithAction(t.Context(), spinOpts(), action, spinDeps(spinner, SpinCancelWait))
	if !errors.Is(err, errSpinner) {
		t.Fatalf("spinWithAction() error = %v, want the spinner error", err)
	}
	if n := finished.Load(); n != 1 {
		t.Fatalf("action finished %d times before spinWithAction returned, want 1", n)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("action calls = %d, want exactly 1", n)
	}
	if n := cancelled.Load(); n != 0 {
		t.Fatal("action context was cancelled after a spinner failure, want it left running")
	}
}

// TestSpinWithAction_CancelWaitsForAction is the N14 race test: it failed on
// the code before the fix, where spinWithAction returned as soon as the
// spinner quit and the action still wrote its result afterwards.
func TestSpinWithAction_CancelWaitsForAction(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		spinErr error
	}{
		{name: "spinner quits", spinErr: nil},
		{name: "spinner interrupted", spinErr: tea.ErrInterrupted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// The spinner returns at once, as it does after Ctrl+C: spinWithAction
			// must cancel the action and not return while it can still write.
			spinner := func(componentTerminal, SpinOptions, <-chan struct{}) error { return tc.spinErr }
			var result string
			action := func(ctx context.Context) {
				<-ctx.Done()
				time.Sleep(20 * time.Millisecond)
				result = "written"
			}

			err := spinWithAction(t.Context(), spinOpts(), action, spinDeps(spinner, SpinCancelWait))
			if !errors.Is(err, ErrSpinCancelled) || !errors.Is(err, context.Canceled) {
				t.Fatalf("spinWithAction() error = %v, want ErrSpinCancelled wrapping context.Canceled", err)
			}
			if result != "written" {
				t.Fatalf("result = %q after spinWithAction returned, want the action finished", result)
			}
		})
	}
}

func TestSpinWithAction_ActionDoneBeforeSpinnerQuit(t *testing.T) {
	t.Parallel()

	// The spinner waits for doneCh, as runActionSpinner does, so the action
	// ended first: a normal completion.
	spinner := func(_ componentTerminal, _ SpinOptions, doneCh <-chan struct{}) error {
		<-doneCh
		return nil
	}
	var cancelled atomic.Bool
	action := func(ctx context.Context) { cancelled.Store(ctx.Err() != nil) }

	if err := spinWithAction(t.Context(), spinOpts(), action, spinDeps(spinner, SpinCancelWait)); err != nil {
		t.Fatalf("spinWithAction() error = %v, want nil when the action ended first", err)
	}
	if cancelled.Load() {
		t.Fatal("action saw a cancelled context, want it left alone")
	}
}

func TestSpinWithAction_CancelWaitBound(t *testing.T) {
	t.Parallel()

	spinner := func(componentTerminal, SpinOptions, <-chan struct{}) error { return tea.ErrInterrupted }
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	// The action ignores its context and blocks until the test ends.
	action := func(context.Context) { <-release }

	const bound = 50 * time.Millisecond
	start := time.Now()
	err := spinWithAction(t.Context(), spinOpts(), action, spinDeps(spinner, bound))
	elapsed := time.Since(start)
	if !errors.Is(err, ErrSpinActionStillRunning) {
		t.Fatalf("spinWithAction() error = %v, want ErrSpinActionStillRunning", err)
	}
	if elapsed < bound || elapsed > bound+5*time.Second {
		t.Fatalf("spinWithAction() returned after %v, want about the %v bound", elapsed, bound)
	}
}

func TestSpinActionModel_CtrlCInterrupts(t *testing.T) {
	t.Parallel()

	model := spinActionModel{doneCh: make(chan struct{})}
	_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Update(ctrl+c) returned no command, want tea.Interrupt")
	}
	msg := cmd()
	if _, ok := msg.(tea.InterruptMsg); !ok {
		t.Fatalf("Update(ctrl+c) command message = %T, want tea.InterruptMsg", msg)
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
