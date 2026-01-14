// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/pack"
	"invowk-cli/pkg/packs"
)

var (
	// packValidateDeep enables deep validation including invkfile parsing
	packValidateDeep bool

	// packCreatePath is the parent directory for pack creation
	packCreatePath string
	// packCreateScripts creates a scripts directory in the pack
	packCreateScripts bool
	// packCreatePack is the pack identifier for the invkfile
	packCreatePack string
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
	Long: `Manage invowk packs - self-contained folders containing invkfiles and scripts.

A pack is a folder with the ` + cmdStyle.Render(".invkpack") + ` suffix that contains:
  - ` + cmdStyle.Render("invkpack.cue") + ` (required): Pack metadata (name, version, dependencies)
  - ` + cmdStyle.Render("invkfile.cue") + ` (optional): Command definitions
  - Optional script files referenced by command implementations

Pack names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + cmdStyle.Render("com.example.mycommands.invkpack") + `)
  - The folder prefix must match the 'pack' field in invkpack.cue

Examples:
  invowk pack validate ./mycommands.invkpack
  invowk pack validate ./com.example.tools.invkpack --deep`,
}

// packValidateCmd validates an invowk pack
var packValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate an invowk pack",
	Long: `Validate the structure and contents of an invowk pack.

Checks performed:
  - Folder name follows pack naming conventions
  - Contains exactly one invkfile.cue at the root
  - No nested packs inside
  - (with --deep) Invkfile parses successfully

Examples:
  invowk pack validate ./mycommands.invkpack
  invowk pack validate ./com.example.tools.invkpack --deep`,
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
  invowk pack create mytools --path /path/to/dir --pack-id "com.example.tools"`,
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
  invowk pack archive ./mytools.invkpack
  invowk pack archive ./mytools.invkpack --output ./dist/mytools.zip`,
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
  invowk pack import ./mytools.invkpack.zip
  invowk pack import https://example.com/packs/mytools.zip
  invowk pack import ./pack.zip --path ./local-packs
  invowk pack import ./pack.zip --overwrite`,
	Args: cobra.ExactArgs(1),
	RunE: runPackImport,
}

// packAliasCmd manages pack aliases for collision disambiguation
var packAliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage pack aliases",
	Long: `Manage pack aliases for collision disambiguation.

When two packs have the same 'pack' identifier, you can use aliases to
give them different names. Aliases are stored in your invowk configuration.

Examples:
  invowk pack alias set /path/to/pack my-alias
  invowk pack alias list
  invowk pack alias remove /path/to/pack`,
}

// packAliasSetCmd sets an alias for a pack
var packAliasSetCmd = &cobra.Command{
	Use:   "set <pack-path> <alias>",
	Short: "Set an alias for a pack",
	Long: `Set an alias for a pack to resolve naming collisions.

The alias will be used as the pack identifier instead of the pack's
declared 'pack' field when discovering commands.

Examples:
  invowk pack alias set ./mypack.invkpack my-tools
  invowk pack alias set /absolute/path/pack.invkpack custom-name`,
	Args: cobra.ExactArgs(2),
	RunE: runPackAliasSet,
}

// packAliasListCmd lists all configured aliases
var packAliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pack aliases",
	Long: `List all configured pack aliases.

Shows a table of pack paths and their assigned aliases.

Examples:
  invowk pack alias list`,
	RunE: runPackAliasList,
}

// packAliasRemoveCmd removes an alias for a pack
var packAliasRemoveCmd = &cobra.Command{
	Use:   "remove <pack-path>",
	Short: "Remove an alias for a pack",
	Long: `Remove a previously configured alias for a pack.

The pack will revert to using its declared 'pack' identifier.

Examples:
  invowk pack alias remove ./mypack.invkpack
  invowk pack alias remove /absolute/path/pack.invkpack`,
	Args: cobra.ExactArgs(1),
	RunE: runPackAliasRemove,
}

var (
	// packVendorUpdate forces re-fetching of vendored dependencies
	packVendorUpdate bool
	// packVendorPrune removes unused vendored packs
	packVendorPrune bool

	// packAddAlias is the alias for the added pack dependency
	packAddAlias string
	// packAddPath is the subdirectory path within the repository
	packAddPath string
)

// packVendorCmd vendors dependencies into invk_packs/
var packVendorCmd = &cobra.Command{
	Use:   "vendor [pack-path]",
	Short: "Vendor pack dependencies",
	Long: `Vendor pack dependencies into the invk_packs/ directory.

This command reads the 'requires' field from the invkfile and fetches
all dependencies into the invk_packs/ subdirectory, enabling offline
and self-contained distribution.

If no pack-path is specified, vendors dependencies for the current directory's
invkfile or pack.

Examples:
  invowk pack vendor
  invowk pack vendor ./mypack.invkpack
  invowk pack vendor --update
  invowk pack vendor --prune`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPackVendor,
}

// packAddCmd adds a new pack dependency
var packAddCmd = &cobra.Command{
	Use:   "add <git-url> <version>",
	Short: "Add a pack dependency",
	Long: `Add a new pack dependency from a Git repository.

The git-url should be an HTTPS or SSH URL to a Git repository containing
an invowk pack. The version should be a semantic version constraint.

Version constraint formats:
  ^1.2.0  - Compatible with 1.2.0 (>=1.2.0 <2.0.0)
  ~1.2.0  - Approximately 1.2.0 (>=1.2.0 <1.3.0)
  >=1.0.0 - Greater than or equal to 1.0.0
  1.2.3   - Exact version 1.2.3

Examples:
  invowk pack add https://github.com/user/pack.git ^1.0.0
  invowk pack add git@github.com:user/pack.git ~2.0.0 --alias mypack
  invowk pack add https://github.com/user/monorepo.git ^1.0.0 --path packs/utils`,
	Args: cobra.ExactArgs(2),
	RunE: runPackAdd,
}

// packRemoveCmd removes a pack dependency
var packRemoveCmd = &cobra.Command{
	Use:   "remove <git-url>",
	Short: "Remove a pack dependency",
	Long: `Remove a pack dependency from the lock file.

This removes the pack from the lock file. The cached pack files are not deleted.
Don't forget to also remove the requires entry from your invkfile.cue.

Examples:
  invowk pack remove https://github.com/user/pack.git`,
	Args: cobra.ExactArgs(1),
	RunE: runPackRemove,
}

// packSyncCmd syncs dependencies from the invkfile
var packSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync dependencies from invkfile",
	Long: `Sync all dependencies declared in the invkfile.

This reads the 'requires' field from the invkfile, resolves all version
constraints, downloads the packs, and updates the lock file.

Examples:
  invowk pack sync`,
	RunE: runPackSync,
}

// packUpdateCmd updates pack dependencies
var packUpdateCmd = &cobra.Command{
	Use:   "update [git-url]",
	Short: "Update pack dependencies",
	Long: `Update pack dependencies to their latest matching versions.

Without arguments, updates all packs. With a git-url argument, updates
only that specific pack.

Examples:
  invowk pack update
  invowk pack update https://github.com/user/pack.git`,
	RunE: runPackUpdate,
}

// packDepsCmd lists pack dependencies
var packDepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "List pack dependencies",
	Long: `List all pack dependencies from the lock file.

Shows all resolved packs with their versions, namespaces, and cache paths.

Examples:
  invowk pack deps`,
	RunE: runPackDeps,
}

func init() {
	packCmd.AddCommand(packValidateCmd)
	packCmd.AddCommand(packCreateCmd)
	packCmd.AddCommand(packListCmd)
	packCmd.AddCommand(packArchiveCmd)
	packCmd.AddCommand(packImportCmd)
	packCmd.AddCommand(packAliasCmd)
	packCmd.AddCommand(packVendorCmd)

	// Dependency management commands
	packCmd.AddCommand(packAddCmd)
	packCmd.AddCommand(packRemoveCmd)
	packCmd.AddCommand(packSyncCmd)
	packCmd.AddCommand(packUpdateCmd)
	packCmd.AddCommand(packDepsCmd)

	// Register alias subcommands
	packAliasCmd.AddCommand(packAliasSetCmd)
	packAliasCmd.AddCommand(packAliasListCmd)
	packAliasCmd.AddCommand(packAliasRemoveCmd)

	packValidateCmd.Flags().BoolVar(&packValidateDeep, "deep", false, "perform deep validation including invkfile parsing")

	packCreateCmd.Flags().StringVarP(&packCreatePath, "path", "p", "", "parent directory for the pack (default: current directory)")
	packCreateCmd.Flags().BoolVar(&packCreateScripts, "scripts", false, "create a scripts/ subdirectory")
	packCreateCmd.Flags().StringVarP(&packCreatePack, "pack-id", "g", "", "pack identifier for the invkfile (default: pack name)")
	packCreateCmd.Flags().StringVarP(&packCreateDescription, "description", "d", "", "description for the invkfile")

	packArchiveCmd.Flags().StringVarP(&packPackOutput, "output", "o", "", "output path for the ZIP file (default: <pack-name>.invkpack.zip)")

	packImportCmd.Flags().StringVarP(&packImportPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	packImportCmd.Flags().BoolVar(&packImportOverwrite, "overwrite", false, "overwrite existing pack if present")

	packVendorCmd.Flags().BoolVar(&packVendorUpdate, "update", false, "force re-fetch of all dependencies")
	packVendorCmd.Flags().BoolVar(&packVendorPrune, "prune", false, "remove unused vendored packs")

	packAddCmd.Flags().StringVar(&packAddAlias, "alias", "", "alias for the pack namespace")
	packAddCmd.Flags().StringVar(&packAddPath, "path", "", "subdirectory path within the repository")
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

	// Deep validation: parse invkfile
	var invkfileError error
	if packValidateDeep && result.InvkfilePath != "" {
		_, invkfileError = invkfile.Parse(result.InvkfilePath)
		if invkfileError != nil {
			result.AddIssue("invkfile", invkfileError.Error(), "invkfile.cue")
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
			fmt.Printf("%s Invkfile parses successfully\n", packSuccessIcon)
		} else {
			fmt.Printf("%s Use --deep to also validate invkfile syntax\n", packWarningIcon)
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
		Pack:             packCreatePack,
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
	fmt.Printf("   1. Edit %s to add your commands\n", packPathStyle.Render(filepath.Join(packPath, "invkfile.cue")))
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

func runPackAliasSet(cmd *cobra.Command, args []string) error {
	packPath := args[0]
	alias := args[1]

	fmt.Println(packTitleStyle.Render("Set Pack Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(packPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify the path exists and is a valid pack or invkfile
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Load current config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize PackAliases map if nil
	if cfg.PackAliases == nil {
		cfg.PackAliases = make(map[string]string)
	}

	// Set the alias
	cfg.PackAliases[absPath] = alias

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias set successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", packInfoIcon, packPathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s\n", packInfoIcon, cmdStyle.Render(alias))

	return nil
}

func runPackAliasList(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Pack Aliases"))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.PackAliases == nil || len(cfg.PackAliases) == 0 {
		fmt.Printf("%s No aliases configured\n", packWarningIcon)
		fmt.Println()
		fmt.Printf("%s To set an alias: %s\n", packInfoIcon, cmdStyle.Render("invowk pack alias set <path> <alias>"))
		return nil
	}

	fmt.Printf("%s Found %d alias(es)\n", packInfoIcon, len(cfg.PackAliases))
	fmt.Println()

	for path, alias := range cfg.PackAliases {
		fmt.Printf("%s %s\n", packSuccessIcon, cmdStyle.Render(alias))
		fmt.Printf("   %s\n", packDetailStyle.Render(path))
	}

	return nil
}

func runPackAliasRemove(cmd *cobra.Command, args []string) error {
	packPath := args[0]

	fmt.Println(packTitleStyle.Render("Remove Pack Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(packPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.PackAliases == nil {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Check if alias exists
	alias, exists := cfg.PackAliases[absPath]
	if !exists {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Remove the alias
	delete(cfg.PackAliases, absPath)

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias removed successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", packInfoIcon, packPathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s (removed)\n", packInfoIcon, cmdStyle.Render(alias))

	return nil
}

func runPackVendor(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Vendor Pack Dependencies"))

	// Determine the target directory
	var targetDir string
	if len(args) > 0 {
		targetDir = args[0]
	} else {
		targetDir = "."
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Find invkfile
	invkfilePath := filepath.Join(absPath, "invkfile.cue")
	if _, err := os.Stat(invkfilePath); os.IsNotExist(err) {
		// Maybe it's a pack directory
		if pack.IsPack(absPath) {
			invkfilePath = filepath.Join(absPath, "invkfile.cue")
		} else {
			return fmt.Errorf("no invkfile.cue found in %s", absPath)
		}
	}

	// Parse invkpack.cue to get requirements
	invkpackPath := filepath.Join(absPath, "invkpack.cue")
	meta, err := invkfile.ParseInvkpack(invkpackPath)
	if err != nil {
		return fmt.Errorf("failed to parse invkpack.cue: %w", err)
	}

	if len(meta.Requires) == 0 {
		fmt.Printf("%s No dependencies declared in invkpack.cue\n", packWarningIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invkpack.cue\n", packInfoIcon, len(meta.Requires))

	// Determine vendor directory
	vendorDir := pack.GetVendoredPacksDir(absPath)

	// Handle prune mode
	if packVendorPrune {
		return pruneVendoredPacks(vendorDir, meta)
	}

	// Create vendor directory if it doesn't exist
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		return fmt.Errorf("failed to create vendor directory: %w", err)
	}

	fmt.Printf("%s Vendor directory: %s\n", packInfoIcon, packPathStyle.Render(vendorDir))
	fmt.Println()

	// For now, just show what would be vendored
	// Full implementation would use packs.Resolver to fetch and copy
	fmt.Printf("%s Vendoring is not yet fully implemented\n", packWarningIcon)
	fmt.Println()
	fmt.Printf("%s The following dependencies would be vendored:\n", packInfoIcon)
	for _, req := range meta.Requires {
		fmt.Printf("   %s %s@%s\n", packInfoIcon, req.GitURL, req.Version)
	}

	return nil
}

// pruneVendoredPacks removes vendored packs that are not in the requirements
func pruneVendoredPacks(vendorDir string, meta *invkfile.Invkpack) error {
	fmt.Println()
	fmt.Printf("%s Pruning unused vendored packs...\n", packInfoIcon)

	// Check if vendor directory exists
	if _, err := os.Stat(vendorDir); os.IsNotExist(err) {
		fmt.Printf("%s No vendor directory found, nothing to prune\n", packWarningIcon)
		return nil
	}

	// List vendored packs
	vendoredPacks, err := pack.ListVendoredPacks(filepath.Dir(vendorDir))
	if err != nil {
		return fmt.Errorf("failed to list vendored packs: %w", err)
	}

	if len(vendoredPacks) == 0 {
		fmt.Printf("%s No vendored packs found\n", packInfoIcon)
		return nil
	}

	// Build a set of required pack names/URLs for comparison
	// This is a simplified check - full implementation would match by Git URL
	requiredSet := make(map[string]bool)
	for _, req := range meta.Requires {
		requiredSet[req.GitURL] = true
	}

	// For now, just list what would be pruned
	fmt.Printf("%s Found %d vendored pack(s)\n", packInfoIcon, len(vendoredPacks))
	fmt.Printf("%s Prune functionality not yet fully implemented\n", packWarningIcon)

	return nil
}

func runPackAdd(cmd *cobra.Command, args []string) error {
	gitURL := args[0]
	version := args[1]

	fmt.Println(packTitleStyle.Render("Add Pack Dependency"))

	// Create pack resolver
	resolver, err := packs.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create pack resolver: %w", err)
	}

	// Create requirement
	req := packs.PackRef{
		GitURL:  gitURL,
		Version: version,
		Alias:   packAddAlias,
		Path:    packAddPath,
	}

	fmt.Printf("%s Resolving %s@%s...\n", packInfoIcon, gitURL, version)

	// Add the pack
	ctx := context.Background()
	resolved, err := resolver.Add(ctx, req)
	if err != nil {
		fmt.Printf("%s Failed to add pack: %v\n", packErrorIcon, err)
		return err
	}

	fmt.Printf("%s Pack added successfully\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Git URL:   %s\n", packInfoIcon, packPathStyle.Render(resolved.PackRef.GitURL))
	fmt.Printf("%s Version:   %s → %s\n", packInfoIcon, version, cmdStyle.Render(resolved.ResolvedVersion))
	fmt.Printf("%s Namespace: %s\n", packInfoIcon, cmdStyle.Render(resolved.Namespace))
	fmt.Printf("%s Cache:     %s\n", packInfoIcon, packDetailStyle.Render(resolved.CachePath))

	// Show how to add to invkfile
	fmt.Println()
	fmt.Printf("%s To use this pack, add to your invkfile.cue:\n", packInfoIcon)
	fmt.Println()
	fmt.Println("requires: [")
	fmt.Printf("\t{\n")
	fmt.Printf("\t\tgit_url: %q\n", req.GitURL)
	fmt.Printf("\t\tversion: %q\n", req.Version)
	if req.Alias != "" {
		fmt.Printf("\t\talias:   %q\n", req.Alias)
	}
	if req.Path != "" {
		fmt.Printf("\t\tpath:    %q\n", req.Path)
	}
	fmt.Printf("\t},\n")
	fmt.Println("]")

	return nil
}

func runPackRemove(cmd *cobra.Command, args []string) error {
	gitURL := args[0]

	fmt.Println(packTitleStyle.Render("Remove Pack Dependency"))

	// Create pack resolver
	resolver, err := packs.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create pack resolver: %w", err)
	}

	fmt.Printf("%s Removing %s...\n", packInfoIcon, gitURL)

	// Remove the pack
	ctx := context.Background()
	if err := resolver.Remove(ctx, gitURL); err != nil {
		fmt.Printf("%s Failed to remove pack: %v\n", packErrorIcon, err)
		return err
	}

	fmt.Printf("%s Pack removed from lock file\n", packSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Don't forget to remove the requires entry from your invkpack.cue\n", packInfoIcon)

	return nil
}

func runPackSync(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Sync Pack Dependencies"))

	// Parse invkpack.cue to get requirements
	invkpackPath := filepath.Join(".", "invkpack.cue")
	meta, err := invkfile.ParseInvkpack(invkpackPath)
	if err != nil {
		return fmt.Errorf("failed to parse invkpack.cue: %w", err)
	}

	// Extract requirements from invkpack
	requirements := extractPackRequirementsFromMetadata(meta)
	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invkpack.cue\n", packInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invkpack.cue\n", packInfoIcon, len(requirements))

	// Create pack resolver
	resolver, err := packs.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create pack resolver: %w", err)
	}

	// Sync packs
	ctx := context.Background()
	resolved, err := resolver.Sync(ctx, requirements)
	if err != nil {
		fmt.Printf("%s Failed to sync packs: %v\n", packErrorIcon, err)
		return err
	}

	fmt.Println()
	for _, p := range resolved {
		fmt.Printf("%s %s → %s\n", packSuccessIcon,
			cmdStyle.Render(p.Namespace),
			packDetailStyle.Render(p.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", packSuccessIcon, packs.LockFileName)

	return nil
}

func runPackUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Update Pack Dependencies"))

	// Create pack resolver
	resolver, err := packs.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create pack resolver: %w", err)
	}

	var gitURL string
	if len(args) > 0 {
		gitURL = args[0]
		fmt.Printf("%s Updating %s...\n", packInfoIcon, gitURL)
	} else {
		fmt.Printf("%s Updating all packs...\n", packInfoIcon)
	}

	// Update packs
	ctx := context.Background()
	updated, err := resolver.Update(ctx, gitURL)
	if err != nil {
		fmt.Printf("%s Failed to update packs: %v\n", packErrorIcon, err)
		return err
	}

	if len(updated) == 0 {
		fmt.Printf("%s No packs to update\n", packInfoIcon)
		return nil
	}

	fmt.Println()
	for _, p := range updated {
		fmt.Printf("%s %s → %s\n", packSuccessIcon,
			cmdStyle.Render(p.Namespace),
			packDetailStyle.Render(p.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", packSuccessIcon, packs.LockFileName)

	return nil
}

func runPackDeps(cmd *cobra.Command, args []string) error {
	fmt.Println(packTitleStyle.Render("Pack Dependencies"))

	// Create pack resolver
	resolver, err := packs.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create pack resolver: %w", err)
	}

	// List packs
	ctx := context.Background()
	deps, err := resolver.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pack dependencies: %w", err)
	}

	if len(deps) == 0 {
		fmt.Printf("%s No pack dependencies found\n", packInfoIcon)
		fmt.Println()
		fmt.Printf("%s To add packs, use: %s\n", packInfoIcon, cmdStyle.Render("invowk pack add <git-url> <version>"))
		return nil
	}

	fmt.Printf("%s Found %d pack dependency(ies)\n", packInfoIcon, len(deps))
	fmt.Println()

	for _, dep := range deps {
		fmt.Printf("%s %s\n", packSuccessIcon, cmdStyle.Render(dep.Namespace))
		fmt.Printf("   Git URL:  %s\n", dep.PackRef.GitURL)
		fmt.Printf("   Version:  %s → %s\n", dep.PackRef.Version, packDetailStyle.Render(dep.ResolvedVersion))
		if len(dep.GitCommit) >= 12 {
			fmt.Printf("   Commit:   %s\n", packDetailStyle.Render(dep.GitCommit[:12]))
		}
		fmt.Printf("   Cache:    %s\n", packDetailStyle.Render(dep.CachePath))
		fmt.Println()
	}

	return nil
}

// extractPackRequirementsFromMetadata extracts pack requirements from Invkpack.
func extractPackRequirementsFromMetadata(meta *invkfile.Invkpack) []packs.PackRef {
	var reqs []packs.PackRef

	if meta == nil || meta.Requires == nil {
		return reqs
	}

	for _, r := range meta.Requires {
		reqs = append(reqs, packs.PackRef{
			GitURL:  r.GitURL,
			Version: r.Version,
			Alias:   r.Alias,
			Path:    r.Path,
		})
	}

	return reqs
}
