// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"maps"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// dagExecutionStackKey is the context key for the re-entrancy guard.
// The value is a map[string]bool tracking which commands are currently
// being executed in the DAG chain, preventing infinite recursion.
type dagExecutionStackKey struct{}

// resolveExecDeps merges dependencies from all levels (root + command + impl),
// resolves OR-alternatives via discovery, and returns a deduplicated list of
// resolved command names in merge order. This is the shared resolution logic
// used by both dry-run (to display which deps would run) and actual execution
// (to run them). Keeping this shared ensures dry-run output always matches
// real execution behaviour.
func (s *commandService) resolveExecDeps(ctx context.Context, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext) ([]string, error) {
	// Guard against nil SelectedImpl and Invowkfile. While ResolveRuntime
	// and discovery currently guarantee non-nil at runtime, these are defensive
	// guards consistent with ValidateExecutionDAG which uses the same pattern.
	var rootDeps *invowkfile.DependsOn
	if cmdInfo.Invowkfile != nil {
		rootDeps = cmdInfo.Invowkfile.DependsOn
	}
	var implDeps *invowkfile.DependsOn
	if execCtx.SelectedImpl != nil {
		implDeps = execCtx.SelectedImpl.DependsOn
	}
	merged := invowkfile.MergeDependsOnAll(
		rootDeps,
		cmdInfo.Command.DependsOn,
		implDeps,
	)
	if merged == nil {
		return nil, nil
	}

	// Deduplicate on the resolved name so the same dep appearing at multiple
	// merge levels (root + command + impl) only runs once. Dedup keys on the
	// resolved command name after OR-alternative resolution to catch equivalent
	// deps with differently-ordered alternative lists.
	seen := make(map[string]bool)
	var names []string
	for _, dep := range merged.GetExecutableCommandDeps() {
		if len(dep.Alternatives) == 0 {
			continue
		}

		depName, err := s.resolveExecutableDep(ctx, dep.Alternatives)
		if err != nil {
			return nil, err
		}

		if seen[depName] {
			continue
		}
		seen[depName] = true
		names = append(names, depName)
	}
	return names, nil
}

// executeDepCommands runs all depends_on.cmds entries that have execute: true,
// in merge order (root -> command -> implementation level, preserving declaration
// order within each level), before the main command executes. Merge order matters
// because deduplication keeps the first occurrence: when the same dep appears at
// root and impl levels, root's declaration wins and impl's duplicate is skipped.
// Each dependency is executed through the full CommandService.Execute pipeline,
// so transitive execute deps are handled recursively. If any dependency fails,
// execution stops and the error is returned.
func (s *commandService) executeDepCommands(ctx context.Context, req ExecuteRequest, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext) error {
	// Check re-entrancy guard early to fail fast on cycles without
	// unnecessary resolution work.
	stack := dagStackFromContext(ctx)
	if stack[req.Name] {
		return fmt.Errorf("dependency cycle detected at runtime: command '%s' is already executing", req.Name)
	}

	depNames, err := s.resolveExecDeps(ctx, cmdInfo, execCtx)
	if err != nil {
		return err
	}
	if len(depNames) == 0 {
		return nil
	}

	// Mark current command as executing using copy-on-write to avoid mutating
	// the parent's stack map. Each recursive Execute() call sees a snapshot of
	// the stack at entry, and the parent's view is unaffected when the child
	// returns. This is structurally safe for future parallelism — no shared
	// mutable state — whereas the previous defer-delete pattern required
	// sequential execution to be correct.
	newStack := make(map[string]bool, len(stack)+1)
	maps.Copy(newStack, stack)
	newStack[req.Name] = true
	ctx = context.WithValue(ctx, dagExecutionStackKey{}, newStack)

	for _, depName := range depNames {
		// Check for cancellation between dependency executions so that
		// Ctrl+C or deadline expiry is honoured promptly between deps.
		if ctx.Err() != nil {
			return fmt.Errorf("dependency execution cancelled: %w", ctx.Err())
		}

		if req.Verbose {
			fmt.Fprintf(s.stdout, "%s Running dependency '%s'...\n", VerboseHighlightStyle.Render("→"), depName)
		}

		// Build a minimal ExecuteRequest for the dependency command.
		// CLI-level overrides (env files, env vars, env inheritance mode, workdir,
		// interactive mode) are intentionally NOT propagated. Each dependency
		// command derives its env configuration from its own CUE definitions,
		// ensuring deps behave consistently regardless of how the parent was invoked.
		// Only config-path, verbose, force-rebuild, and dry-run are propagated
		// because they affect the global execution environment rather than
		// per-command configuration.
		//
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

		result, _, execErr := s.Execute(ctx, depReq)
		if execErr != nil {
			return fmt.Errorf("dependency '%s' failed: %w", depName, execErr)
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

// dagStackFromContext extracts the DAG execution stack from context, or returns nil.
// A nil map is safe for reads (returns false); the caller in executeDepCommands
// creates a new map via copy-on-write when there are deps to execute.
func dagStackFromContext(ctx context.Context) map[string]bool {
	stack, _ := ctx.Value(dagExecutionStackKey{}).(map[string]bool)
	return stack
}
