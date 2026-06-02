// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestCheckCapabilityDependencies(t *testing.T) {
	t.Parallel()

	t.Run("nil deps", testCheckCapabilityDependenciesNilDeps)
	t.Run("empty capabilities", testCheckCapabilityDependenciesEmptyCapabilities)
	t.Run("injected checker accepts alternative", testCheckCapabilityDependenciesInjectedCheckerAcceptsAlternative)
	t.Run("injected checker reports missing alternative", testCheckCapabilityDependenciesInjectedCheckerReportsMissingAlternative)
	t.Run("injected checker receives request scoped context and io", testCheckCapabilityDependenciesInjectedCheckerReceivesContextAndIO)
}

func testCheckCapabilityDependenciesNilDeps(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	if err := CheckCapabilityDependencies(nil, ctx); err != nil {
		t.Fatalf("CheckCapabilityDependencies() = %v, want nil", err)
	}
}

func testCheckCapabilityDependenciesEmptyCapabilities(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	deps := &invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{}}
	if err := CheckCapabilityDependencies(deps, ctx); err != nil {
		t.Fatalf("CheckCapabilityDependencies() = %v, want nil", err)
	}
}

func testCheckCapabilityDependenciesInjectedCheckerAcceptsAlternative(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	deps := dependsOnCapability(invowkfile.CapabilityInternet)
	if err := CheckCapabilityDependenciesWithChecker(deps, ctx, fakeCapabilityChecker{}); err != nil {
		t.Fatalf("CheckCapabilityDependenciesWithChecker() = %v, want nil", err)
	}
}

func testCheckCapabilityDependenciesInjectedCheckerReportsMissingAlternative(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	deps := dependsOnCapability(invowkfile.CapabilityInternet)
	checker := fakeCapabilityChecker{
		invowkfile.CapabilityInternet: &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityInternet,
			Message:    "offline",
		},
	}

	err := CheckCapabilityDependenciesWithChecker(deps, ctx, checker)
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
	if len(depErr.MissingCapabilities) != 1 {
		t.Fatalf("missing capabilities = %d, want 1", len(depErr.MissingCapabilities))
	}
}

func testCheckCapabilityDependenciesInjectedCheckerReceivesContextAndIO(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	ioCtx := IOContext{Stdout: stdout, Stderr: stderr}
	ctx := newDependencyExecutionContext(t)
	ctx.Context = t.Context()
	ctx.IO = ioCtx
	deps := dependsOnCapability(invowkfile.CapabilityTTY)
	checker := &recordingCapabilityChecker{}

	if err := CheckCapabilityDependenciesWithChecker(deps, ctx, checker); err != nil {
		t.Fatalf("CheckCapabilityDependenciesWithChecker() = %v, want nil", err)
	}
	if len(checker.requests) != 1 {
		t.Fatalf("recorded requests = %d, want 1", len(checker.requests))
	}
	got := checker.requests[0]
	if got.ctx != ctx.Context {
		t.Fatal("capability checker did not receive execution context")
	}
	if got.ioCtx.Stdout != stdout || got.ioCtx.Stderr != stderr {
		t.Fatal("capability checker did not receive execution IO")
	}
	if got.capability != invowkfile.CapabilityTTY {
		t.Fatalf("Capability = %q, want %q", got.capability, invowkfile.CapabilityTTY)
	}
}

func dependsOnCapability(capability invowkfile.CapabilityName) *invowkfile.DependsOn {
	return &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{
			{Alternatives: []invowkfile.CapabilityName{capability}},
		},
	}
}
