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
	"github.com/invowk/invowk/internal/tuiserver"
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
	t.Setenv(tuiserver.EnvTUIAddr, server.URL)
	t.Setenv(tuiserver.EnvTUIToken, "secret-token")

	cmd := newTUISpinCommand()
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

func immediateSpin(_ tui.SpinOptions, action func()) error {
	action()
	return nil
}
