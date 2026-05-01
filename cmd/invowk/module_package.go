// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/app/moduleops"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

// newModuleArchiveCommand creates the `invowk module archive` command.
func newModuleArchiveCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "archive <path>",
		Short: "Create a ZIP archive from a module",
		Long: `Create a ZIP archive of an invowk module for distribution.

The archive will contain the module directory with all its contents.

Examples:
  invowk module archive ./mytools.invowkmod
  invowk module archive ./mytools.invowkmod --output ./dist/mytools.zip`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runModuleArchive(args, output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "output path for the ZIP file (default: <module-name>.invowkmod.zip)")

	return cmd
}

// newModuleImportCommand creates the `invowk module import` command.
func newModuleImportCommand() *cobra.Command {
	var (
		importPath      string
		importOverwrite bool
	)

	cmd := &cobra.Command{
		Use:   "import <source>",
		Short: "Import a module from a ZIP file or URL",
		Long: `Import an invowk module from a local ZIP file or a URL.

By default, modules are imported to ~/.invowk/cmds.

Examples:
  invowk module import ./mytools.invowkmod.zip
  invowk module import https://example.com/modules/mytools.zip
  invowk module import ./module.zip --path ./local-modules
  invowk module import ./module.zip --overwrite`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleImport(cmd.Context(), args, importPath, importOverwrite)
		},
	}

	cmd.Flags().StringVarP(&importPath, "path", "p", "", "destination directory (default: ~/.invowk/cmds)")
	cmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "overwrite existing module if present")

	return cmd
}

// newModuleVendorCommand creates the `invowk module vendor` command.
func newModuleVendorCommand() *cobra.Command {
	var (
		vendorUpdate bool
		vendorPrune  bool
	)

	cmd := &cobra.Command{
		Use:   "vendor [module-path]",
		Short: "Vendor module dependencies",
		Long: `Vendor module dependencies into the invowk_modules/ directory.

This command reads the 'requires' field from the invowkmod.cue, resolves
all dependencies, and copies them into the invowk_modules/ subdirectory,
enabling offline and self-contained distribution.

If a lock file (invowkmod.lock.cue) exists, vendoring uses the locked
versions for reproducibility. Use --update to force re-resolution of all
dependencies (updates the lock file and re-copies everything).

If no module-path is specified, vendors dependencies for the current directory's
module.

Examples:
  invowk module vendor
  invowk module vendor ./mymodule.invowkmod
  invowk module vendor --update
  invowk module vendor --prune`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleVendor(cmd.Context(), args, vendorUpdate, vendorPrune)
		},
	}

	cmd.Flags().BoolVar(&vendorUpdate, "update", false, "force re-fetch of all dependencies")
	cmd.Flags().BoolVar(&vendorPrune, "prune", false, "remove unused vendored modules")

	return cmd
}

func runModuleArchive(args []string, output string) error {
	modulePath := args[0]

	fmt.Println(moduleTitleStyle.Render("Archive Module"))

	// Archive the module
	zipPath, err := moduleops.Archive(types.FilesystemPath(modulePath), types.FilesystemPath(output))
	if err != nil {
		return fmt.Errorf("failed to archive module: %w", err)
	}

	// Get file info for size
	info, err := os.Stat(string(zipPath))
	if err != nil {
		return fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Printf("%s Module archived successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Output: %s\n", moduleInfoIcon, modulePathStyle.Render(string(zipPath)))
	fmt.Printf("%s Size: %s\n", moduleInfoIcon, formatFileSize(info.Size()))

	return nil
}

func runModuleImport(ctx context.Context, args []string, importPath string, importOverwrite bool) error {
	source := args[0]

	fmt.Println(moduleTitleStyle.Render("Import Module"))

	// Default destination to user commands directory
	destDir := types.FilesystemPath(importPath)
	if destDir == "" {
		var err error
		destDir, err = config.CommandsDir()
		if err != nil {
			return fmt.Errorf("failed to get commands directory: %w", err)
		}
	}

	// Import the module
	opts := moduleops.UnpackOptions{
		Source:    source,
		DestDir:   destDir,
		Overwrite: importOverwrite,
	}

	result, err := moduleops.Unpack(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to import module: %w", err)
	}

	fmt.Printf("%s Module imported successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(string(result.ModuleName())))
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(string(result.ModulePath())))
	fmt.Println()
	fmt.Printf("%s The module commands are now available via invowk\n", moduleInfoIcon)

	return nil
}

func runModuleVendor(ctx context.Context, args []string, vendorUpdate, vendorPrune bool) error {
	fmt.Println(moduleTitleStyle.Render("Vendor Module Dependencies"))

	// Determine the target directory
	var targetDir string
	if len(args) > 0 {
		targetDir = args[0]
	} else {
		targetDir = "."
	}

	targetPath := types.FilesystemPath(targetDir)
	if err := targetPath.Validate(); err != nil {
		return fmt.Errorf("module path: %w", err)
	}
	requirements, result, strategy, err := moduleops.VendorDependencies(ctx, targetPath, vendorUpdate, vendorPrune)
	if err != nil && len(requirements) == 0 {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	if len(requirements) == 0 {
		fmt.Printf("%s No dependencies declared in invowkmod.cue\n", moduleWarningIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invowkmod.cue\n", moduleInfoIcon, len(requirements))
	switch strategy {
	case moduleops.VendorResolutionUpdated:
		fmt.Printf("%s Re-resolving all dependencies (--update)\n", moduleInfoIcon)
	case moduleops.VendorResolutionLocked:
		fmt.Printf("%s Loading from lock file\n", moduleInfoIcon)
	case moduleops.VendorResolutionSynced:
		fmt.Printf("%s Resolving dependencies (no lock file)\n", moduleInfoIcon)
	}
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Print results
	fmt.Println()
	fmt.Printf("%s Vendor directory: %s\n", moduleInfoIcon, modulePathStyle.Render(string(result.VendorDir)))
	fmt.Println()

	for _, entry := range result.Vendored {
		fmt.Printf("   %s %s\n", moduleSuccessIcon, entry.Namespace)
	}

	if len(result.Pruned) > 0 {
		fmt.Println()
		fmt.Printf("%s Pruned %d stale module(s):\n", moduleInfoIcon, len(result.Pruned))
		for _, name := range result.Pruned {
			fmt.Printf("   %s %s\n", moduleWarningIcon, name)
		}
	}

	fmt.Println()
	fmt.Printf("%s Vendored %d module(s) successfully\n", moduleSuccessIcon, len(result.Vendored))

	return nil
}

// formatFileSize formats a file size in bytes to a human-readable string
//
//plint:render
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
