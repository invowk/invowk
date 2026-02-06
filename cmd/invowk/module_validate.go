// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"

	"github.com/spf13/cobra"
)

// newModuleValidateCommand creates the `invowk module validate` command.
func newModuleValidateCommand() *cobra.Command {
	var deep bool

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an invowk module",
		Long: `Validate the structure and contents of an invowk module.

Checks performed:
  - Folder name follows module naming conventions
  - Contains required invkmod.cue at the root
  - No nested modules inside
  - (with --deep) Invkfile parses successfully (if present)

Examples:
  invowk module validate ./mycommands.invkmod
  invowk module validate ./com.example.tools.invkmod --deep`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleValidate(args, deep)
		},
	}

	cmd.Flags().BoolVar(&deep, "deep", false, "perform deep validation including invkfile parsing")

	return cmd
}

func runModuleValidate(args []string, deep bool) error {
	modulePath := args[0]

	// Convert to absolute path for display
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Println(moduleTitleStyle.Render("Module Validation"))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))

	// Perform validation
	result, err := invkmod.Validate(modulePath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Display module name if parsed successfully
	if result.ModuleName != "" {
		fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(result.ModuleName))
	}

	// Deep validation: parse invkfile
	if deep && result.InvkfilePath != "" {
		inv, invkfileError := invkfile.Parse(result.InvkfilePath)
		if invkfileError != nil {
			result.AddIssue("invkfile", invkfileError.Error(), "invkfile.cue")
		} else if inv != nil {
			// Validate command tree structure (leaf-only args constraint)
			var commands []*discovery.CommandInfo
			for name, cmd := range inv.FlattenCommands() {
				commands = append(commands, &discovery.CommandInfo{
					Name:     name,
					FilePath: result.InvkfilePath,
					Command:  cmd,
					Invkfile: inv,
				})
			}
			if treeErr := discovery.ValidateCommandTree(commands); treeErr != nil {
				result.AddIssue("command_tree", treeErr.Error(), result.InvkfilePath)
			}
		}
	}

	fmt.Println()

	// Display results
	if result.Valid {
		fmt.Printf("%s Module is valid\n", moduleSuccessIcon)

		// Show what was checked
		fmt.Println()
		fmt.Printf("%s Structure check passed\n", moduleSuccessIcon)
		fmt.Printf("%s Naming convention check passed\n", moduleSuccessIcon)
		fmt.Printf("%s Required files present\n", moduleSuccessIcon)

		if deep {
			fmt.Printf("%s Invkfile parses successfully\n", moduleSuccessIcon)
		} else {
			fmt.Printf("%s Use --deep to also validate invkfile syntax\n", moduleWarningIcon)
		}

		return nil
	}

	// Display issues
	fmt.Printf("%s Module validation failed with %d issue(s)\n", moduleErrorIcon, len(result.Issues))
	fmt.Println()

	for i, issue := range result.Issues {
		issueNum := fmt.Sprintf("%d.", i+1)
		issueType := moduleIssueTypeStyle.Render(fmt.Sprintf("[%s]", issue.Type))

		if issue.Path != "" {
			fmt.Printf("%s %s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, modulePathStyle.Render(issue.Path), issue.Message)
		} else {
			fmt.Printf("%s %s %s\n", moduleIssueStyle.Render(issueNum), issueType, issue.Message)
		}
	}

	// Exit with error code
	os.Exit(1)
	return nil
}
