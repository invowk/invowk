// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
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

func (s *fakeAmbiguityCommandService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return &discovery.CommandInfo{Name: "build", SimpleName: "build"}, req, nil, nil
}

// TestCreateRuntimeRegistry_SingleContainerInstance verifies that registry setup
// creates at most one ContainerRuntime instance per execution so cleanup and
// provisioning state stay scoped to that execution.
func TestCreateRuntimeRegistry_SingleContainerInstance(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	result := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer result.Cleanup()

	// If the container engine is available, verify it's registered exactly once
	// by retrieving it and confirming it's a *ContainerRuntime.
	rt, err := result.Registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		// Container engine not available in this environment — the invariant
		// is trivially satisfied (zero instances).
		t.Logf("container runtime not available: %v (invariant trivially satisfied)", err)
		return
	}

	if _, ok := rt.(*runtime.ContainerRuntime); !ok {
		t.Fatalf("expected *runtime.ContainerRuntime, got %T", rt)
	}

	// Verify that calling CreateRuntimeRegistry a second time does not share
	// state with the first registry (each call creates its own instance).
	result2 := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer result2.Cleanup()

	rt2, err := result2.Registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return
	}

	if rt == rt2 {
		t.Fatal("two CreateRuntimeRegistry calls returned the same ContainerRuntime pointer — instances must be independent")
	}
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
// including edge cases like empty values, values containing '=', and
// malformed entries that should be skipped.
func TestParseEnvVarFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  map[string]string
	}{
		{
			name:  "nil input returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty slice returns nil",
			input: []string{},
			want:  nil,
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
			name:  "empty value after equals",
			input: []string{"FOO="},
			want:  map[string]string{"FOO": ""},
		},
		{
			name:  "value containing equals sign",
			input: []string{"FOO=bar=baz"},
			want:  map[string]string{"FOO": "bar=baz"},
		},
		{
			name:  "missing equals skipped",
			input: []string{"INVALID"},
			want:  nil,
		},
		{
			name:  "equals at start skipped",
			input: []string{"=value"},
			want:  nil,
		},
		{
			name:  "mix of valid and invalid",
			input: []string{"GOOD=val", "BAD", "ALSO_GOOD="},
			want:  map[string]string{"GOOD": "val", "ALSO_GOOD": ""},
		},
		{
			name:  "all invalid returns nil",
			input: []string{"NO_EQUALS", "=leading", ""},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseEnvVarFlags(tt.input)

			if tt.want == nil {
				if got != nil {
					t.Fatalf("parseEnvVarFlags() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("parseEnvVarFlags() = nil, want %v", tt.want)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("parseEnvVarFlags() has %d entries, want %d", len(got), len(tt.want))
			}

			for k, wantV := range tt.want {
				gotV, exists := got[k]
				if !exists {
					t.Errorf("missing key %q", k)
					continue
				}
				if gotV != wantV {
					t.Errorf("key %q = %q, want %q", k, gotV, wantV)
				}
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
