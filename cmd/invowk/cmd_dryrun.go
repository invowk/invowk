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

// renderDryRun prints the resolved execution context without executing.
// It shows the command name, source, runtime, platform, working directory,
// script content, and environment variables — everything a user needs to
// understand what invowk would do.
func renderDryRun(w io.Writer, req ExecuteRequest, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext, resolved appexec.RuntimeSelection) {
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

	fmt.Fprintln(w, SubtitleStyle.Render("  Note: dependency validation (tools, cmds, filepaths, capabilities, custom checks, env vars) is not performed in dry-run mode."))
	fmt.Fprintln(w)
}

// isArgEnvVar checks if a key matches the ARG1, ARG2, ..., ARGC pattern
// used by the positional argument projection system.
func isArgEnvVar(k string) bool {
	if k == "ARGC" {
		return true
	}
	rest, ok := strings.CutPrefix(k, "ARG")
	if !ok || rest == "" {
		return false
	}
	// Remaining chars must all be digits (ARG1, ARG2, ..., ARG99, etc.)
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
