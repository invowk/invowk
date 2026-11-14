// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"

	"github.com/spf13/cobra"
)

type fakeAmbiguityCommandService struct {
	err error
}

func (s *fakeAmbiguityCommandService) Execute(context.Context, ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	return ExecuteResult{}, nil, nil
}

func (s *fakeAmbiguityCommandService) ResolveCommand(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	if s.err != nil {
		return nil, req, nil, s.err
	}
	return &discovery.CommandInfo{Name: "build", SimpleName: "build"}, req, nil, nil
}

func (s *fakeAmbiguityCommandService) ResolveWatchPlan(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error) {
	if s.err != nil {
		return nil, req, commandsvc.WatchPlan{}, nil, s.err
	}
	return &discovery.CommandInfo{Name: "build", SimpleName: "build"}, req, commandsvc.WatchPlan{}, nil, nil
}

func (s *fakeAmbiguityCommandService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return &discovery.CommandInfo{Name: "build", SimpleName: "build"}, req, nil, nil
}

// TestCreateRuntimeSession_ContainerInitializationIsScoped verifies that
// runtime setup returns independent sessions so cleanup and provisioning state
// stay scoped to each execution.
func TestCreateRuntimeSession_ContainerInitializationIsScoped(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	session := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer session.Close()

	if session.NewExecutionID() == "" {
		t.Fatal("Create() returned a session that cannot create execution IDs")
	}
	if err := session.ContainerInitErr(); err != nil {
		// Container engine not available in this environment — the invariant
		// is trivially satisfied (zero instances).
		t.Logf("container runtime not available: %v (invariant trivially satisfied)", err)
		return
	}

	session2 := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer session2.Close()
	if session2.ContainerInitErr() != nil {
		t.Logf("second container session unavailable: %v", session2.ContainerInitErr())
		return
	}

	if session == session2 {
		t.Fatal("two Create calls returned the same runtime session")
	}
}

func runtimeContextForMode(t testing.TB, mode invowkfile.RuntimeMode) *runtime.ExecutionContext {
	t.Helper()

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "hello",
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: "true"},
				Runtimes:  []invowkfile.RuntimeConfig{{Name: mode}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	ctx := runtime.NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = mode
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]
	return ctx
}

func newTestHostAccess(t testing.TB) *commandadapters.HostAccess {
	t.Helper()

	hostAccess, err := commandadapters.NewHostAccess()
	if err != nil {
		t.Fatalf("NewHostAccess() error = %v", err)
	}
	return hostAccess
}

// TestParseEnvVarFlags verifies parsing of KEY=VALUE strings into a map,
// including edge cases like empty values and values containing '=', and that
// malformed entries (no '=' or an empty KEY) are rejected with an error
// naming the value instead of being skipped.
func TestParseEnvVarFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr string
	}{
		{
			name:  "nil input returns an empty map",
			input: nil,
			want:  map[string]string{},
		},
		{
			name:  "empty slice returns an empty map",
			input: []string{},
			want:  map[string]string{},
		},
		{
			name:  "single valid entry",
			input: []string{"FOO=bar"},
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "multiple valid entries",
			input: []string{"FOO=bar", "BAZ=qux"},
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "empty value after equals is valid",
			input: []string{"FOO="},
			want:  map[string]string{"FOO": ""},
		},
		{
			name:  "value containing equals sign",
			input: []string{"FOO=bar=baz"},
			want:  map[string]string{"FOO": "bar=baz"},
		},
		{
			name:    "missing equals is an error",
			input:   []string{"INVALID"},
			wantErr: `invalid --ivk-env-var value "INVALID": expected KEY=VALUE`,
		},
		{
			name:    "equals at start is an error",
			input:   []string{"=value"},
			wantErr: `invalid --ivk-env-var value "=value"`,
		},
		{
			name:    "empty entry is an error",
			input:   []string{""},
			wantErr: `invalid --ivk-env-var value ""`,
		},
		{
			name:    "first invalid entry among valid ones is reported",
			input:   []string{"GOOD=val", "BAD", "ALSO_GOOD="},
			wantErr: `"BAD"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseEnvVarFlags(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseEnvVarFlags() = %v, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseEnvVarFlags() error = %q, want it to contain %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("parseEnvVarFlags() = %v on error, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEnvVarFlags() error = %v", err)
			}

			if !maps.Equal(got, tt.want) {
				t.Fatalf("parseEnvVarFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// newReservedFlagTestCommand returns a Cobra command carrying the reserved
// --ivk-* leaf flags that buildCommandExecuteRequest reads, parsed from args.
func newReservedFlagTestCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "leaf"}
	cmd.Flags().StringArray("ivk-env-file", nil, "")
	cmd.Flags().StringArray("ivk-env-var", nil, "")
	cmd.Flags().String("ivk-env-inherit-mode", "", "")
	cmd.Flags().StringArray("ivk-env-inherit-allow", nil, "")
	cmd.Flags().StringArray("ivk-env-inherit-deny", nil, "")
	cmd.Flags().String("ivk-workdir", "", "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v) error = %v", args, err)
	}
	return cmd
}

// TestBuildCommandExecuteRequestReservedFlagErrors verifies that every invalid
// reserved --ivk-* flag value is rejected before the request is built and maps
// to the usage exit status, while valid values (including KEY=) are accepted.
func TestBuildCommandExecuteRequestReservedFlagErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		runtime       string
		containerName string
		wantErr       string
	}{
		{name: "valid values", args: []string{"--ivk-env-var", "FOO=", "--ivk-workdir", "sub", "--ivk-env-inherit-allow", "HOME"}},
		{name: "unknown runtime mode", runtime: "bogus", wantErr: "bogus"},
		{name: "unknown env inherit mode", args: []string{"--ivk-env-inherit-mode", "bogus"}, wantErr: "bogus"},
		{name: "env var without equals", args: []string{"--ivk-env-var", "NOEQ"}, wantErr: `invalid --ivk-env-var value "NOEQ"`},
		{name: "env var with empty name", args: []string{"--ivk-env-var", "=v"}, wantErr: `invalid --ivk-env-var value "=v"`},
		{name: "whitespace workdir", args: []string{"--ivk-workdir", " "}, wantErr: "invalid workdir"},
		{name: "empty env file", args: []string{"--ivk-env-file="}, wantErr: "dotenv file path"},
		{name: "invalid allowlist name", args: []string{"--ivk-env-inherit-allow", "1BAD"}, wantErr: "1BAD"},
		{name: "invalid denylist name", args: []string{"--ivk-env-inherit-deny", "1BAD"}, wantErr: "1BAD"},
		{name: "invalid container name", containerName: "../x", wantErr: "../x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newReservedFlagTestCommand(t, tt.args...)
			cmdFlags := &cmdFlagValues{runtimeOverride: tt.runtime, containerName: tt.containerName}
			req, err := buildCommandExecuteRequest(cmd, &rootFlagValues{}, cmdFlags, executeRequestOptions{Name: "leaf"})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("buildCommandExecuteRequest() error = %v", err)
				}
				if value, ok := req.EnvVars["FOO"]; !ok || value != "" {
					t.Fatalf("EnvVars = %v, want FOO set to the empty string", req.EnvVars)
				}
				return
			}
			if err == nil {
				t.Fatalf("buildCommandExecuteRequest() succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr)) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
			}
			if got := exitCodeForError(err); got != ExitCodeUsage {
				t.Fatalf("exitCodeForError(%v) = %d, want %d", err, got, ExitCodeUsage)
			}
		})
	}
}

// TestCheckAmbiguousCommand verifies ambiguity detection across multiple sources.
// It exercises the three key outcomes: no ambiguity (single source), ambiguity
// detected (multiple sources), and zero matches (unknown command).
func TestCheckAmbiguousCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		err       error
		wantErr   bool
		wantAmbig bool
	}{
		{
			name:    "empty args returns nil",
			args:    []string{},
			wantErr: false,
		},
		{
			name: "ambiguous command returns error",
			args: []string{"deploy"},
			err: &commandsvc.ClassifiedError{
				Err: &commandsvc.AmbiguousCommandError{
					CommandName: "deploy",
					Sources:     []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"},
				},
				Kind: commandsvc.ErrorKindCommandAmbiguous,
			},
			wantErr:   true,
			wantAmbig: true,
		},
		{
			name:    "non-ambiguous command returns nil",
			args:    []string{"build"},
			wantErr: false,
		},
		{
			name:    "unknown command returns nil",
			args:    []string{"nonexistent"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := &App{
				Commands:    &fakeAmbiguityCommandService{err: tt.err},
				Diagnostics: &defaultDiagnosticRenderer{},
				stderr:      &bytes.Buffer{},
			}
			err := checkAmbiguousCommand(t.Context(), app, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantAmbig {
					var ambigErr *AmbiguousCommandError
					if !errors.As(err, &ambigErr) {
						t.Fatalf("expected *AmbiguousCommandError, got %T: %v", err, err)
					}
					if string(ambigErr.CommandName) != tt.args[0] {
						t.Errorf("AmbiguousCommandError.CommandName = %q, want %q", ambigErr.CommandName, tt.args[0])
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestCheckAmbiguousCommand_ServiceError verifies that non-ambiguity errors are
// silently swallowed (checkAmbiguousCommand is best-effort).
func TestCheckAmbiguousCommand_ServiceError(t *testing.T) {
	t.Parallel()

	app := &App{
		Commands:    &fakeAmbiguityCommandService{err: errors.New("discovery failed")},
		Diagnostics: &defaultDiagnosticRenderer{},
		stderr:      &bytes.Buffer{},
	}

	err := checkAmbiguousCommand(t.Context(), app, []string{"deploy"})
	if err != nil {
		t.Fatalf("expected nil (discovery errors swallowed), got: %v", err)
	}
}
