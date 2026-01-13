package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/pkg/bundle"
	"invowk-cli/pkg/invowkfile"
)

var (
	// bundleValidateDeep enables deep validation including invowkfile parsing
	bundleValidateDeep bool
)

// bundleCmd represents the bundle command group
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Manage invowk bundles",
	Long: `Manage invowk bundles - self-contained folders containing invowkfiles and scripts.

A bundle is a folder with the ` + cmdStyle.Render(".invowkbundle") + ` suffix that contains:
  - Exactly one ` + cmdStyle.Render("invowkfile.cue") + ` at the root
  - Optional script files referenced by command implementations

Bundle names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + cmdStyle.Render("com.example.mycommands.invowkbundle") + `)

Examples:
  invowk bundle validate ./mycommands.invowkbundle
  invowk bundle validate ./com.example.tools.invowkbundle --deep`,
}

// bundleValidateCmd validates an invowk bundle
var bundleValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate an invowk bundle",
	Long: `Validate the structure and contents of an invowk bundle.

Checks performed:
  - Folder name follows bundle naming conventions
  - Contains exactly one invowkfile.cue at the root
  - No nested bundles inside
  - (with --deep) Invowkfile parses successfully

Examples:
  invowk bundle validate ./mycommands.invowkbundle
  invowk bundle validate ./com.example.tools.invowkbundle --deep`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleValidate,
}

func init() {
	bundleCmd.AddCommand(bundleValidateCmd)

	bundleValidateCmd.Flags().BoolVar(&bundleValidateDeep, "deep", false, "perform deep validation including invowkfile parsing")
}

// Style definitions for bundle validation output
var (
	bundleSuccessIcon = successStyle.Render("✓")
	bundleErrorIcon   = errorStyle.Render("✗")
	bundleWarningIcon = warningStyle.Render("!")
	bundleInfoIcon    = subtitleStyle.Render("•")

	bundleTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				MarginBottom(1)

	bundleIssueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				PaddingLeft(2)

	bundleIssueTypeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true)

	bundlePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	bundleDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				PaddingLeft(2)
)

func runBundleValidate(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]

	// Convert to absolute path for display
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Println(bundleTitleStyle.Render("Bundle Validation"))
	fmt.Printf("%s Path: %s\n", bundleInfoIcon, bundlePathStyle.Render(absPath))

	// Perform validation
	result, err := bundle.Validate(bundlePath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Display bundle name if parsed successfully
	if result.BundleName != "" {
		fmt.Printf("%s Name: %s\n", bundleInfoIcon, cmdStyle.Render(result.BundleName))
	}

	// Deep validation: parse invowkfile
	var invowkfileError error
	if bundleValidateDeep && result.InvowkfilePath != "" {
		_, invowkfileError = invowkfile.Parse(result.InvowkfilePath)
		if invowkfileError != nil {
			result.AddIssue("invowkfile", invowkfileError.Error(), "invowkfile.cue")
		}
	}

	fmt.Println()

	// Display results
	if result.Valid {
		fmt.Printf("%s Bundle is valid\n", bundleSuccessIcon)

		// Show what was checked
		fmt.Println()
		fmt.Printf("%s Structure check passed\n", bundleSuccessIcon)
		fmt.Printf("%s Naming convention check passed\n", bundleSuccessIcon)
		fmt.Printf("%s Required files present\n", bundleSuccessIcon)

		if bundleValidateDeep {
			fmt.Printf("%s Invowkfile parses successfully\n", bundleSuccessIcon)
		} else {
			fmt.Printf("%s Use --deep to also validate invowkfile syntax\n", bundleWarningIcon)
		}

		return nil
	}

	// Display issues
	fmt.Printf("%s Bundle validation failed with %d issue(s)\n", bundleErrorIcon, len(result.Issues))
	fmt.Println()

	for i, issue := range result.Issues {
		issueNum := fmt.Sprintf("%d.", i+1)
		issueType := bundleIssueTypeStyle.Render(fmt.Sprintf("[%s]", issue.Type))

		if issue.Path != "" {
			fmt.Printf("%s %s %s %s\n", bundleIssueStyle.Render(issueNum), issueType, bundlePathStyle.Render(issue.Path), issue.Message)
		} else {
			fmt.Printf("%s %s %s\n", bundleIssueStyle.Render(issueNum), issueType, issue.Message)
		}
	}

	// Exit with error code
	os.Exit(1)
	return nil
}
