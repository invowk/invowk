// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	// moduleAddAlias is the alias for the added module dependency
	moduleAddAlias string
	// moduleAddPath is the subdirectory path within the repository
	moduleAddPath string

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

func initModuleDepsCmd() {
	moduleAddCmd.Flags().StringVar(&moduleAddAlias, "alias", "", "alias for the module namespace")
	moduleAddCmd.Flags().StringVar(&moduleAddPath, "path", "", "subdirectory path within the repository")
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
