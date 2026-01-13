// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/modules"
)

var (
	// moduleAddAlias is the alias for the added module
	moduleAddAlias string

	// moduleAddPath is the subdirectory path within the repository
	moduleAddPath string

	// moduleUpdateAll updates all modules
	moduleUpdateAll bool
)

// moduleCmd represents the module command group
var moduleCmd = &cobra.Command{
	Use:   "module",
	Short: "Manage pack dependencies",
	Long: `Manage pack module dependencies from Git repositories.

Modules allow packs to declare dependencies on other packs hosted in Git
repositories (GitHub, GitLab, etc.). Dependencies are declared in the
invkfile using the 'requires' field with semantic version constraints.

Examples:
  invowk module add https://github.com/user/pack.git ^1.0.0
  invowk module list
  invowk module sync
  invowk module update`,
}

// moduleAddCmd adds a new module dependency
var moduleAddCmd = &cobra.Command{
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
  invowk module add https://github.com/user/pack.git ^1.0.0
  invowk module add git@github.com:user/pack.git ~2.0.0 --alias mypack
  invowk module add https://github.com/user/monorepo.git ^1.0.0 --path packs/utils`,
	Args: cobra.ExactArgs(2),
	RunE: runModuleAdd,
}

// moduleRemoveCmd removes a module dependency
var moduleRemoveCmd = &cobra.Command{
	Use:   "remove <git-url>",
	Short: "Remove a pack dependency",
	Long: `Remove a pack dependency from the invkfile.

This removes the module from both the invkfile requires field and the
lock file. The cached module files are not deleted.

Examples:
  invowk module remove https://github.com/user/pack.git`,
	Args: cobra.ExactArgs(1),
	RunE: runModuleRemove,
}

// moduleListCmd lists all module dependencies
var moduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pack dependencies",
	Long: `List all pack dependencies from the lock file.

Shows all resolved modules with their versions, namespaces, and cache paths.

Examples:
  invowk module list`,
	RunE: runModuleList,
}

// moduleSyncCmd syncs dependencies from the invkfile
var moduleSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync dependencies from invkfile",
	Long: `Sync all dependencies declared in the invkfile.

This reads the 'requires' field from the invkfile, resolves all version
constraints, downloads the modules, and updates the lock file.

Examples:
  invowk module sync`,
	RunE: runModuleSync,
}

// moduleUpdateCmd updates module dependencies
var moduleUpdateCmd = &cobra.Command{
	Use:   "update [git-url]",
	Short: "Update pack dependencies",
	Long: `Update pack dependencies to their latest matching versions.

Without arguments, updates all modules. With a git-url argument, updates
only that specific module.

Examples:
  invowk module update
  invowk module update https://github.com/user/pack.git`,
	RunE: runModuleUpdate,
}

func init() {
	moduleCmd.AddCommand(moduleAddCmd)
	moduleCmd.AddCommand(moduleRemoveCmd)
	moduleCmd.AddCommand(moduleListCmd)
	moduleCmd.AddCommand(moduleSyncCmd)
	moduleCmd.AddCommand(moduleUpdateCmd)

	moduleAddCmd.Flags().StringVar(&moduleAddAlias, "alias", "", "alias for the module namespace")
	moduleAddCmd.Flags().StringVar(&moduleAddPath, "path", "", "subdirectory path within the repository")

	moduleUpdateCmd.Flags().BoolVar(&moduleUpdateAll, "all", false, "update all modules")
}

// Style definitions for module output
var (
	moduleSuccessIcon = successStyle.Render("✓")
	moduleErrorIcon   = errorStyle.Render("✗")
	moduleInfoIcon    = subtitleStyle.Render("•")

	moduleTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				MarginBottom(1)

	moduleNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Bold(true)

	moduleVersionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981"))

	modulePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

func runModuleAdd(cmd *cobra.Command, args []string) error {
	gitURL := args[0]
	version := args[1]

	fmt.Println(moduleTitleStyle.Render("Add Module"))

	// Create module manager
	mgr, err := modules.NewManager("", "")
	if err != nil {
		return fmt.Errorf("failed to create module manager: %w", err)
	}

	// Create requirement
	req := modules.Requirement{
		GitURL:  gitURL,
		Version: version,
		Alias:   moduleAddAlias,
		Path:    moduleAddPath,
	}

	fmt.Printf("%s Resolving %s@%s...\n", moduleInfoIcon, gitURL, version)

	// Add the module
	ctx := context.Background()
	resolved, err := mgr.Add(ctx, req)
	if err != nil {
		fmt.Printf("%s Failed to add module: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Printf("%s Module added successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Git URL:   %s\n", moduleInfoIcon, moduleNameStyle.Render(resolved.Requirement.GitURL))
	fmt.Printf("%s Version:   %s → %s\n", moduleInfoIcon, version, moduleVersionStyle.Render(resolved.ResolvedVersion))
	fmt.Printf("%s Namespace: %s\n", moduleInfoIcon, moduleNameStyle.Render(resolved.Namespace))
	fmt.Printf("%s Cache:     %s\n", moduleInfoIcon, modulePathStyle.Render(resolved.CachePath))

	// Update invkfile with the new requirement
	fmt.Println()
	fmt.Printf("%s To use this module, add to your invkfile.cue:\n", moduleInfoIcon)
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

	fmt.Println(moduleTitleStyle.Render("Remove Module"))

	// Create module manager
	mgr, err := modules.NewManager("", "")
	if err != nil {
		return fmt.Errorf("failed to create module manager: %w", err)
	}

	fmt.Printf("%s Removing %s...\n", moduleInfoIcon, gitURL)

	// Remove the module
	ctx := context.Background()
	if err := mgr.Remove(ctx, gitURL); err != nil {
		fmt.Printf("%s Failed to remove module: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Printf("%s Module removed from lock file\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Don't forget to remove the requires entry from your invkfile.cue\n", moduleInfoIcon)

	return nil
}

func runModuleList(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Module Dependencies"))

	// Create module manager
	mgr, err := modules.NewManager("", "")
	if err != nil {
		return fmt.Errorf("failed to create module manager: %w", err)
	}

	// List modules
	ctx := context.Background()
	mods, err := mgr.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	if len(mods) == 0 {
		fmt.Printf("%s No modules found\n", moduleInfoIcon)
		fmt.Println()
		fmt.Printf("%s To add modules, use: invowk module add <git-url> <version>\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d module(s)\n", moduleInfoIcon, len(mods))
	fmt.Println()

	for _, mod := range mods {
		fmt.Printf("%s %s\n", moduleSuccessIcon, moduleNameStyle.Render(mod.Namespace))
		fmt.Printf("   Git URL:  %s\n", mod.Requirement.GitURL)
		fmt.Printf("   Version:  %s → %s\n", mod.Requirement.Version, moduleVersionStyle.Render(mod.ResolvedVersion))
		fmt.Printf("   Commit:   %s\n", modulePathStyle.Render(mod.GitCommit[:12]))
		fmt.Printf("   Cache:    %s\n", modulePathStyle.Render(mod.CachePath))
		fmt.Println()
	}

	return nil
}

func runModuleSync(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Sync Modules"))

	// Find invkfile
	invkfilePath := filepath.Join(".", "invkfile.cue")
	if _, err := os.Stat(invkfilePath); os.IsNotExist(err) {
		invkfilePath = filepath.Join(".", "invkfile")
		if _, err := os.Stat(invkfilePath); os.IsNotExist(err) {
			return fmt.Errorf("no invkfile found in current directory")
		}
	}

	// Parse invkfile
	inv, err := invkfile.Parse(invkfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse invkfile: %w", err)
	}

	// Extract requirements from invkfile
	requirements := extractRequirements(inv)
	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invkfile\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invkfile\n", moduleInfoIcon, len(requirements))

	// Create module manager
	mgr, err := modules.NewManager("", "")
	if err != nil {
		return fmt.Errorf("failed to create module manager: %w", err)
	}

	// Sync modules
	ctx := context.Background()
	resolved, err := mgr.Sync(ctx, requirements)
	if err != nil {
		fmt.Printf("%s Failed to sync modules: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Println()
	for _, mod := range resolved {
		fmt.Printf("%s %s → %s\n", moduleSuccessIcon,
			moduleNameStyle.Render(mod.Namespace),
			moduleVersionStyle.Render(mod.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, modules.LockFileName)

	return nil
}

func runModuleUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Update Modules"))

	// Create module manager
	mgr, err := modules.NewManager("", "")
	if err != nil {
		return fmt.Errorf("failed to create module manager: %w", err)
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
	updated, err := mgr.Update(ctx, gitURL)
	if err != nil {
		fmt.Printf("%s Failed to update modules: %v\n", moduleErrorIcon, err)
		return err
	}

	if len(updated) == 0 {
		fmt.Printf("%s No modules to update\n", moduleInfoIcon)
		return nil
	}

	fmt.Println()
	for _, mod := range updated {
		fmt.Printf("%s %s → %s\n", moduleSuccessIcon,
			moduleNameStyle.Render(mod.Namespace),
			moduleVersionStyle.Render(mod.ResolvedVersion))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, modules.LockFileName)

	return nil
}

// extractRequirements extracts module requirements from an invkfile.
// This reads the Requires field from the parsed invkfile.
func extractRequirements(inv *invkfile.Invkfile) []modules.Requirement {
	var reqs []modules.Requirement

	// Check if invkfile has Requires field
	if inv.Requires == nil {
		return reqs
	}

	for _, r := range inv.Requires {
		reqs = append(reqs, modules.Requirement{
			GitURL:  r.GitURL,
			Version: r.Version,
			Alias:   r.Alias,
			Path:    r.Path,
		})
	}

	return reqs
}

// formatGitURL formats a git URL for display.
func formatGitURL(url string) string {
	// Remove common prefixes for cleaner display
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "git@")
	url = strings.TrimSuffix(url, ".git")
	url = strings.ReplaceAll(url, ":", "/")
	return url
}
