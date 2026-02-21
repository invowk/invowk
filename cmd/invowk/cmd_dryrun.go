// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// collectExecDepNames returns human-readable labels for execute deps.
// For single-alternative deps, returns the name directly. For multi-alternative
// deps, returns "name (alternatives: [a, b])" so dry-run output shows the full
// picture rather than misleadingly showing only the first alternative.
func collectExecDepNames(cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext) []string {
	var implDeps *invowkfile.DependsOn
	if execCtx.SelectedImpl != nil {
		implDeps = execCtx.SelectedImpl.DependsOn
	}
	merged := invowkfile.MergeDependsOnAll(
		cmdInfo.Invowkfile.DependsOn,
		cmdInfo.Command.DependsOn,
		implDeps,
	)
	if merged == nil {
		return nil
	}
	// Deduplicate names so the same dep appearing at multiple merge levels
	// is only shown once, consistent with executeDepCommands dedup.
	seen := make(map[string]bool)
	var names []string
	for _, dep := range merged.GetExecutableCommandDeps() {
		switch len(dep.Alternatives) {
		case 0:
			continue
		case 1:
			if !seen[dep.Alternatives[0]] {
				seen[dep.Alternatives[0]] = true
				names = append(names, dep.Alternatives[0])
			}
		default:
			label := fmt.Sprintf("%s (alternatives: %v)", dep.Alternatives[0], dep.Alternatives)
			if !seen[dep.Alternatives[0]] {
				seen[dep.Alternatives[0]] = true
				names = append(names, label)
			}
		}
	}
	return names
}

// renderDryRun prints the resolved execution context without executing.
// It shows the command name, source, runtime, platform, working directory,
// execute deps, script content, and environment variables — everything a user
// needs to understand what invowk would do. execDepNames lists command names
// that would run before this command via depends_on execute: true.
func renderDryRun(w io.Writer, req ExecuteRequest, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext, resolved appexec.RuntimeSelection, execDepNames []string) {
	fmt.Fprintln(w, TitleStyle.Render("Dry Run"))
	fmt.Fprintln(w)

	// Command metadata.
	fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Command:"), req.Name)
	fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Source:"), cmdInfo.SourceID)
	fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Runtime:"), string(resolved.Mode))
	fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Platform:"), string(invowkfile.GetCurrentHostOS()))

	if execCtx.WorkDir != "" {
		fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("WorkDir:"), execCtx.WorkDir)
	}

	if resolved.Impl == nil {
		fmt.Fprintln(w)
		return
	}

	if resolved.Impl.Timeout != "" {
		fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Timeout:"), resolved.Impl.Timeout)
	}

	if len(execDepNames) > 0 {
		fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Exec deps:"), strings.Join(execDepNames, ", "))
	}

	// Script content.
	fmt.Fprintln(w)
	fmt.Fprintln(w, VerboseHighlightStyle.Render("  Script:"))
	script := resolved.Impl.Script
	if resolved.Impl.IsScriptFile() {
		fmt.Fprintf(w, "    (file: %s)\n", script)
	} else {
		for line := range strings.SplitSeq(script, "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}

	// Environment variables, split into metadata (INVOWK_*/ARG*) and user-defined.
	invowkVars := make(map[string]string)
	userVars := make(map[string]string)
	for k, v := range execCtx.Env.ExtraEnv {
		if strings.HasPrefix(k, "INVOWK_") || isArgEnvVar(k) {
			invowkVars[k] = v
		} else {
			userVars[k] = v
		}
	}
	if len(invowkVars) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, VerboseHighlightStyle.Render("  Environment (INVOWK_* / ARG*):"))
		for _, k := range slices.Sorted(maps.Keys(invowkVars)) {
			fmt.Fprintf(w, "    %s=%s\n", k, invowkVars[k])
		}
	}
	if len(userVars) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, VerboseHighlightStyle.Render("  Environment (user-defined):"))
		for _, k := range slices.Sorted(maps.Keys(userVars)) {
			fmt.Fprintf(w, "    %s=%s\n", k, userVars[k])
		}
	}

	fmt.Fprintln(w)
}

// isArgEnvVar checks if a key matches the ARG1, ARG2, ..., ARGC pattern
// used by the positional argument projection system.
func isArgEnvVar(k string) bool {
	if k == "ARGC" {
		return true
	}
	if len(k) < 4 || k[:3] != "ARG" {
		return false
	}
	// Remaining chars must all be digits (ARG1, ARG2, ..., ARG99, etc.)
	for i := 3; i < len(k); i++ {
		if k[i] < '0' || k[i] > '9' {
			return false
		}
	}
	return true
}
