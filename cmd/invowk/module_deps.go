// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

// newModuleAddCommand creates the `invowk module add` command.
func newModuleAddCommand() *cobra.Command {
	var (
		addAlias string
		addPath  string
	)

	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleAdd(args, addAlias, addPath)
		},
	}

	cmd.Flags().StringVar(&addAlias, "alias", "", "alias for the module namespace")
	cmd.Flags().StringVar(&addPath, "path", "", "subdirectory path within the repository")

	return cmd
}

// newModuleRemoveCommand creates the `invowk module remove` command.
func newModuleRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <identifier>",
		Short: "Remove a module dependency",
		Long: `Remove a module dependency from both the lock file and invowkmod.cue.

The identifier can be:
  - A git URL:   https://github.com/user/module.git
  - A namespace: myalias (if alias was set)
  - A name:      modulename (bare name without @version)
  - A full key:  modulename@1.2.3

The cached module files are not deleted.

Examples:
  invowk module remove https://github.com/user/module.git
  invowk module remove myalias
  invowk module remove modulename`,
		Args: cobra.ExactArgs(1),
		RunE: runModuleRemove,
	}
}

// newModuleSyncCommand creates the `invowk module sync` command.
func newModuleSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync dependencies from invowkmod.cue",
		Long: `Sync all dependencies declared in invowkmod.cue.

This reads the 'requires' field from invowkmod.cue, resolves all version
constraints, downloads the modules, and updates the lock file.

Examples:
  invowk module sync`,
		Args: cobra.ExactArgs(0),
		RunE: runModuleSync,
	}
}

// newModuleUpdateCommand creates the `invowk module update` command.
func newModuleUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update [identifier]",
		Short: "Update module dependencies",
		Long: `Update module dependencies to their latest matching versions.

Without arguments, updates all modules. With an identifier, updates
only the matching module(s).

The identifier can be:
  - A git URL:   https://github.com/user/module.git
  - A namespace: myalias (if alias was set)
  - A name:      modulename (bare name without @version)
  - A full key:  modulename@1.2.3

Examples:
  invowk module update
  invowk module update https://github.com/user/module.git
  invowk module update myalias
  invowk module update modulename`,
		Args: cobra.MaximumNArgs(1),
		RunE: runModuleUpdate,
	}
}

// newModuleDepsCommand creates the `invowk module deps` command.
func newModuleDepsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "deps",
		Short: "List module dependencies",
		Long: `List all module dependencies from the lock file.

Shows all resolved modules with their versions, namespaces, and cache paths.

Examples:
  invowk module deps`,
		Args: cobra.ExactArgs(0),
		RunE: runModuleDeps,
	}
}

func runModuleAdd(args []string, addAlias, addPath string) error {
	gitURL := args[0]
	version := args[1]

	fmt.Println(moduleTitleStyle.Render("Add Module Dependency"))

	// Create module resolver
	resolver, err := invowkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	// Create requirement
	req := invowkmod.ModuleRef{
		GitURL:  invowkmod.GitURL(gitURL),
		Version: invowkmod.SemVerConstraint(version),
		Alias:   invowkmod.ModuleAlias(addAlias),
		Path:    invowkmod.SubdirectoryPath(addPath),
	}

	fmt.Printf("%s Resolving %s@%s...\n", moduleInfoIcon, gitURL, version)

	// Add the module (resolves, caches, and updates lock file)
	ctx := context.Background()
	resolved, err := resolver.Add(ctx, req)
	if err != nil {
		fmt.Printf("%s Failed to add module: %v\n", moduleErrorIcon, err)
		return err
	}

	fmt.Printf("%s Module resolved and lock file updated\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Git URL:   %s\n", moduleInfoIcon, modulePathStyle.Render(string(resolved.ModuleRef.GitURL)))
	fmt.Printf("%s Version:   %s → %s\n", moduleInfoIcon, version, CmdStyle.Render(string(resolved.ResolvedVersion)))
	fmt.Printf("%s Namespace: %s\n", moduleInfoIcon, CmdStyle.Render(string(resolved.Namespace)))
	fmt.Printf("%s Cache:     %s\n", moduleInfoIcon, moduleDetailStyle.Render(string(resolved.CachePath)))

	// Auto-edit invowkmod.cue to add the requires entry
	invowkmodPath := filepath.Join(".", "invowkmod.cue")
	if editErr := invowkmod.AddRequirement(types.FilesystemPath(invowkmodPath), req); editErr != nil {
		if os.IsNotExist(editErr) {
			fmt.Println()
			fmt.Printf("%s invowkmod.cue not found — lock file was updated but you need to create invowkmod.cue\n", moduleInfoIcon)
		} else {
			fmt.Println()
			fmt.Printf("%s Could not auto-edit invowkmod.cue: %v\n", moduleInfoIcon, editErr)
		}
	} else {
		fmt.Printf("%s Updated invowkmod.cue with new requires entry\n", moduleSuccessIcon)
	}

	return nil
}

func runModuleRemove(cmd *cobra.Command, args []string) error {
	identifier := args[0]

	fmt.Println(moduleTitleStyle.Render("Remove Module Dependency"))

	// Create module resolver
	resolver, err := invowkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	fmt.Printf("%s Removing %s...\n", moduleInfoIcon, identifier)

	// Remove from lock file
	ctx := context.Background()
	results, err := resolver.Remove(ctx, identifier)
	if err != nil {
		// Format ambiguous matches nicely
		if ambigErr, ok := errors.AsType[*invowkmod.AmbiguousIdentifierError](err); ok && ambigErr != nil {
			fmt.Printf("%s %v\n", moduleErrorIcon, ambigErr)
			return err
		}
		fmt.Printf("%s Failed to remove module: %v\n", moduleErrorIcon, err)
		return err
	}

	// Auto-edit invowkmod.cue to remove the requires entries
	invowkmodPath := filepath.Join(".", "invowkmod.cue")
	for i := range results {
		if editErr := invowkmod.RemoveRequirement(types.FilesystemPath(invowkmodPath), results[i].RemovedEntry.GitURL, results[i].RemovedEntry.Path); editErr != nil {
			fmt.Printf("%s Could not auto-edit invowkmod.cue: %v\n", moduleInfoIcon, editErr)
		}
	}

	for i := range results {
		fmt.Printf("%s Removed %s\n", moduleSuccessIcon, CmdStyle.Render(string(results[i].RemovedEntry.Namespace)))
	}

	fmt.Println()
	fmt.Printf("%s Lock file and invowkmod.cue updated\n", moduleSuccessIcon)

	return nil
}

func runModuleSync(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Sync Module Dependencies"))

	// Parse invowkmod.cue to get requirements
	invowkmodulePath := filepath.Join(".", "invowkmod.cue")
	meta, err := invowkfile.ParseInvowkmod(invowkfile.FilesystemPath(invowkmodulePath))
	if err != nil {
		return fmt.Errorf("failed to parse invowkmod.cue: %w", err)
	}

	// Extract requirements from invowkmod
	requirements := extractModuleRequirementsFromMetadata(meta)
	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invowkmod.cue\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invowkmod.cue\n", moduleInfoIcon, len(requirements))

	// Create module resolver
	resolver, err := invowkmod.NewResolver("", "")
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
			CmdStyle.Render(string(p.Namespace)),
			moduleDetailStyle.Render(string(p.ResolvedVersion)))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, invowkmod.LockFileName)

	return nil
}

func runModuleUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Update Module Dependencies"))

	// Create module resolver
	resolver, err := invowkmod.NewResolver("", "")
	if err != nil {
		return fmt.Errorf("failed to create module resolver: %w", err)
	}

	var identifier string
	if len(args) > 0 {
		identifier = args[0]
		fmt.Printf("%s Updating %s...\n", moduleInfoIcon, identifier)
	} else {
		fmt.Printf("%s Updating all modules...\n", moduleInfoIcon)
	}

	// Update modules
	ctx := context.Background()
	updated, err := resolver.Update(ctx, identifier)
	if err != nil {
		// Format ambiguous matches nicely
		if ambigErr, ok := errors.AsType[*invowkmod.AmbiguousIdentifierError](err); ok && ambigErr != nil {
			fmt.Printf("%s %v\n", moduleErrorIcon, ambigErr)
			return err
		}
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
			CmdStyle.Render(string(p.Namespace)),
			moduleDetailStyle.Render(string(p.ResolvedVersion)))
	}

	fmt.Println()
	fmt.Printf("%s Lock file updated: %s\n", moduleSuccessIcon, invowkmod.LockFileName)

	return nil
}

func runModuleDeps(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Module Dependencies"))

	// Create module resolver
	resolver, err := invowkmod.NewResolver("", "")
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
		fmt.Printf("%s To add modules, use: %s\n", moduleInfoIcon, CmdStyle.Render("invowk module add <git-url> <version>"))
		return nil
	}

	fmt.Printf("%s Found %d module dependency(ies)\n", moduleInfoIcon, len(deps))
	fmt.Println()

	for _, dep := range deps {
		fmt.Printf("%s %s\n", moduleSuccessIcon, CmdStyle.Render(string(dep.Namespace)))
		fmt.Printf("   Git URL:  %s\n", dep.ModuleRef.GitURL)
		fmt.Printf("   Version:  %s → %s\n", dep.ModuleRef.Version, moduleDetailStyle.Render(string(dep.ResolvedVersion)))
		if len(dep.GitCommit) >= 12 {
			fmt.Printf("   Commit:   %s\n", moduleDetailStyle.Render(string(dep.GitCommit[:12])))
		}
		fmt.Printf("   Cache:    %s\n", moduleDetailStyle.Render(string(dep.CachePath)))
		fmt.Println()
	}

	return nil
}

// extractModuleRequirementsFromMetadata extracts module requirements from Invowkmod.
func extractModuleRequirementsFromMetadata(meta *invowkfile.Invowkmod) []invowkmod.ModuleRef {
	var reqs []invowkmod.ModuleRef

	if meta == nil || meta.Requires == nil {
		return reqs
	}

	for _, r := range meta.Requires {
		reqs = append(reqs, invowkmod.ModuleRef(r))
	}

	return reqs
}
