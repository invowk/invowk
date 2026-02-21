// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// dagExecutionStackKey is the context key for the re-entrancy guard.
// The value is a map[string]bool tracking which commands are currently
// being executed in the DAG chain, preventing infinite recursion.
type dagExecutionStackKey struct{}

// executeDepCommands runs all depends_on.cmds entries that have execute: true,
// in merge order (root -> command -> implementation level, preserving declaration
// order within each level), before the main command executes. Merge order matters
// because deduplication keeps the first occurrence: when the same dep appears at
// root and impl levels, root's declaration wins and impl's duplicate is skipped.
// Each dependency is executed through the full CommandService.Execute pipeline,
// so transitive execute deps are handled recursively. If any dependency fails,
// execution stops and the error is returned.
func (s *commandService) executeDepCommands(ctx context.Context, req ExecuteRequest, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext) error {
	// Collect execute deps from merged depends_on (root + cmd + impl).
	// Guard against nil SelectedImpl: while ResolveRuntime guarantees non-nil
	// at runtime, this is defensive consistency with the timeout nil check in
	// dispatchExecution.
	var implDeps *invowkfile.DependsOn
	if execCtx.SelectedImpl != nil {
		implDeps = execCtx.SelectedImpl.DependsOn
	}
	merged := invowkfile.MergeDependsOnAll(
		cmdInfo.Invowkfile.DependsOn,
		cmdInfo.Command.DependsOn,
		implDeps,
	)
	if merged == nil || !merged.HasExecutableCommandDeps() {
		return nil
	}

	// Get or create the re-entrancy guard from context.
	// When a fresh map is created (first call in the chain), it must be attached
	// to the context via context.WithValue below so recursive Execute() calls
	// see the same map. When an existing map is returned, mutations here are
	// visible through the already-propagated context reference.
	stack := dagStackFromContext(ctx)
	if stack[req.Name] {
		return fmt.Errorf("dependency cycle detected at runtime: command '%s' is already executing", req.Name)
	}
	stack[req.Name] = true
	defer delete(stack, req.Name)

	// Propagate the stack through context for recursive calls.
	ctx = context.WithValue(ctx, dagExecutionStackKey{}, stack)

	// Deduplicate so the same dep appearing at multiple merge levels
	// (root + command + impl) only runs once. Dedup on the resolved command
	// name after OR-alternative resolution to catch equivalent deps with
	// differently-ordered alternative lists.
	executed := make(map[string]bool)
	execDeps := merged.GetExecutableCommandDeps()
	for _, dep := range execDeps {
		// Check for cancellation between dependency executions so that
		// Ctrl+C or deadline expiry is honoured promptly between deps.
		if ctx.Err() != nil {
			return fmt.Errorf("dependency execution cancelled: %w", ctx.Err())
		}

		if len(dep.Alternatives) == 0 {
			continue
		}

		// Resolve which alternative to execute using OR semantics:
		// iterate alternatives in order, execute the first discoverable one.
		// If that execution fails, stop — do NOT fall back to the next alternative.
		depName, resolveErr := s.resolveExecutableDep(ctx, dep.Alternatives)
		if resolveErr != nil {
			return resolveErr
		}

		if executed[depName] {
			continue
		}
		executed[depName] = true

		if req.Verbose {
			fmt.Fprintf(s.stdout, "%s Running dependency '%s'...\n", VerboseHighlightStyle.Render("→"), depName)
		}

		// Build a minimal ExecuteRequest for the dependency command.
		// Static cycle detection (ValidateExecutionDAG via Kahn's algorithm)
		// prevents statically-known cycles; the runtime stack guard above
		// catches dynamic cycles from OR-alternative resolution.
		depReq := ExecuteRequest{
			Name:         depName,
			Verbose:      req.Verbose,
			ForceRebuild: req.ForceRebuild,
			ConfigPath:   req.ConfigPath,
			DryRun:       req.DryRun,
		}

		result, _, err := s.Execute(ctx, depReq)
		if err != nil {
			return fmt.Errorf("dependency '%s' failed: %w", depName, err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("dependency '%s' exited with code %d; run 'invowk cmd %s' independently to diagnose", depName, result.ExitCode, depName)
		}
	}

	return nil
}

// resolveExecutableDep finds the first discoverable command among alternatives.
// This follows the same OR semantics as tool/env/capability dependency validation:
// iterate in order, first discoverable one wins. Unlike non-execute deps where
// discoverability is checked but the command isn't run, here the resolved command
// will actually be executed.
//
// Context cancellation is checked before each discovery attempt so that
// Ctrl+C or deadline expiry propagates promptly instead of being swallowed.
// Non-discovery errors (CUE parse failures, filesystem errors) are propagated
// immediately rather than being masked as "not found".
func (s *commandService) resolveExecutableDep(ctx context.Context, alternatives []string) (string, error) {
	for _, alt := range alternatives {
		// Bail early on context cancellation instead of swallowing it.
		if ctx.Err() != nil {
			return "", fmt.Errorf("execute dependency resolution cancelled: %w", ctx.Err())
		}
		result, err := s.discovery.GetCommand(ctx, alt)
		if err != nil {
			// Propagate non-discovery errors immediately. These indicate real
			// problems (CUE parse failures, filesystem errors, context errors)
			// that should not be masked as "command not found".
			return "", fmt.Errorf("execute dependency resolution for %q failed: %w", alt, err)
		}
		if result.Command != nil {
			return alt, nil
		}
		// Command not found (nil Command, nil error) — try next alternative.
	}
	return "", fmt.Errorf("none of the execute dependency alternatives %v were found", alternatives)
}

// dagStackFromContext extracts the DAG execution stack from context, or returns
// a new empty map. Callers that create a new map must attach it to the context
// via context.WithValue(ctx, dagExecutionStackKey{}, stack) before passing the
// context to recursive Execute() calls.
func dagStackFromContext(ctx context.Context) map[string]bool {
	if stack, ok := ctx.Value(dagExecutionStackKey{}).(map[string]bool); ok {
		return stack
	}
	return make(map[string]bool)
}
