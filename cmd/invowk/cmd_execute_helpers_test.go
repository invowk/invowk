// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
)

// mockDiscoveryService implements DiscoveryService for testing checkAmbiguousCommand.
// It returns a pre-built CommandSetResult without touching the filesystem.
type mockDiscoveryService struct {
	result discovery.CommandSetResult
	err    error
}

func (m *mockDiscoveryService) DiscoverCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return m.result, m.err
}

func (m *mockDiscoveryService) DiscoverAndValidateCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return m.result, m.err
}

func (m *mockDiscoveryService) GetCommand(_ context.Context, _ string) (discovery.LookupResult, error) {
	return discovery.LookupResult{}, nil
}

// TestCreateRuntimeRegistry_SingleContainerInstance verifies the invariant that
// createRuntimeRegistry creates at most one ContainerRuntime instance. The
// ContainerRuntime.runMu mutex provides intra-process serialization as a fallback
// when flock-based cross-process locking is unavailable; multiple instances would
// each have their own mutex, defeating the serialization guarantee.
func TestCreateRuntimeRegistry_SingleContainerInstance(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	result := createRuntimeRegistry(cfg, nil)
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

	// Verify that calling createRuntimeRegistry a second time does not share
	// state with the first registry (each call creates its own instance).
	result2 := createRuntimeRegistry(cfg, nil)
	defer result2.Cleanup()

	rt2, err := result2.Registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return
	}

	if rt == rt2 {
		t.Fatal("two createRuntimeRegistry calls returned the same ContainerRuntime pointer — instances must be independent")
	}
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

	// Build a command set with "deploy" in two sources and "build" in one source.
	set := discovery.NewDiscoveredCommandSet()
	set.BySimpleName["deploy"] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: discovery.SourceIDInvowkfile},
		{SimpleName: "deploy", SourceID: "mymodule"},
	}
	set.BySimpleName["build"] = []*discovery.CommandInfo{
		{SimpleName: "build", SourceID: discovery.SourceIDInvowkfile},
	}
	set.AmbiguousNames["deploy"] = true
	set.BySource[discovery.SourceIDInvowkfile] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: discovery.SourceIDInvowkfile},
		{SimpleName: "build", SourceID: discovery.SourceIDInvowkfile},
	}
	set.BySource["mymodule"] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: "mymodule"},
	}
	set.SourceOrder = []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"}

	mock := &mockDiscoveryService{
		result: discovery.CommandSetResult{Set: set},
	}

	app := &App{
		Discovery:   mock,
		Diagnostics: &defaultDiagnosticRenderer{},
		stderr:      &bytes.Buffer{},
	}
	rootFlags := &rootFlagValues{}

	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		wantAmbig bool
	}{
		{
			name:    "empty args returns nil",
			args:    []string{},
			wantErr: false,
		},
		{
			name:      "ambiguous command returns error",
			args:      []string{"deploy"},
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

			err := checkAmbiguousCommand(context.Background(), app, rootFlags, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantAmbig {
					var ambigErr *AmbiguousCommandError
					if !errors.As(err, &ambigErr) {
						t.Fatalf("expected *AmbiguousCommandError, got %T: %v", err, err)
					}
					if ambigErr.CommandName != tt.args[0] {
						t.Errorf("AmbiguousCommandError.CommandName = %q, want %q", ambigErr.CommandName, tt.args[0])
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestCheckAmbiguousCommand_DiscoveryError verifies that discovery errors are
// silently swallowed (checkAmbiguousCommand is best-effort).
func TestCheckAmbiguousCommand_DiscoveryError(t *testing.T) {
	t.Parallel()

	mock := &mockDiscoveryService{
		err: errors.New("discovery failed"),
	}

	app := &App{
		Discovery:   mock,
		Diagnostics: &defaultDiagnosticRenderer{},
		stderr:      &bytes.Buffer{},
	}

	err := checkAmbiguousCommand(context.Background(), app, &rootFlagValues{}, []string{"deploy"})
	if err != nil {
		t.Fatalf("expected nil (discovery errors swallowed), got: %v", err)
	}
}
