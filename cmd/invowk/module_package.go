// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"

	"github.com/spf13/cobra"
)

var (
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
)

func initModulePackageCmd() {
	moduleArchiveCmd.Flags().StringVarP(&moduleArchiveOutput, "output", "o", "", "output path for the ZIP file (default: <module-name>.invkmod.zip)")
	moduleImportCmd.Flags().StringVarP(&moduleImportPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	moduleImportCmd.Flags().BoolVar(&moduleImportOverwrite, "overwrite", false, "overwrite existing module if present")
	moduleVendorCmd.Flags().BoolVar(&moduleVendorUpdate, "update", false, "force re-fetch of all dependencies")
	moduleVendorCmd.Flags().BoolVar(&moduleVendorPrune, "prune", false, "remove unused vendored modules")
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
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(b.Name()))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(modulePath))
	fmt.Println()
	fmt.Printf("%s The module commands are now available via invowk\n", moduleInfoIcon)

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
