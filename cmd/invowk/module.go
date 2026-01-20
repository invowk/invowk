// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"context"
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// moduleValidateDeep enables deep validation including invkfile parsing
	moduleValidateDeep bool

	// moduleCreatePath is the parent directory for module creation
	moduleCreatePath string
	// moduleCreateScripts creates a scripts directory in the module
	moduleCreateScripts bool
	// moduleCreateModule is the module identifier for the invkfile
	moduleCreateModule string
	// moduleCreateDescription is the description for the module
	moduleCreateDescription string

	// moduleArchiveOutput is the output path for the archived module
	moduleArchiveOutput string

	// moduleImportPath is the destination directory for imported modules
	moduleImportPath string
	// moduleImportOverwrite allows overwriting existing modules
	moduleImportOverwrite bool

	// moduleVendorUpdate forces re-fetching of vendored dependencies
	moduleVendorUpdate bool
	// moduleVendorPrune removes unused vendored modules
	moduleVendorPrune bool

	// moduleAddAlias is the alias for the added module dependency
	moduleAddAlias string
	// moduleAddPath is the subdirectory path within the repository
	moduleAddPath string

	// Style definitions for module validation output
	moduleSuccessIcon = successStyle.Render("✓")
	moduleErrorIcon   = errorStyle.Render("✗")
	moduleWarningIcon = warningStyle.Render("!")
	moduleInfoIcon    = subtitleStyle.Render("•")

	moduleTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				MarginBottom(1)

	moduleIssueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				PaddingLeft(2)

	moduleIssueTypeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true)

	modulePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	moduleDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				PaddingLeft(2)

	// moduleCmd represents the module command group
	moduleCmd = &cobra.Command{
		Use:     "module",
		Aliases: []string{"mod"},
		Short:   "Manage invowk modules",
		Long: `Manage invowk modules - self-contained folders containing invkfiles and scripts.

A module is a folder with the ` + cmdStyle.Render(".invkmod") + ` suffix that contains:
  - ` + cmdStyle.Render("invkmod.cue") + ` (required): Module metadata (name, version, dependencies)
  - ` + cmdStyle.Render("invkfile.cue") + ` (optional): Command definitions
  - Optional script files referenced by command implementations

Module names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + cmdStyle.Render("com.example.mycommands.invkmod") + `)
  - The folder prefix must match the 'module' field in invkmod.cue

Examples:
  invowk module validate ./mycommands.invkmod
  invowk module validate ./com.example.tools.invkmod --deep`,
	}

	// moduleValidateCmd validates an invowk module
	moduleValidateCmd = &cobra.Command{
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
		RunE: runModuleValidate,
	}

	// moduleCreateCmd creates a new module
	moduleCreateCmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new invowk module",
		Long: `Create a new invowk module with the given name.

The module name must follow naming conventions:
  - Start with a letter
  - Contain only alphanumeric characters
  - Use dots to separate segments (RDNS style recommended)

Examples:
  invowk module create mycommands
  invowk module create com.example.mytools
  invowk module create mytools --scripts
  invowk module create mytools --path /path/to/dir --module-id "com.example.tools"`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleCreate,
	}

	// moduleListCmd lists all discovered modules
	moduleListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all discovered modules",
		Long: `List all invowk modules discovered in:
  - Current directory
  - User commands directory (~/.invowk/cmds)
  - Configured search paths

Examples:
  invowk module list`,
		RunE: runModuleList,
	}

	// moduleArchiveCmd creates a ZIP archive from a module
	moduleArchiveCmd = &cobra.Command{
		Use:   "archive <path>",
		Short: "Create a ZIP archive from a module",
		Long: `Create a ZIP archive of an invowk module for distribution.

The archive will contain the module directory with all its contents.

Examples:
  invowk module archive ./mytools.invkmod
  invowk module archive ./mytools.invkmod --output ./dist/mytools.zip`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleArchive,
	}

	// moduleImportCmd imports a module from a ZIP file or URL
	moduleImportCmd = &cobra.Command{
		Use:   "import <source>",
		Short: "Import a module from a ZIP file or URL",
		Long: `Import an invowk module from a local ZIP file or a URL.

By default, modules are imported to ~/.invowk/cmds.

Examples:
  invowk module import ./mytools.invkmod.zip
  invowk module import https://example.com/modules/mytools.zip
  invowk module import ./module.zip --path ./local-modules
  invowk module import ./module.zip --overwrite`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleImport,
	}

	// moduleAliasCmd manages module aliases for collision disambiguation
	moduleAliasCmd = &cobra.Command{
		Use:   "alias",
		Short: "Manage module aliases",
		Long: `Manage module aliases for collision disambiguation.

When two modules have the same 'module' identifier, you can use aliases to
give them different names. Aliases are stored in your invowk configuration.

Examples:
  invowk module alias set /path/to/module my-alias
  invowk module alias list
  invowk module alias remove /path/to/module`,
	}

	// moduleAliasSetCmd sets an alias for a module
	moduleAliasSetCmd = &cobra.Command{
		Use:   "set <module-path> <alias>",
		Short: "Set an alias for a module",
		Long: `Set an alias for a module to resolve naming collisions.

The alias will be used as the module identifier instead of the module's
declared 'module' field when discovering commands.

Examples:
  invowk module alias set ./mymodule.invkmod my-tools
  invowk module alias set /absolute/path/mymodule.invkmod custom-name`,
		Args: cobra.ExactArgs(2),
		RunE: runModuleAliasSet,
	}

	// moduleAliasListCmd lists all configured aliases
	moduleAliasListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all module aliases",
		Long: `List all configured module aliases.

Shows a table of module paths and their assigned aliases.

Examples:
  invowk module alias list`,
		RunE: runModuleAliasList,
	}

	// moduleAliasRemoveCmd removes an alias for a module
	moduleAliasRemoveCmd = &cobra.Command{
		Use:   "remove <module-path>",
		Short: "Remove an alias for a module",
		Long: `Remove a previously configured alias for a module.

The module will revert to using its declared 'module' identifier.

Examples:
  invowk module alias remove ./mymodule.invkmod
  invowk module alias remove /absolute/path/mymodule.invkmod`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleAliasRemove,
	}

	// moduleVendorCmd vendors dependencies into invk_modules/
	moduleVendorCmd = &cobra.Command{
		Use:   "vendor [module-path]",
		Short: "Vendor module dependencies",
		Long: `Vendor module dependencies into the invk_modules/ directory.

This command reads the 'requires' field from the invkmod.cue and fetches
all dependencies into the invk_modules/ subdirectory, enabling offline
and self-contained distribution.

If no module-path is specified, vendors dependencies for the current directory's
module.

Examples:
  invowk module vendor
  invowk module vendor ./mymodule.invkmod
  invowk module vendor --update
  invowk module vendor --prune`,
		Args: cobra.MaximumNArgs(1),
		RunE: runModuleVendor,
	}

	// moduleAddCmd adds a new module dependency
	moduleAddCmd = &cobra.Command{
		Use:   "add <git-url> <version>",
		Short: "Add a module dependency",
		Long: `Add a new module dependency from a Git repository.

The git-url should be an HTTPS or SSH URL to a Git repository containing
an invowk module. The version should be a semantic version constraint.

Version constraint formats:
  ^1.2.0  - Compatible with 1.2.0 (>=1.2.0 <2.0.0)
  ~1.2.0  - Approximately 1.2.0 (>=1.2.0 <1.3.0)
  >=1.0.0 - Greater than or equal to 1.0.0
  1.2.3   - Exact version 1.2.3

Examples:
  invowk module add https://github.com/user/module.git ^1.0.0
  invowk module add git@github.com:user/module.git ~2.0.0 --alias mymodule
  invowk module add https://github.com/user/monorepo.git ^1.0.0 --path modules/utils`,
		Args: cobra.ExactArgs(2),
		RunE: runModuleAdd,
	}

	// moduleRemoveCmd removes a module dependency
	moduleRemoveCmd = &cobra.Command{
		Use:   "remove <git-url>",
		Short: "Remove a module dependency",
		Long: `Remove a module dependency from the lock file.

This removes the module from the lock file. The cached module files are not deleted.
Don't forget to also remove the requires entry from your invkmod.cue.

Examples:
  invowk module remove https://github.com/user/module.git`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleRemove,
	}

	// moduleSyncCmd syncs dependencies from the invkfile
	moduleSyncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Sync dependencies from invkmod.cue",
		Long: `Sync all dependencies declared in invkmod.cue.

This reads the 'requires' field from invkmod.cue, resolves all version
constraints, downloads the modules, and updates the lock file.

Examples:
  invowk module sync`,
		RunE: runModuleSync,
	}

	// moduleUpdateCmd updates module dependencies
	moduleUpdateCmd = &cobra.Command{
		Use:   "update [git-url]",
		Short: "Update module dependencies",
		Long: `Update module dependencies to their latest matching versions.

Without arguments, updates all modules. With a git-url argument, updates
only that specific module.

Examples:
  invowk module update
  invowk module update https://github.com/user/module.git`,
		RunE: runModuleUpdate,
	}

	// moduleDepsCmd lists module dependencies
	moduleDepsCmd = &cobra.Command{
		Use:   "deps",
		Short: "List module dependencies",
		Long: `List all module dependencies from the lock file.

Shows all resolved modules with their versions, namespaces, and cache paths.

Examples:
  invowk module deps`,
		RunE: runModuleDeps,
	}
)

func init() {
	moduleCmd.AddCommand(moduleValidateCmd)
	moduleCmd.AddCommand(moduleCreateCmd)
	moduleCmd.AddCommand(moduleListCmd)
	moduleCmd.AddCommand(moduleArchiveCmd)
	moduleCmd.AddCommand(moduleImportCmd)
	moduleCmd.AddCommand(moduleAliasCmd)
	moduleCmd.AddCommand(moduleVendorCmd)

	// Dependency management commands
	moduleCmd.AddCommand(moduleAddCmd)
	moduleCmd.AddCommand(moduleRemoveCmd)
	moduleCmd.AddCommand(moduleSyncCmd)
	moduleCmd.AddCommand(moduleUpdateCmd)
	moduleCmd.AddCommand(moduleDepsCmd)

	// Register alias subcommands
	moduleAliasCmd.AddCommand(moduleAliasSetCmd)
	moduleAliasCmd.AddCommand(moduleAliasListCmd)
	moduleAliasCmd.AddCommand(moduleAliasRemoveCmd)

	moduleValidateCmd.Flags().BoolVar(&moduleValidateDeep, "deep", false, "perform deep validation including invkfile parsing")

	moduleCreateCmd.Flags().StringVarP(&moduleCreatePath, "path", "p", "", "parent directory for the module (default: current directory)")
	moduleCreateCmd.Flags().BoolVar(&moduleCreateScripts, "scripts", false, "create a scripts/ subdirectory")
	moduleCreateCmd.Flags().StringVarP(&moduleCreateModule, "module-id", "g", "", "module identifier for invkmod.cue (default: module name)")
	moduleCreateCmd.Flags().StringVarP(&moduleCreateDescription, "description", "d", "", "description for invkmod.cue")

	moduleArchiveCmd.Flags().StringVarP(&moduleArchiveOutput, "output", "o", "", "output path for the ZIP file (default: <module-name>.invkmod.zip)")

	moduleImportCmd.Flags().StringVarP(&moduleImportPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	moduleImportCmd.Flags().BoolVar(&moduleImportOverwrite, "overwrite", false, "overwrite existing module if present")

	moduleVendorCmd.Flags().BoolVar(&moduleVendorUpdate, "update", false, "force re-fetch of all dependencies")
	moduleVendorCmd.Flags().BoolVar(&moduleVendorPrune, "prune", false, "remove unused vendored modules")

	moduleAddCmd.Flags().StringVar(&moduleAddAlias, "alias", "", "alias for the module namespace")
	moduleAddCmd.Flags().StringVar(&moduleAddPath, "path", "", "subdirectory path within the repository")
}

func runModuleValidate(cmd *cobra.Command, args []string) error {
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
		fmt.Printf("%s Name: %s\n", moduleInfoIcon, cmdStyle.Render(result.ModuleName))
	}

	// Deep validation: parse invkfile
	var invkfileError error
	if moduleValidateDeep && result.InvkfilePath != "" {
		_, invkfileError = invkfile.Parse(result.InvkfilePath)
		if invkfileError != nil {
			result.AddIssue("invkfile", invkfileError.Error(), "invkfile.cue")
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

		if moduleValidateDeep {
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

func runModuleCreate(cmd *cobra.Command, args []string) error {
	moduleName := args[0]

	// Validate module name first
	if err := invkmod.ValidateName(moduleName); err != nil {
		return err
	}

	fmt.Println(moduleTitleStyle.Render("Create Module"))

	// Create the module
	opts := invkmod.CreateOptions{
		Name:             moduleName,
		ParentDir:        moduleCreatePath,
		Module:           moduleCreateModule,
		Description:      moduleCreateDescription,
		CreateScriptsDir: moduleCreateScripts,
	}

	modulePath, err := invkmod.Create(opts)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	fmt.Printf("%s Module created successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(modulePath))
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, cmdStyle.Render(moduleName))

	if moduleCreateScripts {
		fmt.Printf("%s Scripts directory created\n", moduleInfoIcon)
	}

	fmt.Println()
	fmt.Printf("%s Next steps:\n", moduleInfoIcon)
	fmt.Printf("   1. Edit %s to add your commands\n", modulePathStyle.Render(filepath.Join(modulePath, "invkfile.cue")))
	if moduleCreateScripts {
		fmt.Printf("   2. Add script files to %s\n", modulePathStyle.Render(filepath.Join(modulePath, "scripts")))
	}
	fmt.Printf("   3. Run %s to validate\n", cmdStyle.Render("invowk module validate "+modulePath))

	return nil
}

func runModuleList(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Discovered Modules"))

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

	// Filter for modules only
	var modules []*discovery.DiscoveredFile
	for _, f := range files {
		if f.Module != nil {
			modules = append(modules, f)
		}
	}

	if len(modules) == 0 {
		fmt.Printf("%s No modules found\n", moduleWarningIcon)
		fmt.Println()
		fmt.Printf("%s Modules are discovered in:\n", moduleInfoIcon)
		fmt.Printf("   - Current directory\n")
		fmt.Printf("   - User commands directory (~/.invowk/cmds)\n")
		fmt.Printf("   - Configured search paths\n")
		return nil
	}

	fmt.Printf("%s Found %d module(s)\n", moduleInfoIcon, len(modules))
	fmt.Println()

	// Group by source
	bySource := make(map[discovery.Source][]*discovery.DiscoveredFile)
	for _, b := range modules {
		bySource[b.Source] = append(bySource[b.Source], b)
	}

	// Display modules by source
	sources := []discovery.Source{
		discovery.SourceCurrentDir,
		discovery.SourceUserDir,
		discovery.SourceConfigPath,
		discovery.SourceModule,
	}

	for _, source := range sources {
		sourceModules := bySource[source]
		if len(sourceModules) == 0 {
			continue
		}

		fmt.Printf("%s %s:\n", moduleInfoIcon, source.String())
		for _, p := range sourceModules {
			fmt.Printf("   %s %s\n", moduleSuccessIcon, cmdStyle.Render(p.Module.Name()))
			fmt.Printf("      %s\n", moduleDetailStyle.Render(p.Module.Path))
		}
		fmt.Println()
	}

	return nil
}

func runModuleArchive(cmd *cobra.Command, args []string) error {
	modulePath := args[0]

	fmt.Println(moduleTitleStyle.Render("Archive Module"))

	// Archive the module
	zipPath, err := invkmod.Archive(modulePath, moduleArchiveOutput)
	if err != nil {
		return fmt.Errorf("failed to archive module: %w", err)
	}

	// Get file info for size
	info, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Printf("%s Module archived successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Output: %s\n", moduleInfoIcon, modulePathStyle.Render(zipPath))
	fmt.Printf("%s Size: %s\n", moduleInfoIcon, formatFileSize(info.Size()))

	return nil
}

func runModuleImport(cmd *cobra.Command, args []string) error {
	source := args[0]

	fmt.Println(moduleTitleStyle.Render("Import Module"))

	// Default destination to user commands directory
	destDir := moduleImportPath
	if destDir == "" {
		var err error
		destDir, err = config.CommandsDir()
		if err != nil {
			return fmt.Errorf("failed to get commands directory: %w", err)
		}
	}

	// Import the module
	opts := invkmod.UnpackOptions{
		Source:    source,
		DestDir:   destDir,
		Overwrite: moduleImportOverwrite,
	}

	modulePath, err := invkmod.Unpack(opts)
	if err != nil {
		return fmt.Errorf("failed to import module: %w", err)
	}

	// Load the module to get its name
	b, err := invkmod.Load(modulePath)
	if err != nil {
		return fmt.Errorf("failed to load imported module: %w", err)
	}

	fmt.Printf("%s Module imported successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, cmdStyle.Render(b.Name()))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(modulePath))
	fmt.Println()
	fmt.Printf("%s The module commands are now available via invowk\n", moduleInfoIcon)

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

func runModuleAliasSet(cmd *cobra.Command, args []string) error {
	modulePath := args[0]
	alias := args[1]

	fmt.Println(moduleTitleStyle.Render("Set Module Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify the path exists and is a valid module or invkfile
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Load current config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize ModuleAliases map if nil
	if cfg.ModuleAliases == nil {
		cfg.ModuleAliases = make(map[string]string)
	}

	// Set the alias
	cfg.ModuleAliases[absPath] = alias

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias set successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s\n", moduleInfoIcon, cmdStyle.Render(alias))

	return nil
}

func runModuleAliasList(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Module Aliases"))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.ModuleAliases) == 0 {
		fmt.Printf("%s No aliases configured\n", moduleWarningIcon)
		fmt.Println()
		fmt.Printf("%s To set an alias: %s\n", moduleInfoIcon, cmdStyle.Render("invowk module alias set <path> <alias>"))
		return nil
	}

	fmt.Printf("%s Found %d alias(es)\n", moduleInfoIcon, len(cfg.ModuleAliases))
	fmt.Println()

	for path, alias := range cfg.ModuleAliases {
		fmt.Printf("%s %s\n", moduleSuccessIcon, cmdStyle.Render(alias))
		fmt.Printf("   %s\n", moduleDetailStyle.Render(path))
	}

	return nil
}

func runModuleAliasRemove(cmd *cobra.Command, args []string) error {
	modulePath := args[0]

	fmt.Println(moduleTitleStyle.Render("Remove Module Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.ModuleAliases == nil {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Check if alias exists
	alias, exists := cfg.ModuleAliases[absPath]
	if !exists {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Remove the alias
	delete(cfg.ModuleAliases, absPath)

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias removed successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s (removed)\n", moduleInfoIcon, cmdStyle.Render(alias))

	return nil
}

func runModuleVendor(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Vendor Module Dependencies"))

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
	if _, statErr := os.Stat(invkfilePath); os.IsNotExist(statErr) {
		// Maybe it's a module directory
		if !invkmod.IsModule(absPath) {
			return fmt.Errorf("no invkfile.cue found in %s", absPath)
		}
	}

	// Parse invkmod.cue to get requirements
	invkmodulePath := filepath.Join(absPath, "invkmod.cue")
	meta, err := invkfile.ParseInvkmod(invkmodulePath)
	if err != nil {
		return fmt.Errorf("failed to parse invkmod.cue: %w", err)
	}

	if len(meta.Requires) == 0 {
		fmt.Printf("%s No dependencies declared in invkmod.cue\n", moduleWarningIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invkmod.cue\n", moduleInfoIcon, len(meta.Requires))

	// Determine vendor directory
	vendorDir := invkmod.GetVendoredModulesDir(absPath)

	// Handle prune mode
	if moduleVendorPrune {
		return pruneVendoredModules(vendorDir, meta)
	}

	// Create vendor directory if it doesn't exist
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return fmt.Errorf("failed to create vendor directory: %w", err)
	}

	fmt.Printf("%s Vendor directory: %s\n", moduleInfoIcon, modulePathStyle.Render(vendorDir))
	fmt.Println()

	// For now, just show what would be vendored
	// Full implementation would use module resolver to fetch and copy
	fmt.Printf("%s Vendoring is not yet fully implemented\n", moduleWarningIcon)
	fmt.Println()
	fmt.Printf("%s The following dependencies would be vendored:\n", moduleInfoIcon)
	for _, req := range meta.Requires {
		fmt.Printf("   %s %s@%s\n", moduleInfoIcon, req.GitURL, req.Version)
	}

	return nil
}

// pruneVendoredModules removes vendored modules that are not in the requirements
func pruneVendoredModules(vendorDir string, meta *invkfile.Invkmod) error {
	fmt.Println()
	fmt.Printf("%s Pruning unused vendored modules...\n", moduleInfoIcon)

	// Check if vendor directory exists
	if _, err := os.Stat(vendorDir); os.IsNotExist(err) {
		fmt.Printf("%s No vendor directory found, nothing to prune\n", moduleWarningIcon)
		return nil
	}

	// List vendored modules
	vendoredModules, err := invkmod.ListVendoredModules(filepath.Dir(vendorDir))
	if err != nil {
		return fmt.Errorf("failed to list vendored modules: %w", err)
	}

	if len(vendoredModules) == 0 {
		fmt.Printf("%s No vendored modules found\n", moduleInfoIcon)
		return nil
	}

	// Build a set of required module names/URLs for comparison
	// This is a simplified check - full implementation would match by Git URL
	requiredSet := make(map[string]bool)
	for _, req := range meta.Requires {
		requiredSet[req.GitURL] = true
	}

	// For now, just list what would be pruned
	fmt.Printf("%s Found %d vendored module(s)\n", moduleInfoIcon, len(vendoredModules))
	fmt.Printf("%s Prune functionality not yet fully implemented\n", moduleWarningIcon)

	return nil
}

func runModuleAdd(cmd *cobra.Command, args []string) error {
	gitURL := args[0]
	version := args[1]

	fmt.Println(moduleTitleStyle.Render("Add Module Dependency"))

	// Create module resolver
	resolver, err := invkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	// Create requirement
	req := invkmod.ModuleRef{
		GitURL:  gitURL,
		Version: version,
		Alias:   moduleAddAlias,
		Path:    moduleAddPath,
	}

	fmt.Printf("%s Resolving %s@%s...\n", moduleInfoIcon, gitURL, version)

	// Add the module
	ctx := context.Background()
	resolved, err := resolver.Add(ctx, req)
	if err != nil {
		fmt.Printf("%s Failed to add module: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Printf("%s Module added successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Git URL:   %s\n", moduleInfoIcon, modulePathStyle.Render(resolved.ModuleRef.GitURL))
	fmt.Printf("%s Version:   %s → %s\n", moduleInfoIcon, version, cmdStyle.Render(resolved.ResolvedVersion))
	fmt.Printf("%s Namespace: %s\n", moduleInfoIcon, cmdStyle.Render(resolved.Namespace))
	fmt.Printf("%s Cache:     %s\n", moduleInfoIcon, moduleDetailStyle.Render(resolved.CachePath))

	// Show how to add to invkfile
	fmt.Println()
	fmt.Printf("%s To use this module, add to your invkmod.cue:\n", moduleInfoIcon)
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

func runModuleRemove(cmd *cobra.Command, args []string) error {
	gitURL := args[0]

	fmt.Println(moduleTitleStyle.Render("Remove Module Dependency"))

	// Create module resolver
	resolver, err := invkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	fmt.Printf("%s Removing %s...\n", moduleInfoIcon, gitURL)

	// Remove the module
	ctx := context.Background()
	if err := resolver.Remove(ctx, gitURL); err != nil {
		fmt.Printf("%s Failed to remove module: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Printf("%s Module removed from lock file\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Don't forget to remove the requires entry from your invkmod.cue\n", moduleInfoIcon)

	return nil
}

func runModuleSync(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Sync Module Dependencies"))

	// Parse invkmod.cue to get requirements
	invkmodulePath := filepath.Join(".", "invkmod.cue")
	meta, err := invkfile.ParseInvkmod(invkmodulePath)
	if err != nil {
		return fmt.Errorf("failed to parse invkmod.cue: %w", err)
	}

	// Extract requirements from invkmod
	requirements := extractModuleRequirementsFromMetadata(meta)
	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invkmod.cue\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invkmod.cue\n", moduleInfoIcon, len(requirements))

	// Create module resolver
	resolver, err := invkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	// Sync modules
	ctx := context.Background()
	resolved, err := resolver.Sync(ctx, requirements)
	if err != nil {
		fmt.Printf("%s Failed to sync modules: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Println()
	for _, p := range resolved {
		fmt.Printf("%s %s → %s\n", moduleSuccessIcon,
			cmdStyle.Render(p.Namespace),
			moduleDetailStyle.Render(p.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, invkmod.LockFileName)

	return nil
}

func runModuleUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Update Module Dependencies"))

	// Create module resolver
	resolver, err := invkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	var gitURL string
	if len(args) > 0 {
		gitURL = args[0]
		fmt.Printf("%s Updating %s...\n", moduleInfoIcon, gitURL)
	} else {
		fmt.Printf("%s Updating all modules...\n", moduleInfoIcon)
	}

	// Update modules
	ctx := context.Background()
	updated, err := resolver.Update(ctx, gitURL)
	if err != nil {
		fmt.Printf("%s Failed to update modules: %v\n", moduleErrorIcon, err)
		return err
	}

	if len(updated) == 0 {
		fmt.Printf("%s No modules to update\n", moduleInfoIcon)
		return nil
	}

	fmt.Println()
	for _, p := range updated {
		fmt.Printf("%s %s → %s\n", moduleSuccessIcon,
			cmdStyle.Render(p.Namespace),
			moduleDetailStyle.Render(p.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, invkmod.LockFileName)

	return nil
}

func runModuleDeps(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Module Dependencies"))

	// Create module resolver
	resolver, err := invkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	// List modules
	ctx := context.Background()
	deps, err := resolver.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list module dependencies: %w", err)
	}

	if len(deps) == 0 {
		fmt.Printf("%s No module dependencies found\n", moduleInfoIcon)
		fmt.Println()
		fmt.Printf("%s To add modules, use: %s\n", moduleInfoIcon, cmdStyle.Render("invowk module add <git-url> <version>"))
		return nil
	}

	fmt.Printf("%s Found %d module dependency(ies)\n", moduleInfoIcon, len(deps))
	fmt.Println()

	for _, dep := range deps {
		fmt.Printf("%s %s\n", moduleSuccessIcon, cmdStyle.Render(dep.Namespace))
		fmt.Printf("   Git URL:  %s\n", dep.ModuleRef.GitURL)
		fmt.Printf("   Version:  %s → %s\n", dep.ModuleRef.Version, moduleDetailStyle.Render(dep.ResolvedVersion))
		if len(dep.GitCommit) >= 12 {
			fmt.Printf("   Commit:   %s\n", moduleDetailStyle.Render(dep.GitCommit[:12]))
		}
		fmt.Printf("   Cache:    %s\n", moduleDetailStyle.Render(dep.CachePath))
		fmt.Println()
	}

	return nil
}

// extractModuleRequirementsFromMetadata extracts module requirements from Invkmod.
func extractModuleRequirementsFromMetadata(meta *invkfile.Invkmod) []invkmod.ModuleRef {
	var reqs []invkmod.ModuleRef

	if meta == nil || meta.Requires == nil {
		return reqs
	}

	for _, r := range meta.Requires {
		reqs = append(reqs, invkmod.ModuleRef(r))
	}

	return reqs
}
