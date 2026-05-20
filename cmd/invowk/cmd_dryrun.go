// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const dryRunFieldFmt = "  %s %s\n"

// renderDryRun prints the resolved execution context without executing.
// It shows the command name, source, runtime, platform, working directory,
// script content, and environment variables — everything a user needs to
// understand what invowk would do.
func renderDryRun(w io.Writer, plan commandsvc.DryRunPlan) {
	fmt.Fprintln(w, TitleStyle.Render("Dry Run"))
	fmt.Fprintln(w)

	// Command metadata.
	fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Command:"), plan.CommandName)
	fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Source:"), plan.SourceID)
	fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Runtime:"), string(plan.Runtime))
	fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Platform:"), string(plan.Platform))

	if plan.WorkDir != "" {
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("WorkDir:"), plan.WorkDir)
	}

	if err := plan.Script.Validate(); err != nil {
		fmt.Fprintln(w)
		return
	}

	if plan.Timeout != "" {
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Timeout:"), plan.Timeout)
	}
	if plan.Runtime == invowkfile.RuntimeContainer && plan.PersistentContainerMode != "" {
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("Container:"), plan.PersistentContainerMode)
	}
	if plan.PersistentContainerMode == "persistent" {
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("ContainerName:"), plan.PersistentContainerName)
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("ContainerNameSource:"), plan.PersistentContainerNameSource)
		fmt.Fprintf(w, dryRunFieldFmt, VerboseHighlightStyle.Render("CreateIfMissing:"), strconv.FormatBool(plan.PersistentContainerCreateIfMissing))
	}

	// Script content.
	fmt.Fprintln(w)
	fmt.Fprintln(w, VerboseHighlightStyle.Render("  Script:"))
	if plan.Script.IsFile() {
		fmt.Fprintf(w, "    (file: %s)\n", *plan.Script.File)
	} else {
		for line := range strings.SplitSeq(string(plan.Script.Content), "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
	renderDryRunInterpreter(w, plan)

	// Environment variables, split into metadata (INVOWK_*/ARG*) and user-defined.
	invowkVars := make(map[string]string)
	userVars := make(map[string]string)
	for k, v := range plan.Env {
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

	if plan.DependencyValidationSkipped {
		fmt.Fprintln(w, SubtitleStyle.Render("  Note: dependency validation (tools, cmds, filepaths, capabilities, custom checks, env vars) is not performed in dry-run mode."))
	}
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

func renderDryRunInterpreter(w io.Writer, plan commandsvc.DryRunPlan) {
	if plan.ScriptInterpreter.Provenance() == "" {
		return
	}
	fmt.Fprintf(w, "    Interpreter: %s\n", dryRunInterpreterDescription(plan.Runtime, plan.ScriptInterpreter))
	diagnostics := plan.ScriptInterpreter.Diagnostics()
	for i := range diagnostics {
		diag := diagnostics[i]
		fmt.Fprintf(w, "    warning: %s\n", diag.Message())
	}
}

//goplint:ignore -- CLI rendering helper returns human-readable display text.
func dryRunInterpreterDescription(runtime invowkfile.RuntimeMode, analysis invowkfile.ScriptInterpreterAnalysis) string {
	effective := analysis.Effective()
	if runtime == invowkfile.RuntimeVirtual && (!effective.Found || invowkfile.IsShellInterpreter(effective.Interpreter)) {
		return dryRunVirtualInterpreterDescription(analysis)
	}
	switch analysis.Provenance() {
	case invowkfile.ScriptInterpreterProvenanceExplicit:
		return "explicit: " + effective.CommandString()
	case invowkfile.ScriptInterpreterProvenanceShebang:
		return "shebang-detected: " + effective.CommandString()
	case invowkfile.ScriptInterpreterProvenanceDefaultShell:
		return "default shell behavior"
	default:
		return "default shell behavior"
	}
}

//goplint:ignore -- CLI rendering helper returns human-readable display text.
func dryRunVirtualInterpreterDescription(analysis invowkfile.ScriptInterpreterAnalysis) string {
	effective := analysis.Effective()
	switch analysis.Provenance() {
	case invowkfile.ScriptInterpreterProvenanceExplicit:
		return "virtual shell (embedded mvdan/sh; explicit shell-compatible interpreter: " + effective.CommandString() + ")"
	case invowkfile.ScriptInterpreterProvenanceShebang:
		return "virtual shell (embedded mvdan/sh; shell-compatible shebang: " + effective.CommandString() + ")"
	case invowkfile.ScriptInterpreterProvenanceDefaultShell:
		return "virtual shell (embedded mvdan/sh; default shell behavior)"
	default:
		return "virtual shell (embedded mvdan/sh; default shell behavior)"
	}
}
