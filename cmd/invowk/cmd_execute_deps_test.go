// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// lookupDiscoveryServiceFunc is a test mock that delegates GetCommand to a function.
type lookupDiscoveryServiceFunc struct {
	getCommand func(ctx context.Context, name string) (discovery.LookupResult, error)
}

func (m *lookupDiscoveryServiceFunc) DiscoverCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{}, nil
}

func (m *lookupDiscoveryServiceFunc) DiscoverAndValidateCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{}, nil
}

func (m *lookupDiscoveryServiceFunc) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	return m.getCommand(ctx, name)
}

func TestDagStackFromContext_NilContextValue(t *testing.T) {
	t.Parallel()
	stack := dagStackFromContext(context.Background())
	if stack == nil {
		t.Fatal("expected non-nil map from fresh context")
	}
	if len(stack) != 0 {
		t.Errorf("expected empty map, got %v", stack)
	}
}

func TestDagStackFromContext_ExistingStack(t *testing.T) {
	t.Parallel()
	existing := map[string]bool{"cmd-a": true}
	ctx := context.WithValue(context.Background(), dagExecutionStackKey{}, existing)
	stack := dagStackFromContext(ctx)
	if !stack["cmd-a"] {
		t.Fatal("expected existing stack entry to be preserved")
	}
	// Verify it's the same map instance (mutations are shared).
	stack["cmd-b"] = true
	if !existing["cmd-b"] {
		t.Fatal("expected mutations to be visible through original map reference")
	}
}

func TestResolveExecutableDep_FirstDiscoverable(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
		discovery: &lookupDiscoveryServiceFunc{
			getCommand: func(_ context.Context, name string) (discovery.LookupResult, error) {
				switch name {
				case "preferred":
					// Not found: nil Command, nil error — signals "not discoverable".
					return discovery.LookupResult{}, nil
				case "fallback":
					return discovery.LookupResult{Command: &discovery.CommandInfo{Name: "fallback"}}, nil
				default:
					return discovery.LookupResult{}, fmt.Errorf("unknown command: %s", name)
				}
			},
		},
	}

	resolved, err := svc.resolveExecutableDep(context.Background(), []string{"preferred", "fallback"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "fallback" {
		t.Errorf("expected 'fallback', got %q", resolved)
	}
}

func TestResolveExecutableDep_AllNotFound(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
		discovery: &lookupDiscoveryServiceFunc{
			getCommand: func(_ context.Context, _ string) (discovery.LookupResult, error) {
				// Not found: nil Command, nil error.
				return discovery.LookupResult{}, nil
			},
		},
	}

	_, err := svc.resolveExecutableDep(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error when all alternatives not found")
	}
	if got := err.Error(); !strings.Contains(got, "were found") {
		t.Errorf("expected 'were found' in message, got: %s", got)
	}
}

func TestResolveExecutableDep_DiscoveryErrorPropagated(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
		discovery: &lookupDiscoveryServiceFunc{
			getCommand: func(_ context.Context, name string) (discovery.LookupResult, error) {
				return discovery.LookupResult{}, fmt.Errorf("CUE parse error for %s", name)
			},
		},
	}

	// Non-discovery errors propagate immediately instead of being masked.
	_, err := svc.resolveExecutableDep(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error to propagate from discovery")
	}
	// Should propagate the first error, not try subsequent alternatives.
	if got := err.Error(); !strings.Contains(got, "CUE parse error for a") {
		t.Errorf("expected first error propagated, got: %s", got)
	}
}

func TestResolveExecutableDep_ContextCancelled(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
		discovery: &lookupDiscoveryServiceFunc{
			getCommand: func(_ context.Context, _ string) (discovery.LookupResult, error) {
				t.Fatal("GetCommand should not be called after context cancellation")
				return discovery.LookupResult{}, nil
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := svc.resolveExecutableDep(ctx, []string{"cmd"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestExecuteDepCommands_NoDeps(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
	}
	cmd := &invowkfile.Command{Name: "test"}
	inv := &invowkfile.Invowkfile{}
	cmdInfo := &discovery.CommandInfo{
		Command:    cmd,
		Invowkfile: inv,
	}
	execCtx := runtime.NewExecutionContext(cmd, inv)
	err := svc.executeDepCommands(context.Background(), ExecuteRequest{Name: "test"}, cmdInfo, execCtx)
	if err != nil {
		t.Fatalf("unexpected error for command with no deps: %v", err)
	}
}

func TestExecuteDepCommands_RuntimeCycleDetected(t *testing.T) {
	t.Parallel()
	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
	}

	cmd := &invowkfile.Command{
		Name: "cyclic",
		DependsOn: &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []string{"cyclic"}, Execute: true},
			},
		},
	}
	inv := &invowkfile.Invowkfile{}
	cmdInfo := &discovery.CommandInfo{
		Command:    cmd,
		Invowkfile: inv,
	}

	// Pre-populate the context stack with "cyclic" to simulate it already executing.
	stack := map[string]bool{"cyclic": true}
	ctx := context.WithValue(context.Background(), dagExecutionStackKey{}, stack)

	execCtx := runtime.NewExecutionContext(cmd, inv)
	err := svc.executeDepCommands(ctx, ExecuteRequest{Name: "cyclic"}, cmdInfo, execCtx)
	if err == nil {
		t.Fatal("expected runtime cycle detection error")
	}
	if got := err.Error(); !strings.Contains(got, "dependency cycle detected at runtime") {
		t.Errorf("expected cycle error message, got: %s", got)
	}
}

func TestExecuteDepCommands_CancelledContextBetweenDeps(t *testing.T) {
	t.Parallel()

	svc := &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
	}

	// Two execute deps — but context is already cancelled so the loop
	// should bail before attempting to resolve either one.
	cmd := &invowkfile.Command{
		Name: "test",
		DependsOn: &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []string{"dep-a"}, Execute: true},
				{Alternatives: []string{"dep-b"}, Execute: true},
			},
		},
	}
	inv := &invowkfile.Invowkfile{}
	cmdInfo := &discovery.CommandInfo{
		Command:    cmd,
		Invowkfile: inv,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	execCtx := runtime.NewExecutionContext(cmd, inv)
	err := svc.executeDepCommands(ctx, ExecuteRequest{Name: "test"}, cmdInfo, execCtx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
