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
	fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("Platform:"), string(execCtx.Command.GetSupportedPlatforms()[0]))

	if execCtx.WorkDir != "" {
		fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("WorkDir:"), execCtx.WorkDir)
	}

	if resolved.Impl != nil && resolved.Impl.Timeout != "" {
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

	// Environment variables (INVOWK_* only for brevity).
	invowkVars := make(map[string]string)
	for k, v := range execCtx.Env.ExtraEnv {
		if strings.HasPrefix(k, "INVOWK_") || k == "ARGC" || (len(k) > 3 && k[:3] == "ARG") {
			invowkVars[k] = v
		}
	}
	if len(invowkVars) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, VerboseHighlightStyle.Render("  Environment (INVOWK_*):"))
		for _, k := range slices.Sorted(maps.Keys(invowkVars)) {
			fmt.Fprintf(w, "    %s=%s\n", k, invowkVars[k])
		}
	}

	fmt.Fprintln(w)
}
