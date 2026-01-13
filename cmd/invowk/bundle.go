package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/bundle"
	"invowk-cli/pkg/invowkfile"
)

var (
	// bundleValidateDeep enables deep validation including invowkfile parsing
	bundleValidateDeep bool

	// bundleCreatePath is the parent directory for bundle creation
	bundleCreatePath string
	// bundleCreateScripts creates a scripts directory in the bundle
	bundleCreateScripts bool
	// bundleCreateGroup is the group name for the bundle
	bundleCreateGroup string
	// bundleCreateDescription is the description for the bundle
	bundleCreateDescription string

	// bundlePackOutput is the output path for the packed bundle
	bundlePackOutput string

	// bundleImportPath is the destination directory for imported bundles
	bundleImportPath string
	// bundleImportOverwrite allows overwriting existing bundles
	bundleImportOverwrite bool
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

// bundleCreateCmd creates a new bundle
var bundleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new invowk bundle",
	Long: `Create a new invowk bundle with the given name.

The bundle name must follow naming conventions:
  - Start with a letter
  - Contain only alphanumeric characters
  - Use dots to separate segments (RDNS style recommended)

Examples:
  invowk bundle create mycommands
  invowk bundle create com.example.mytools
  invowk bundle create mytools --scripts
  invowk bundle create mytools --path /path/to/dir --group "My Tools"`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleCreate,
}

// bundleListCmd lists all discovered bundles
var bundleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered bundles",
	Long: `List all invowk bundles discovered in:
  - Current directory
  - User commands directory (~/.invowk/cmds)
  - Configured search paths

Examples:
  invowk bundle list`,
	RunE: runBundleList,
}

// bundlePackCmd packs a bundle into a ZIP archive
var bundlePackCmd = &cobra.Command{
	Use:   "pack <path>",
	Short: "Pack a bundle into a ZIP archive",
	Long: `Create a ZIP archive of an invowk bundle for distribution.

The archive will contain the bundle directory with all its contents.

Examples:
  invowk bundle pack ./mytools.invowkbundle
  invowk bundle pack ./mytools.invowkbundle --output ./dist/mytools.zip`,
	Args: cobra.ExactArgs(1),
	RunE: runBundlePack,
}

// bundleImportCmd imports a bundle from a ZIP file or URL
var bundleImportCmd = &cobra.Command{
	Use:   "import <source>",
	Short: "Import a bundle from a ZIP file or URL",
	Long: `Import an invowk bundle from a local ZIP file or a URL.

By default, bundles are imported to ~/.invowk/cmds.

Examples:
  invowk bundle import ./mytools.invowkbundle.zip
  invowk bundle import https://example.com/bundles/mytools.zip
  invowk bundle import ./bundle.zip --path ./local-bundles
  invowk bundle import ./bundle.zip --overwrite`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleImport,
}

func init() {
	bundleCmd.AddCommand(bundleValidateCmd)
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleListCmd)
	bundleCmd.AddCommand(bundlePackCmd)
	bundleCmd.AddCommand(bundleImportCmd)

	bundleValidateCmd.Flags().BoolVar(&bundleValidateDeep, "deep", false, "perform deep validation including invowkfile parsing")

	bundleCreateCmd.Flags().StringVarP(&bundleCreatePath, "path", "p", "", "parent directory for the bundle (default: current directory)")
	bundleCreateCmd.Flags().BoolVar(&bundleCreateScripts, "scripts", false, "create a scripts/ subdirectory")
	bundleCreateCmd.Flags().StringVarP(&bundleCreateGroup, "group", "g", "", "group name for the invowkfile (default: bundle name)")
	bundleCreateCmd.Flags().StringVarP(&bundleCreateDescription, "description", "d", "", "description for the invowkfile")

	bundlePackCmd.Flags().StringVarP(&bundlePackOutput, "output", "o", "", "output path for the ZIP file (default: <bundle-name>.invowkbundle.zip)")

	bundleImportCmd.Flags().StringVarP(&bundleImportPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	bundleImportCmd.Flags().BoolVar(&bundleImportOverwrite, "overwrite", false, "overwrite existing bundle if present")
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

func runBundleCreate(cmd *cobra.Command, args []string) error {
	bundleName := args[0]

	// Validate bundle name first
	if err := bundle.ValidateName(bundleName); err != nil {
		return err
	}

	fmt.Println(bundleTitleStyle.Render("Create Bundle"))

	// Create the bundle
	opts := bundle.CreateOptions{
		Name:             bundleName,
		ParentDir:        bundleCreatePath,
		Group:            bundleCreateGroup,
		Description:      bundleCreateDescription,
		CreateScriptsDir: bundleCreateScripts,
	}

	bundlePath, err := bundle.Create(opts)
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	fmt.Printf("%s Bundle created successfully\n", bundleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path: %s\n", bundleInfoIcon, bundlePathStyle.Render(bundlePath))
	fmt.Printf("%s Name: %s\n", bundleInfoIcon, cmdStyle.Render(bundleName))

	if bundleCreateScripts {
		fmt.Printf("%s Scripts directory created\n", bundleInfoIcon)
	}

	fmt.Println()
	fmt.Printf("%s Next steps:\n", bundleInfoIcon)
	fmt.Printf("   1. Edit %s to add your commands\n", bundlePathStyle.Render(filepath.Join(bundlePath, "invowkfile.cue")))
	if bundleCreateScripts {
		fmt.Printf("   2. Add script files to %s\n", bundlePathStyle.Render(filepath.Join(bundlePath, "scripts")))
	}
	fmt.Printf("   3. Run %s to validate\n", cmdStyle.Render("invowk bundle validate "+bundlePath))

	return nil
}

func runBundleList(cmd *cobra.Command, args []string) error {
	fmt.Println(bundleTitleStyle.Render("Discovered Bundles"))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create discovery instance
	disc := discovery.New(cfg)

	// Discover all files
	files, err := disc.DiscoverAll()
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}

	// Filter for bundles only
	var bundles []*discovery.DiscoveredFile
	for _, f := range files {
		if f.Bundle != nil {
			bundles = append(bundles, f)
		}
	}

	if len(bundles) == 0 {
		fmt.Printf("%s No bundles found\n", bundleWarningIcon)
		fmt.Println()
		fmt.Printf("%s Bundles are discovered in:\n", bundleInfoIcon)
		fmt.Printf("   - Current directory\n")
		fmt.Printf("   - User commands directory (~/.invowk/cmds)\n")
		fmt.Printf("   - Configured search paths\n")
		return nil
	}

	fmt.Printf("%s Found %d bundle(s)\n", bundleInfoIcon, len(bundles))
	fmt.Println()

	// Group by source
	bySource := make(map[discovery.Source][]*discovery.DiscoveredFile)
	for _, b := range bundles {
		bySource[b.Source] = append(bySource[b.Source], b)
	}

	// Display bundles by source
	sources := []discovery.Source{
		discovery.SourceCurrentDir,
		discovery.SourceUserDir,
		discovery.SourceConfigPath,
		discovery.SourceBundle,
	}

	for _, source := range sources {
		sourceBundles := bySource[source]
		if len(sourceBundles) == 0 {
			continue
		}

		fmt.Printf("%s %s:\n", bundleInfoIcon, source.String())
		for _, b := range sourceBundles {
			fmt.Printf("   %s %s\n", bundleSuccessIcon, cmdStyle.Render(b.Bundle.Name))
			fmt.Printf("      %s\n", bundleDetailStyle.Render(b.Bundle.Path))
		}
		fmt.Println()
	}

	return nil
}

func runBundlePack(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]

	fmt.Println(bundleTitleStyle.Render("Pack Bundle"))

	// Pack the bundle
	zipPath, err := bundle.Pack(bundlePath, bundlePackOutput)
	if err != nil {
		return fmt.Errorf("failed to pack bundle: %w", err)
	}

	// Get file info for size
	info, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Printf("%s Bundle packed successfully\n", bundleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Output: %s\n", bundleInfoIcon, bundlePathStyle.Render(zipPath))
	fmt.Printf("%s Size: %s\n", bundleInfoIcon, formatFileSize(info.Size()))

	return nil
}

func runBundleImport(cmd *cobra.Command, args []string) error {
	source := args[0]

	fmt.Println(bundleTitleStyle.Render("Import Bundle"))

	// Default destination to user commands directory
	destDir := bundleImportPath
	if destDir == "" {
		var err error
		destDir, err = config.CommandsDir()
		if err != nil {
			return fmt.Errorf("failed to get commands directory: %w", err)
		}
	}

	// Import the bundle
	opts := bundle.UnpackOptions{
		Source:    source,
		DestDir:   destDir,
		Overwrite: bundleImportOverwrite,
	}

	bundlePath, err := bundle.Unpack(opts)
	if err != nil {
		return fmt.Errorf("failed to import bundle: %w", err)
	}

	// Load the bundle to get its name
	b, err := bundle.Load(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to load imported bundle: %w", err)
	}

	fmt.Printf("%s Bundle imported successfully\n", bundleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Name: %s\n", bundleInfoIcon, cmdStyle.Render(b.Name))
	fmt.Printf("%s Path: %s\n", bundleInfoIcon, bundlePathStyle.Render(bundlePath))
	fmt.Println()
	fmt.Printf("%s The bundle commands are now available via invowk\n", bundleInfoIcon)

	return nil
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
