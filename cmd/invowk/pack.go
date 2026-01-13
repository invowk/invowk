package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invowkfile"
	"invowk-cli/pkg/pack"
)

var (
	// packValidateDeep enables deep validation including invowkfile parsing
	packValidateDeep bool

	// packCreatePath is the parent directory for pack creation
	packCreatePath string
	// packCreateScripts creates a scripts directory in the pack
	packCreateScripts bool
	// packCreateGroup is the group name for the pack
	packCreateGroup string
	// packCreateDescription is the description for the pack
	packCreateDescription string

	// packPackOutput is the output path for the packed pack
	packPackOutput string

	// packImportPath is the destination directory for imported packs
	packImportPath string
	// packImportOverwrite allows overwriting existing packs
	packImportOverwrite bool
)

// packCmd represents the pack command group
var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Manage invowk packs",
	Long: `Manage invowk packs - self-contained folders containing invowkfiles and scripts.

A pack is a folder with the ` + cmdStyle.Render(".invowkpack") + ` suffix that contains:
  - Exactly one ` + cmdStyle.Render("invowkfile.cue") + ` at the root
  - Optional script files referenced by command implementations

Pack names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + cmdStyle.Render("com.example.mycommands.invowkpack") + `)

Examples:
  invowk pack validate ./mycommands.invowkpack
  invowk pack validate ./com.example.tools.invowkpack --deep`,
}

// packValidateCmd validates an invowk pack
var packValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate an invowk pack",
	Long: `Validate the structure and contents of an invowk pack.

Checks performed:
  - Folder name follows pack naming conventions
  - Contains exactly one invowkfile.cue at the root
  - No nested packs inside
  - (with --deep) Invowkfile parses successfully

Examples:
  invowk pack validate ./mycommands.invowkpack
  invowk pack validate ./com.example.tools.invowkpack --deep`,
	Args: cobra.ExactArgs(1),
	RunE: runPackValidate,
}

// packCreateCmd creates a new pack
var packCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new invowk pack",
	Long: `Create a new invowk pack with the given name.

The pack name must follow naming conventions:
  - Start with a letter
  - Contain only alphanumeric characters
  - Use dots to separate segments (RDNS style recommended)

Examples:
  invowk pack create mycommands
  invowk pack create com.example.mytools
  invowk pack create mytools --scripts
  invowk pack create mytools --path /path/to/dir --group "My Tools"`,
	Args: cobra.ExactArgs(1),
	RunE: runPackCreate,
}

// packListCmd lists all discovered packs
var packListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered packs",
	Long: `List all invowk packs discovered in:
  - Current directory
  - User commands directory (~/.invowk/cmds)
  - Configured search paths

Examples:
  invowk pack list`,
	RunE: runPackList,
}

// packArchiveCmd creates a ZIP archive from a pack
var packArchiveCmd = &cobra.Command{
	Use:   "archive <path>",
	Short: "Create a ZIP archive from a pack",
	Long: `Create a ZIP archive of an invowk pack for distribution.

The archive will contain the pack directory with all its contents.

Examples:
  invowk pack archive ./mytools.invowkpack
  invowk pack archive ./mytools.invowkpack --output ./dist/mytools.zip`,
	Args: cobra.ExactArgs(1),
	RunE: runPackArchive,
}

// packImportCmd imports a pack from a ZIP file or URL
var packImportCmd = &cobra.Command{
	Use:   "import <source>",
	Short: "Import a pack from a ZIP file or URL",
	Long: `Import an invowk pack from a local ZIP file or a URL.

By default, packs are imported to ~/.invowk/cmds.

Examples:
  invowk pack import ./mytools.invowkpack.zip
  invowk pack import https://example.com/packs/mytools.zip
  invowk pack import ./pack.zip --path ./local-packs
  invowk pack import ./pack.zip --overwrite`,
	Args: cobra.ExactArgs(1),
	RunE: runPackImport,
}

func init() {
	packCmd.AddCommand(packValidateCmd)
	packCmd.AddCommand(packCreateCmd)
	packCmd.AddCommand(packListCmd)
	packCmd.AddCommand(packArchiveCmd)
	packCmd.AddCommand(packImportCmd)

	packValidateCmd.Flags().BoolVar(&packValidateDeep, "deep", false, "perform deep validation including invowkfile parsing")

	packCreateCmd.Flags().StringVarP(&packCreatePath, "path", "p", "", "parent directory for the pack (default: current directory)")
	packCreateCmd.Flags().BoolVar(&packCreateScripts, "scripts", false, "create a scripts/ subdirectory")
	packCreateCmd.Flags().StringVarP(&packCreateGroup, "group", "g", "", "group name for the invowkfile (default: pack name)")
	packCreateCmd.Flags().StringVarP(&packCreateDescription, "description", "d", "", "description for the invowkfile")

	packArchiveCmd.Flags().StringVarP(&packPackOutput, "output", "o", "", "output path for the ZIP file (default: <pack-name>.invowkpack.zip)")

	packImportCmd.Flags().StringVarP(&packImportPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	packImportCmd.Flags().BoolVar(&packImportOverwrite, "overwrite", false, "overwrite existing pack if present")
}

// Style definitions for pack validation output
var (
	packSuccessIcon = successStyle.Render("✓")
	packErrorIcon   = errorStyle.Render("✗")
	packWarningIcon = warningStyle.Render("!")
	packInfoIcon    = subtitleStyle.Render("•")

	packTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	packIssueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			PaddingLeft(2)

	packIssueTypeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true)

	packPathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	packDetailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			PaddingLeft(2)
)

func runPackValidate(cmd *cobra.Command, args []string) error {
	packPath := args[0]

	// Convert to absolute path for display
	absPath, err := filepath.Abs(packPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Println(packTitleStyle.Render("Pack Validation"))
	fmt.Printf("%s Path: %s\n", packInfoIcon, packPathStyle.Render(absPath))

	// Perform validation
	result, err := pack.Validate(packPath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Display pack name if parsed successfully
	if result.PackName != "" {
		fmt.Printf("%s Name: %s\n", packInfoIcon, cmdStyle.Render(result.PackName))
	}

	// Deep validation: parse invowkfile
	var invowkfileError error
	if packValidateDeep && result.InvowkfilePath != "" {
		_, invowkfileError = invowkfile.Parse(result.InvowkfilePath)
		if invowkfileError != nil {
			result.AddIssue("invowkfile", invowkfileError.Error(), "invowkfile.cue")
		}
	}

	fmt.Println()

	// Display results
	if result.Valid {
		fmt.Printf("%s Pack is valid\n", packSuccessIcon)

		// Show what was checked
		fmt.Println()
		fmt.Printf("%s Structure check passed\n", packSuccessIcon)
		fmt.Printf("%s Naming convention check passed\n", packSuccessIcon)
		fmt.Printf("%s Required files present\n", packSuccessIcon)

		if packValidateDeep {
			fmt.Printf("%s Invowkfile parses successfully\n", packSuccessIcon)
		} else {
			fmt.Printf("%s Use --deep to also validate invowkfile syntax\n", packWarningIcon)
		}

		return nil
	}

	// Display issues
	fmt.Printf("%s Pack validation failed with %d issue(s)\n", packErrorIcon, len(result.Issues))
	fmt.Println()

	for i, issue := range result.Issues {
		issueNum := fmt.Sprintf("%d.", i+1)
		issueType := packIssueTypeStyle.Render(fmt.Sprintf("[%s]", issue.Type))

		if issue.Path != "" {
			fmt.Printf("%s %s %s %s\n", packIssueStyle.Render(issueNum), issueType, packPathStyle.Render(issue.Path), issue.Message)
		} else {
			fmt.Printf("%s %s %s\n", packIssueStyle.Render(issueNum), issueType, issue.Message)
		}
	}

	// Exit with error code
	os.Exit(1)
	return nil
}

func runPackCreate(cmd *cobra.Command, args []string) error {
	packName := args[0]

	// Validate pack name first
	if err := pack.ValidateName(packName); err != nil {
		return err
	}

	fmt.Println(packTitleStyle.Render("Create Pack"))

	// Create the pack
	opts := pack.CreateOptions{
		Name:             packName,
		ParentDir:        packCreatePath,
		Group:            packCreateGroup,
		Description:      packCreateDescription,
		CreateScriptsDir: packCreateScripts,
	}

	packPath, err := pack.Create(opts)
	if err != nil {
		return fmt.Errorf("failed to create pack: %w", err)
	}

	fmt.Printf("%s Pack created successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path: %s\n", packInfoIcon, packPathStyle.Render(packPath))
	fmt.Printf("%s Name: %s\n", packInfoIcon, cmdStyle.Render(packName))

	if packCreateScripts {
		fmt.Printf("%s Scripts directory created\n", packInfoIcon)
	}

	fmt.Println()
	fmt.Printf("%s Next steps:\n", packInfoIcon)
	fmt.Printf("   1. Edit %s to add your commands\n", packPathStyle.Render(filepath.Join(packPath, "invowkfile.cue")))
	if packCreateScripts {
		fmt.Printf("   2. Add script files to %s\n", packPathStyle.Render(filepath.Join(packPath, "scripts")))
	}
	fmt.Printf("   3. Run %s to validate\n", cmdStyle.Render("invowk pack validate "+packPath))

	return nil
}

func runPackList(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Discovered Packs"))

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

	// Filter for packs only
	var packs []*discovery.DiscoveredFile
	for _, f := range files {
		if f.Pack != nil {
			packs = append(packs, f)
		}
	}

	if len(packs) == 0 {
		fmt.Printf("%s No packs found\n", packWarningIcon)
		fmt.Println()
		fmt.Printf("%s Packs are discovered in:\n", packInfoIcon)
		fmt.Printf("   - Current directory\n")
		fmt.Printf("   - User commands directory (~/.invowk/cmds)\n")
		fmt.Printf("   - Configured search paths\n")
		return nil
	}

	fmt.Printf("%s Found %d pack(s)\n", packInfoIcon, len(packs))
	fmt.Println()

	// Group by source
	bySource := make(map[discovery.Source][]*discovery.DiscoveredFile)
	for _, b := range packs {
		bySource[b.Source] = append(bySource[b.Source], b)
	}

	// Display packs by source
	sources := []discovery.Source{
		discovery.SourceCurrentDir,
		discovery.SourceUserDir,
		discovery.SourceConfigPath,
		discovery.SourcePack,
	}

	for _, source := range sources {
		sourcePacks := bySource[source]
		if len(sourcePacks) == 0 {
			continue
		}

		fmt.Printf("%s %s:\n", packInfoIcon, source.String())
		for _, p := range sourcePacks {
			fmt.Printf("   %s %s\n", packSuccessIcon, cmdStyle.Render(p.Pack.Name))
			fmt.Printf("      %s\n", packDetailStyle.Render(p.Pack.Path))
		}
		fmt.Println()
	}

	return nil
}

func runPackArchive(cmd *cobra.Command, args []string) error {
	packPath := args[0]

	fmt.Println(packTitleStyle.Render("Archive Pack"))

	// Archive the pack
	zipPath, err := pack.Archive(packPath, packPackOutput)
	if err != nil {
		return fmt.Errorf("failed to archive pack: %w", err)
	}

	// Get file info for size
	info, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Printf("%s Pack packed successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Output: %s\n", packInfoIcon, packPathStyle.Render(zipPath))
	fmt.Printf("%s Size: %s\n", packInfoIcon, formatFileSize(info.Size()))

	return nil
}

func runPackImport(cmd *cobra.Command, args []string) error {
	source := args[0]

	fmt.Println(packTitleStyle.Render("Import Pack"))

	// Default destination to user commands directory
	destDir := packImportPath
	if destDir == "" {
		var err error
		destDir, err = config.CommandsDir()
		if err != nil {
			return fmt.Errorf("failed to get commands directory: %w", err)
		}
	}

	// Import the pack
	opts := pack.UnpackOptions{
		Source:    source,
		DestDir:   destDir,
		Overwrite: packImportOverwrite,
	}

	packPath, err := pack.Unpack(opts)
	if err != nil {
		return fmt.Errorf("failed to import pack: %w", err)
	}

	// Load the pack to get its name
	b, err := pack.Load(packPath)
	if err != nil {
		return fmt.Errorf("failed to load imported pack: %w", err)
	}

	fmt.Printf("%s Pack imported successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Name: %s\n", packInfoIcon, cmdStyle.Render(b.Name))
	fmt.Printf("%s Path: %s\n", packInfoIcon, packPathStyle.Render(packPath))
	fmt.Println()
	fmt.Printf("%s The pack commands are now available via invowk\n", packInfoIcon)

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
