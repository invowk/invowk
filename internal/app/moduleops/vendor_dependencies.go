// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// VendorResolutionUpdated means dependencies were re-resolved and the lock file was updated.
	VendorResolutionUpdated VendorResolutionStrategy = "updated"
	// VendorResolutionLocked means dependencies were loaded from the existing lock file.
	VendorResolutionLocked VendorResolutionStrategy = "locked"
	// VendorResolutionSynced means dependencies were resolved because no lock file existed.
	VendorResolutionSynced VendorResolutionStrategy = "synced"
)

type (
	// VendorResolutionStrategy describes how vendored dependencies were resolved.
	VendorResolutionStrategy string

	vendorDependencyResolver interface {
		Sync(ctx context.Context, requirements []invowkmod.ModuleRef) ([]*invowkmod.ResolvedModule, error)
		LoadDeclaredFromLock(ctx context.Context, requirements []invowkmod.ModuleRef) ([]*invowkmod.ResolvedModule, error)
	}
)

// String returns the string representation of the VendorResolutionStrategy.
func (s VendorResolutionStrategy) String() string { return string(s) }

// Validate returns nil when the strategy is one of the known vendor strategies.
func (s VendorResolutionStrategy) Validate() error {
	switch s {
	case VendorResolutionUpdated, VendorResolutionLocked, VendorResolutionSynced:
		return nil
	default:
		return fmt.Errorf("invalid vendor resolution strategy %q", s)
	}
}

// VendorDependencies resolves module dependencies for modulePath and copies
// them into invowk_modules/ according to the update/prune policy.
func VendorDependencies(ctx context.Context, modulePath types.FilesystemPath, update, prune bool) ([]invowkmod.ModuleRef, *VendorResult, VendorResolutionStrategy, error) {
	if err := modulePath.Validate(); err != nil {
		return nil, nil, "", fmt.Errorf("module path: %w", err)
	}

	absPath, err := filepath.Abs(string(modulePath))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to resolve path: %w", err)
	}
	modulePath = types.FilesystemPath(absPath) //goplint:ignore -- filepath.Abs result validated below.
	if validateErr := modulePath.Validate(); validateErr != nil {
		return nil, nil, "", fmt.Errorf("absolute module path: %w", validateErr)
	}

	invowkmodPath := types.FilesystemPath(filepath.Join(absPath, "invowkmod.cue")) //goplint:ignore -- derived from validated module path and constant filename
	if _, statErr := os.Stat(string(invowkmodPath)); os.IsNotExist(statErr) {
		return nil, nil, "", fmt.Errorf("not a module directory (no invowkmod.cue found in %s)", absPath)
	} else if statErr != nil {
		return nil, nil, "", fmt.Errorf("failed to stat invowkmod.cue: %w", statErr)
	}

	requirements, err := modulesync.LoadRequirements(invowkmodPath)
	if err != nil {
		return nil, nil, "", err
	}
	if len(requirements) == 0 {
		return requirements, nil, "", nil
	}

	resolver, err := modulesync.NewResolver(modulePath, "")
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create resolver: %w", err)
	}

	return vendorDependenciesWithResolver(ctx, modulePath, requirements, resolver, update, prune)
}

func vendorDependenciesWithResolver(
	ctx context.Context,
	modulePath types.FilesystemPath,
	requirements []invowkmod.ModuleRef,
	resolver vendorDependencyResolver,
	update bool,
	prune bool,
) ([]invowkmod.ModuleRef, *VendorResult, VendorResolutionStrategy, error) {
	resolved, strategy, err := resolveVendorDependencies(ctx, resolver, modulePath, requirements, update)
	if err != nil {
		return requirements, nil, strategy, err
	}

	result, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    resolved,
		Prune:      prune,
	})
	if err != nil {
		return requirements, nil, "", fmt.Errorf("failed to vendor modules: %w", err)
	}
	return requirements, result, strategy, nil
}

func resolveVendorDependencies(ctx context.Context, resolver vendorDependencyResolver, modulePath types.FilesystemPath, requirements []invowkmod.ModuleRef, update bool) ([]*invowkmod.ResolvedModule, VendorResolutionStrategy, error) {
	if update {
		resolved, err := resolver.Sync(ctx, requirements)
		return resolved, VendorResolutionUpdated, err
	}

	lockPath := types.FilesystemPath(filepath.Join(string(modulePath), invowkmod.LockFileName)) //goplint:ignore -- derived from validated module path and constant filename
	lockSnapshot := invowkmod.InspectLockFile(lockPath)
	if lockSnapshot.StatErr != nil || lockSnapshot.ParseErr != nil {
		return nil, "", fmt.Errorf("failed to inspect lock file: %w", errors.Join(lockSnapshot.StatErr, lockSnapshot.ParseErr))
	}
	if lockSnapshot.Present {
		resolved, err := resolver.LoadDeclaredFromLock(ctx, requirements)
		return resolved, VendorResolutionLocked, err
	}

	resolved, err := resolver.Sync(ctx, requirements)
	return resolved, VendorResolutionSynced, err
}
