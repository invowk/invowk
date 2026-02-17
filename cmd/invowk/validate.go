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

Examples:
  invowk validate                              Validate workspace
  invowk validate ./invowkfile.cue             Validate a single invowkfile
  invowk validate ./mymod.invowkmod            Validate a module
  invowk validate ./mymod.invowkmod --deep     Validate a module with deep checks`,
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
func runWorkspaceValidation(cmd *cobra.Command, app *App) error {
	result, err := app.Discovery.DiscoverAndValidateCommandSet(cmd.Context())

	cwd, _ := os.Getwd()
	fmt.Println(moduleTitleStyle.Render("Workspace Validation"))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(cwd))
	fmt.Println()

	if err != nil && result.Set == nil {
		fmt.Printf("%s Discovery failed: %s\n", moduleErrorIcon, err)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	// Show discovery summary.
	if result.Set != nil {
		fmt.Printf("%s %d source(s) discovered, %d command(s) found\n",
			moduleSuccessIcon,
			len(result.Set.SourceOrder),
			len(result.Set.Commands),
		)
	} else {
		fmt.Printf("%s Configuration loaded\n", moduleSuccessIcon)
	}

	// Show diagnostics.
	if len(result.Diagnostics) > 0 {
		fmt.Println()
		fmt.Printf("%s %d issue(s) found:\n", WarningStyle.Render("!"), len(result.Diagnostics))
		fmt.Println()

		for i, diag := range result.Diagnostics {
			issueNum := fmt.Sprintf("  %d.", i+1)
			codeTag := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", diag.Code))
			if diag.Path != "" {
				fmt.Printf("%s %s %s\n", issueNum, codeTag, modulePathStyle.Render(diag.Path))
				fmt.Printf("     %s\n", diag.Message)
			} else {
				fmt.Printf("%s %s %s\n", issueNum, codeTag, diag.Message)
			}
		}

		fmt.Println()
		fmt.Printf("%s Validation failed with %d issue(s)\n", moduleErrorIcon, len(result.Diagnostics))
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	// Check for tree validation errors separately (ArgsSubcommandConflictError etc.).
	if err != nil {
		fmt.Println()
		fmt.Printf("%s %s\n", moduleErrorIcon, err)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Println()
	fmt.Printf("%s Workspace is valid\n", moduleSuccessIcon)
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
	}

	return nil
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
	absPath, _ := filepath.Abs(invowkfilePath)

	fmt.Println(moduleTitleStyle.Render("Invowkfile Validation"))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Println()

	// Parse the invowkfile (CUE schema + structural validation).
	inv, err := invowkfile.Parse(invowkfilePath)
	if err != nil {
		fmt.Printf("%s CUE schema validation failed\n", moduleErrorIcon)
		fmt.Println()
		fmt.Printf("  %s\n", err)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Printf("%s CUE schema validation passed\n", moduleSuccessIcon)

	// Structural validation (invowkfile.Parse already runs this, but we confirm).
	fmt.Printf("%s Structural validation passed\n", moduleSuccessIcon)

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
		fmt.Printf("%s Command tree validation failed\n", moduleErrorIcon)
		fmt.Println()
		fmt.Printf("  %s\n", treeErr)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &ExitError{Code: 1}
	}

	fmt.Printf("%s Command tree validation passed\n", moduleSuccessIcon)

	cmdCount := len(inv.FlattenCommands())
	fmt.Println()
	fmt.Printf("%s Invowkfile is valid (%d command(s))\n", moduleSuccessIcon, cmdCount)
	return nil
}

// runModulePathValidation validates a module directory and renders styled output.
// Delegates to invowkmod.Validate() with deep parsing for invowkfile content.
func runModulePathValidation(cmd *cobra.Command, modulePath string) error {
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Println(moduleTitleStyle.Render("Module Validation"))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))

	result, err := invowkmod.Validate(modulePath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if result.ModuleName != "" {
		fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(result.ModuleName))
	}

	// Always perform deep validation (parse invowkfile if present).
	if result.InvowkfilePath != "" {
		inv, invErr := invowkfile.Parse(result.InvowkfilePath)
		if invErr != nil {
			result.AddIssue("invowkfile", invErr.Error(), "invowkfile.cue")
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
				result.AddIssue("command_tree", treeErr.Error(), result.InvowkfilePath)
			}
		}
	}

	fmt.Println()

	if result.Valid {
		fmt.Printf("%s Module is valid\n", moduleSuccessIcon)
		fmt.Println()
		fmt.Printf("%s Structure check passed\n", moduleSuccessIcon)
		fmt.Printf("%s Naming convention check passed\n", moduleSuccessIcon)
		fmt.Printf("%s Required files present\n", moduleSuccessIcon)
		if result.InvowkfilePath != "" {
			fmt.Printf("%s Invowkfile parses successfully\n", moduleSuccessIcon)
		}
		return nil
	}

	fmt.Printf("%s Module validation failed with %d issue(s)\n", moduleErrorIcon, len(result.Issues))
	fmt.Println()

	for i, iss := range result.Issues {
		issueNum := fmt.Sprintf("%d.", i+1)
		issueType := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", iss.Type))
		if iss.Path != "" {
			fmt.Printf("%s %s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, modulePathStyle.Render(iss.Path), iss.Message)
		} else {
			fmt.Printf("%s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, iss.Message)
		}
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return &ExitError{Code: 1}
}
