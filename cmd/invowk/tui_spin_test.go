// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
	"github.com/invowk/invowk/pkg/types"
)

type fakeTUISpinRunner struct {
	command string
	args    []string
	result  tuiSpinRunResult
}

func (r *fakeTUISpinRunner) Run(_ context.Context, command string, args []string) tuiSpinRunResult {
	r.command = command
	r.args = append([]string(nil), args...)
	return r.result
}

func TestRunTUISpinWithRunnerSuccess(t *testing.T) {
	t.Parallel()

	cmd := newTUISpinCommand()
	cmd.SetContext(t.Context())
	runner := &fakeTUISpinRunner{result: tuiSpinRunResult{Output: []byte("done\n")}}
	var stdout bytes.Buffer

	err := runTuiSpinWithRunner(cmd, []string{"make", "test"}, runner, immediateSpin, &stdout)
	if err != nil {
		t.Fatalf("runTuiSpinWithRunner() error = %v", err)
	}
	if runner.command != "make" || len(runner.args) != 1 || runner.args[0] != "test" {
		t.Fatalf("runner got command=%q args=%v, want make [test]", runner.command, runner.args)
	}
	if stdout.String() != "done\n" {
		t.Fatalf("stdout = %q, want done newline", stdout.String())
	}
}

func TestRunTUISpinWithRunnerExitCode(t *testing.T) {
	t.Parallel()

	cmd := newTUISpinCommand()
	cmd.SetContext(t.Context())
	runner := &fakeTUISpinRunner{result: tuiSpinRunResult{ExitCode: types.ExitCode(42)}}
	var stdout bytes.Buffer

	err := runTuiSpinWithRunner(cmd, []string{"false"}, runner, immediateSpin, &stdout)
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok {
		t.Fatalf("runTuiSpinWithRunner() error = %T %v, want ExitError", err, err)
	}
	if exitErr.Code != 42 {
		t.Fatalf("ExitError.Code = %d, want 42", exitErr.Code)
	}
	if !cmd.SilenceErrors || !cmd.SilenceUsage {
		t.Fatal("command should silence errors and usage for child exit codes")
	}
}

func TestRunTUISpinWithRunnerError(t *testing.T) {
	t.Parallel()

	cmd := newTUISpinCommand()
	cmd.SetContext(t.Context())
	wantErr := errors.New("runner failed")
	runner := &fakeTUISpinRunner{result: tuiSpinRunResult{Err: wantErr}}
	var stdout bytes.Buffer

	err := runTuiSpinWithRunner(cmd, []string{"tool"}, runner, immediateSpin, &stdout)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runTuiSpinWithRunner() error = %v, want %v", err, wantErr)
	}
}

func TestRunTUISpinDelegatesPresentationAndRunsCommandLocally(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			// The client's reachability preflight.
			_, _ = w.Write([]byte("ok"))
			return
		}
		if r.URL.Path != "/tui" {
			http.NotFound(w, r)
			return
		}
		requests++
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		var req tuiwire.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Decode(request) = %v", err)
		}
		if req.Component != tuiwire.ComponentSpin {
			t.Errorf("Component = %q, want spin", req.Component)
		}
		_ = json.NewEncoder(w).Encode(tuiwire.Response{
			Result: json.RawMessage(`{}`),
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv(tuiwire.EnvTUIAddr, server.URL)
	t.Setenv(tuiwire.EnvTUIToken, "secret-token")

	cmd := newTUISpinCommand()
	cmd.SetContext(t.Context())
	runner := &fakeTUISpinRunner{result: tuiSpinRunResult{Output: []byte("done\n")}}
	var stdout bytes.Buffer

	err := runTuiSpinWithRunner(cmd, []string{"make", "test"}, runner, immediateSpin, &stdout)
	if err != nil {
		t.Fatalf("runTuiSpinWithRunner() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("delegated spin requests = %d, want 1", requests)
	}
	if runner.command != "make" || len(runner.args) != 1 || runner.args[0] != "test" {
		t.Fatalf("runner got command=%q args=%v, want make [test]", runner.command, runner.args)
	}
	if stdout.String() != "done\n" {
		t.Fatalf("stdout = %q, want done newline", stdout.String())
	}
}

func TestRunTUISpinOutcomes(t *testing.T) {
	t.Parallel()

	errSpinner := errors.New("spinner program failed")
	errResolve := errors.New("terminal broke")
	tests := []struct {
		name       string
		spin       tuiSpinAction
		result     tuiSpinRunResult
		wantRun    bool
		wantStdout string
		wantStatus types.ExitCode
		wantErr    error
		wantStderr string
	}{
		{
			name: "cancelled after the command ended", spin: spinThen(tui.ErrSpinCancelled),
			result:  tuiSpinRunResult{Output: []byte("started\n"), ExitCode: types.ExitCode(-1)},
			wantRun: true, wantStdout: "started\n", wantStatus: 1, wantErr: context.Canceled,
		},
		{
			name: "command still running after the bound", spin: spinThen(tui.ErrSpinActionStillRunning),
			result:  tuiSpinRunResult{Output: []byte("started\n")},
			wantRun: true, wantStdout: "", wantStatus: 1, wantErr: tui.ErrSpinActionStillRunning,
		},
		{
			name: "spinner failed, command exited 3", spin: spinThen(errSpinner),
			result:  tuiSpinRunResult{Output: []byte("ok\n"), ExitCode: types.ExitCode(3)},
			wantRun: true, wantStdout: "ok\n", wantStatus: 3, wantStderr: "warning: spinner failed: spinner program failed\n",
		},
		{
			name: "spinner failed, command succeeded", spin: spinThen(errSpinner),
			result:  tuiSpinRunResult{Output: []byte("ok\n")},
			wantRun: true, wantStdout: "ok\n", wantStatus: 0, wantStderr: "warning: spinner failed: spinner program failed\n",
		},
		{
			name: "terminal not resolved, command not run", spin: spinWithoutAction(errResolve),
			wantStatus: 1, wantErr: errResolve,
		},
		{
			name: "spin returned nothing and ran nothing", spin: spinWithoutAction(nil),
			wantStatus: 1, wantErr: errTUISpinNotRun,
		},
		{
			name: "command ended by a signal", spin: immediateSpin,
			result:  tuiSpinRunResult{Output: []byte("partial\n"), ExitCode: types.ExitCode(-1)},
			wantRun: true, wantStdout: "partial\n", wantStatus: 1,
		},
		{
			name: "command status above 255", spin: immediateSpin,
			result:  tuiSpinRunResult{ExitCode: types.ExitCode(300)},
			wantRun: true, wantStatus: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newTUISpinCommand()
			cmd.SetContext(t.Context())
			var stdout, stderr bytes.Buffer
			cmd.SetErr(&stderr)
			runner := &fakeTUISpinRunner{result: tt.result}

			err := runTuiSpinWithRunner(cmd, []string{"tool"}, runner, tt.spin, &stdout)
			if got := exitCodeForError(err); got != tt.wantStatus {
				t.Fatalf("exit status = %d (error %v), want %d", got, err, tt.wantStatus)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want it to wrap %v", err, tt.wantErr)
			}
			if ran := runner.command != ""; ran != tt.wantRun {
				t.Fatalf("command ran = %v, want %v", ran, tt.wantRun)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunTUISpinCancelledContextMapsToStatusOne(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd := newTUISpinCommand()
	cmd.SetContext(ctx)
	// Under fang, SIGINT cancels the cmd context, and the killed command has
	// no exit status (-1 on Unix) or status 1 (Windows).
	for _, result := range []tuiSpinRunResult{
		{Output: []byte("started\n"), ExitCode: types.ExitCode(-1)},
		{Output: []byte("started\n"), ExitCode: types.ExitCode(1)},
		{Err: errors.New("signal: interrupt")},
	} {
		var stdout bytes.Buffer
		err := runTuiSpinWithRunner(cmd, []string{"tool"}, &fakeTUISpinRunner{result: result}, immediateSpin, &stdout)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("result %+v: error = %v, want it to wrap context.Canceled", result, err)
		}
		if got := exitCodeForError(err); got != 1 {
			t.Fatalf("result %+v: exit status = %d, want 1", result, got)
		}
	}
}

func TestTUISpinCommandWaitDelayBelowCancelWait(t *testing.T) {
	t.Parallel()

	if tuiSpinCommandWaitDelay >= tui.SpinCancelWait {
		t.Fatalf("tuiSpinCommandWaitDelay = %v, want it below tui.SpinCancelWait (%v)",
			tuiSpinCommandWaitDelay, tui.SpinCancelWait)
	}
}

func immediateSpin(ctx context.Context, _ tui.SpinOptions, action func(context.Context)) error {
	action(ctx)
	return nil
}

// spinThen runs the action to its end, then returns err, as SpinWithAction
// does after a cancellation or a spinner failure. With
// tui.ErrSpinActionStillRunning, the adapter must not read the result even
// though one is there.
func spinThen(err error) tuiSpinAction {
	return func(ctx context.Context, _ tui.SpinOptions, action func(context.Context)) error {
		action(ctx)
		return err
	}
}

// spinWithoutAction returns err without running the action, as SpinWithAction
// does when the terminal cannot be resolved.
func spinWithoutAction(err error) tuiSpinAction {
	return func(context.Context, tui.SpinOptions, func(context.Context)) error {
		return err
	}
}
