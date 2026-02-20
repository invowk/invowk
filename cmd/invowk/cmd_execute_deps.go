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
// order within each level), before the main command executes. Each dependency is
// executed through the full CommandService.Execute pipeline, so transitive
// execute deps are handled recursively. If any dependency fails, execution
// stops and the error is returned.
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
	// dagStackFromContext returns a map (reference type), so mutations here
	// are visible through the context propagated to recursive Execute() calls.
	stack := dagStackFromContext(ctx)
	if stack[req.Name] {
		return fmt.Errorf("dependency cycle detected at runtime: command '%s' is already executing", req.Name)
	}
	stack[req.Name] = true
	defer delete(stack, req.Name)

	// Propagate the stack through context for recursive calls.
	ctx = context.WithValue(ctx, dagExecutionStackKey{}, stack)

	execDeps := merged.GetExecutableCommandDeps()
	for _, dep := range execDeps {
		if len(dep.Alternatives) == 0 {
			continue
		}

		// Resolve which alternative to execute using OR semantics:
		// iterate alternatives in order, execute the first discoverable one.
		// If that execution fails, stop — do NOT fall back to the next alternative.
		depName, resolveErr := s.resolveExecutableDep(ctx, req, dep.Alternatives)
		if resolveErr != nil {
			return resolveErr
		}

		if req.Verbose {
			fmt.Fprintf(s.stdout, "%s Running dependency '%s'...\n", VerboseHighlightStyle.Render("→"), depName)
		}

		// Build a minimal ExecuteRequest for the dependency command.
		// No args, no flags — execute deps run with defaults only.
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
			return fmt.Errorf("dependency '%s' exited with code %d", depName, result.ExitCode)
		}
	}

	return nil
}

// resolveExecutableDep finds the first discoverable command among alternatives.
// This follows the same OR semantics as tool/env/capability dependency validation:
// iterate in order, first discoverable one wins. Unlike non-execute deps where
// discoverability is checked but the command isn't run, here the resolved command
// will actually be executed.
func (s *commandService) resolveExecutableDep(ctx context.Context, req ExecuteRequest, alternatives []string) (string, error) {
	for _, alt := range alternatives {
		result, err := s.discovery.GetCommand(ctx, alt)
		if err != nil {
			continue
		}
		if result.Command != nil {
			return alt, nil
		}
	}
	return "", fmt.Errorf("none of the execute dependency alternatives %v were found", alternatives)
}

// dagStackFromContext extracts the DAG execution stack from context, or creates
// a new one if not present.
func dagStackFromContext(ctx context.Context) map[string]bool {
	if stack, ok := ctx.Value(dagExecutionStackKey{}).(map[string]bool); ok {
		return stack
	}
	return make(map[string]bool)
}
