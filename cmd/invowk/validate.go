// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"

	"github.com/spf13/cobra"
)

const (
	pathTypeUnknown    pathType = iota
	pathTypeInvowkfile          // invowkfile.cue or directory containing one
	pathTypeModule              // *.invowkmod directory or invowkmod.cue file
)

// pathType represents the detected type of a filesystem path for validation routing.
type pathType int

// newValidateCommand creates the `invowk validate` command.
// Without arguments, it runs workspace-wide discovery validation.
// With a path argument, it auto-detects invowkfile vs module and validates accordingly.
func newValidateCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate invowkfiles and modules",
		Long: `Validate invowkfiles, modules, or the entire workspace.

Without arguments, validates the current workspace by running full discovery
and reporting all diagnostics from every search path.

With a path argument, auto-detects the target type:
  - *.invowkmod directory   → module validation
  - invowkfile.cue file     → invowkfile validation
  - directory with invowkfile.cue → invowkfile validation
  - invowkmod.cue file      → module validation (parent directory)

Module validation always includes invowkfile parsing and command tree validation.

Examples:
  invowk validate                              Validate workspace
  invowk validate ./invowkfile.cue             Validate a single invowkfile
  invowk validate ./mymod.invowkmod            Validate a module`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetContext(contextWithConfigPath(cmd.Context(), ""))

			if len(args) == 0 {
				return runWorkspaceValidation(cmd, app)
			}

			return runPathValidation(cmd, args[0])
		},
	}
}

// runWorkspaceValidation performs full discovery validation and renders styled results.
// It reports both discovery diagnostics and tree validation errors in a single pass
// so users see all issues at once rather than having to fix-and-rerun iteratively.
func runWorkspaceValidation(cmd *cobra.Command, app *App) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	result, err := app.Discovery.DiscoverAndValidateCommandSet(cmd.Context())

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		cwd = "(unable to determine working directory)"
	}
	fmt.Fprintln(stdout, moduleTitleStyle.Render("Workspace Validation"))
	fmt.Fprintf(stdout, "%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(cwd))
	fmt.Fprintln(stdout)

	if err != nil && result.Set == nil {
		fmt.Fprintf(stderr, "%s Discovery failed: %s\n", moduleErrorIcon, err)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	// Show discovery summary.
	if result.Set != nil {
		fmt.Fprintf(stdout, "%s %d source(s) discovered, %d command(s) found\n",
			moduleSuccessIcon,
			len(result.Set.SourceOrder),
			len(result.Set.Commands),
		)
	} else {
		fmt.Fprintf(stdout, "%s Configuration loaded\n", moduleSuccessIcon)
	}

	// Collect all issues (diagnostics + tree errors) and render in a single pass.
	hasIssues := false

	if len(result.Diagnostics) > 0 {
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "%s %d diagnostic issue(s) found:\n", WarningStyle.Render("!"), len(result.Diagnostics))
		fmt.Fprintln(stderr)

		for i, diag := range result.Diagnostics {
			issueNum := fmt.Sprintf("  %d.", i+1)
			codeTag := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", diag.Code))
			if diag.Path != "" {
				fmt.Fprintf(stderr, "%s %s %s\n", issueNum, codeTag, modulePathStyle.Render(diag.Path))
				fmt.Fprintf(stderr, "     %s\n", diag.Message)
			} else {
				fmt.Fprintf(stderr, "%s %s %s\n", issueNum, codeTag, diag.Message)
			}
		}
		hasIssues = true
	}

	// Tree validation errors (ArgsSubcommandConflictError etc.) are separate from
	// discovery diagnostics and must be shown alongside them.
	if err != nil {
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "%s %s\n", moduleErrorIcon, err)
		hasIssues = true
	}

	if hasIssues {
		issueCount := len(result.Diagnostics)
		if err != nil {
			issueCount++
		}
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "%s Validation failed with %d issue(s)\n", moduleErrorIcon, issueCount)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%s Workspace is valid\n", moduleSuccessIcon)
	return nil
}

// runPathValidation validates a single path, auto-detecting its type.
func runPathValidation(cmd *cobra.Command, targetPath string) error {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	pt, resolvedPath := detectPathType(absPath)

	switch pt {
	case pathTypeModule:
		return runModulePathValidation(cmd, resolvedPath)
	case pathTypeInvowkfile:
		return runInvowkfilePathValidation(cmd, resolvedPath)
	case pathTypeUnknown:
		return fmt.Errorf("cannot determine file type for %s: expected invowkfile.cue, *.invowkmod directory, or directory containing invowkfile.cue", targetPath)
	default:
		return fmt.Errorf("internal error: unhandled path type %d for %s", pt, targetPath)
	}
}

// detectPathType determines whether a path is an invowkfile, module, or unknown.
// It returns the detected type and the resolved path to validate.
func detectPathType(absPath string) (detected pathType, resolvedPath string) {
	base := filepath.Base(absPath)

	// Directory ending in .invowkmod → module
	if strings.HasSuffix(base, ".invowkmod") {
		return pathTypeModule, absPath
	}

	// File named invowkfile.cue → invowkfile
	if base == "invowkfile.cue" {
		return pathTypeInvowkfile, absPath
	}

	// File named invowkmod.cue → module (parent directory)
	if base == "invowkmod.cue" {
		return pathTypeModule, filepath.Dir(absPath)
	}

	// Directory containing invowkfile.cue → invowkfile
	candidate := filepath.Join(absPath, "invowkfile.cue")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return pathTypeInvowkfile, candidate
	}

	return pathTypeUnknown, absPath
}

// runInvowkfilePathValidation validates a single invowkfile and renders styled output.
func runInvowkfilePathValidation(cmd *cobra.Command, invowkfilePath string) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	absPath, err := filepath.Abs(invowkfilePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Fprintln(stdout, moduleTitleStyle.Render("Invowkfile Validation"))
	fmt.Fprintf(stdout, "%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Fprintln(stdout)

	// Parse the invowkfile (CUE schema + structural validation).
	inv, err := invowkfile.Parse(invowkfilePath)
	if err != nil {
		fmt.Fprintf(stderr, "%s CUE schema validation failed\n", moduleErrorIcon)
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "  %s\n", err)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Fprintf(stdout, "%s CUE schema validation passed\n", moduleSuccessIcon)

	// invowkfile.Parse() includes structural validation as part of CUE parsing.
	fmt.Fprintf(stdout, "%s Structural validation passed\n", moduleSuccessIcon)

	// Command tree validation (leaf-only args constraint).
	var commands []*discovery.CommandInfo
	for name, cmdDef := range inv.FlattenCommands() {
		commands = append(commands, &discovery.CommandInfo{
			Name:       name,
			FilePath:   invowkfilePath,
			Command:    cmdDef,
			Invowkfile: inv,
		})
	}

	if treeErr := discovery.ValidateCommandTree(commands); treeErr != nil {
		fmt.Fprintf(stderr, "%s Command tree validation failed\n", moduleErrorIcon)
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "  %s\n", treeErr)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Fprintf(stdout, "%s Command tree validation passed\n", moduleSuccessIcon)

	if dagErr := discovery.ValidateExecutionDAG(commands); dagErr != nil {
		fmt.Fprintf(stderr, "%s Execution DAG validation failed\n", moduleErrorIcon)
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "  %s\n", dagErr)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Fprintf(stdout, "%s Execution DAG validation passed\n", moduleSuccessIcon)

	cmdCount := len(commands)
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%s Invowkfile is valid (%d command(s))\n", moduleSuccessIcon, cmdCount)
	return nil
}

// runModulePathValidation validates a module directory and renders styled output.
// It calls invowkmod.Validate() for structural checks, then performs deep validation
// by parsing the module's invowkfile (if present) and validating the command tree.
func runModulePathValidation(cmd *cobra.Command, modulePath string) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Fprintln(stdout, moduleTitleStyle.Render("Module Validation"))
	fmt.Fprintf(stdout, "%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))

	result, err := invowkmod.Validate(modulePath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if result.ModuleName != "" {
		fmt.Fprintf(stdout, "%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(result.ModuleName))
	}

	// Deep validation: parse invowkfile and validate command tree if present.
	if result.InvowkfilePath != "" {
		inv, invErr := invowkfile.Parse(result.InvowkfilePath)
		if invErr != nil {
			result.AddIssue(invowkmod.IssueTypeInvowkfile, invErr.Error(), "invowkfile.cue")
		} else if inv != nil {
			var commands []*discovery.CommandInfo
			for name, cmdDef := range inv.FlattenCommands() {
				commands = append(commands, &discovery.CommandInfo{
					Name:       name,
					FilePath:   result.InvowkfilePath,
					Command:    cmdDef,
					Invowkfile: inv,
				})
			}
			if treeErr := discovery.ValidateCommandTree(commands); treeErr != nil {
				result.AddIssue(invowkmod.IssueTypeCommandTree, treeErr.Error(), result.InvowkfilePath)
			}
			if dagErr := discovery.ValidateExecutionDAG(commands); dagErr != nil {
				result.AddIssue(invowkmod.IssueTypeCommandTree, dagErr.Error(), result.InvowkfilePath)
			}
		}
	}

	fmt.Fprintln(stdout)

	if result.Valid {
		fmt.Fprintf(stdout, "%s Module is valid\n", moduleSuccessIcon)
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "%s Structure check passed\n", moduleSuccessIcon)
		fmt.Fprintf(stdout, "%s Naming convention check passed\n", moduleSuccessIcon)
		fmt.Fprintf(stdout, "%s Required files present\n", moduleSuccessIcon)
		if result.InvowkfilePath != "" {
			fmt.Fprintf(stdout, "%s Invowkfile parses successfully\n", moduleSuccessIcon)
		}
		return nil
	}

	fmt.Fprintf(stderr, "%s Module validation failed with %d issue(s)\n", moduleErrorIcon, len(result.Issues))
	fmt.Fprintln(stderr)

	for i, iss := range result.Issues {
		issueNum := fmt.Sprintf("%d.", i+1)
		issueType := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", iss.Type))
		if iss.Path != "" {
			fmt.Fprintf(stderr, "%s %s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, modulePathStyle.Render(iss.Path), iss.Message)
		} else {
			fmt.Fprintf(stderr, "%s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, iss.Message)
		}
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return &ExitError{Code: 1}
}
