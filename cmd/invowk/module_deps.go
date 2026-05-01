// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

const (
	invowkmodCueFileName = "invowkmod.cue"
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
			return runModuleAdd(cmd.Context(), args, addAlias, addPath)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleRemove(cmd.Context(), args)
		},
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

Invowk uses the explicit-only dependency model (like Go modules): every module
in the dependency tree must be declared in YOUR invowkmod.cue. If a module you
depend on requires another module, sync will fail with an actionable error
listing the missing declarations. Run 'invowk module tidy' to auto-add them.

Examples:
  invowk module sync`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModuleSync(cmd.Context())
		},
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleUpdate(cmd.Context(), args)
		},
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModuleDeps(cmd.Context())
		},
	}
}

func runModuleAdd(ctx context.Context, args []string, addAlias, addPath string) error {
	gitURL := args[0]
	version := args[1]

	fmt.Println(moduleTitleStyle.Render("Add Module Dependency"))

	// Create requirement
	req := invowkmod.ModuleRef{
		GitURL:  invowkmod.GitURL(gitURL),
		Version: invowkmod.SemVerConstraint(version),
		Alias:   invowkmod.ModuleAlias(addAlias),
		Path:    invowkmod.SubdirectoryPath(addPath),
	}

	fmt.Printf("%s Resolving %s@%s...\n", moduleInfoIcon, gitURL, version)

	invowkmodPath := filepath.Join(".", invowkmodCueFileName)
	result, err := modulesync.AddModuleDependency(ctx, types.FilesystemPath(invowkmodPath), req) //goplint:ignore -- relative path from current dir
	if err != nil {
		fmt.Printf("%s Failed to add module: %v\n", moduleErrorIcon, err)
		return err
	}

	resolved := result.Resolved()
	fmt.Printf("%s Module resolved\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Git URL:   %s\n", moduleInfoIcon, modulePathStyle.Render(string(resolved.ModuleRef.GitURL)))
	fmt.Printf("%s Version:   %s → %s\n", moduleInfoIcon, version, CmdStyle.Render(string(resolved.ResolvedVersion)))
	fmt.Printf("%s Namespace: %s\n", moduleInfoIcon, CmdStyle.Render(string(resolved.Namespace)))
	fmt.Printf("%s Cache:     %s\n", moduleInfoIcon, moduleDetailStyle.Render(string(resolved.CachePath)))

	declaration := result.Declaration()
	if declaration.Err() != nil {
		if os.IsNotExist(declaration.Err()) {
			fmt.Println()
			fmt.Printf("%s invowkmod.cue not found — dependency was not persisted\n", moduleInfoIcon)
		} else {
			fmt.Println()
			fmt.Printf("%s Could not auto-edit invowkmod.cue; lock file changes were rolled back: %v\n", moduleInfoIcon, declaration.Err())
		}
	} else {
		fmt.Printf("%s Module lock file updated\n", moduleSuccessIcon)
		fmt.Printf("%s Updated invowkmod.cue with new requires entry\n", moduleSuccessIcon)
	}

	return nil
}

func runModuleRemove(ctx context.Context, args []string) error {
	identifier := args[0]

	fmt.Println(moduleTitleStyle.Render("Remove Module Dependency"))

	fmt.Printf("%s Removing %s...\n", moduleInfoIcon, identifier)
	invowkmodPath := filepath.Join(".", invowkmodCueFileName)
	result, err := modulesync.RemoveModuleDependency(ctx, types.FilesystemPath(invowkmodPath), identifier) //goplint:ignore -- relative path from current dir
	if err != nil {
		// Format ambiguous matches nicely
		if ambigErr, ok := errors.AsType[*modulesync.AmbiguousIdentifierError](err); ok && ambigErr != nil {
			fmt.Printf("%s %v\n", moduleErrorIcon, ambigErr)
			return err
		}
		fmt.Printf("%s Failed to remove module: %v\n", moduleErrorIcon, err)
		return err
	}

	declarations := result.Declarations()
	removed := result.Removed()
	for i := range declarations {
		if declarations[i].Err() != nil {
			fmt.Printf("%s Could not auto-edit invowkmod.cue: %v\n", moduleInfoIcon, declarations[i].Err())
		}
	}

	for i := range removed {
		fmt.Printf("%s Removed %s\n", moduleSuccessIcon, CmdStyle.Render(string(removed[i].RemovedEntry.Namespace)))
	}

	fmt.Println()
	if moduleDeclarationEditsClean(declarations) {
		fmt.Printf("%s Lock file and invowkmod.cue updated\n", moduleSuccessIcon)
	} else {
		fmt.Printf("%s Lock file updated; invowkmod.cue needs manual review\n", moduleInfoIcon)
	}

	return nil
}

func moduleDeclarationEditsClean(edits []modulesync.DeclarationEditResult) bool {
	for i := range edits {
		if edits[i].Err() != nil {
			return false
		}
	}
	return true
}

func runModuleSync(ctx context.Context) error {
	fmt.Println(moduleTitleStyle.Render("Sync Module Dependencies"))

	invowkmodulePath := filepath.Join(".", invowkmodCueFileName)
	requirements, resolved, err := modulesync.SyncModule(ctx, types.FilesystemPath(invowkmodulePath)) //goplint:ignore -- relative path from current dir
	if err != nil {
		fmt.Printf("%s Failed to sync modules: %v\n", moduleErrorIcon, err)
		return err
	}

	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invowkmod.cue\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invowkmod.cue\n", moduleInfoIcon, len(requirements))

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

func runModuleUpdate(ctx context.Context, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Update Module Dependencies"))

	var identifier string
	if len(args) > 0 {
		identifier = args[0]
		fmt.Printf("%s Updating %s...\n", moduleInfoIcon, identifier)
	} else {
		fmt.Printf("%s Updating all modules...\n", moduleInfoIcon)
	}

	invowkmodPath := filepath.Join(".", invowkmodCueFileName)
	updated, err := modulesync.UpdateModule(ctx, types.FilesystemPath(invowkmodPath), identifier) //goplint:ignore -- relative path from current dir
	if err != nil {
		// Format ambiguous matches nicely
		if ambigErr, ok := errors.AsType[*modulesync.AmbiguousIdentifierError](err); ok && ambigErr != nil {
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

func runModuleDeps(ctx context.Context) error {
	fmt.Println(moduleTitleStyle.Render("Module Dependencies"))

	invowkmodPath := filepath.Join(".", invowkmodCueFileName)
	deps, err := modulesync.ListModuleDependencies(ctx, types.FilesystemPath(invowkmodPath)) //goplint:ignore -- relative path from current dir
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
