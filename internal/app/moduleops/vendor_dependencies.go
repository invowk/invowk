// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
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

	invowkmodPath := types.FilesystemPath(filepath.Join(string(modulePath), "invowkmod.cue")) //goplint:ignore -- derived from validated module path and constant filename
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

func resolveVendorDependencies(ctx context.Context, resolver *modulesync.Resolver, modulePath types.FilesystemPath, requirements []invowkmod.ModuleRef, update bool) ([]*invowkmod.ResolvedModule, VendorResolutionStrategy, error) {
	lockPath := filepath.Join(string(modulePath), invowkmod.LockFileName)
	_, lockErr := os.Stat(lockPath)

	switch {
	case update:
		resolved, err := resolver.Sync(ctx, requirements)
		return resolved, VendorResolutionUpdated, err
	case lockErr == nil:
		resolved, err := resolver.LoadFromLock(ctx)
		return resolved, VendorResolutionLocked, err
	default:
		resolved, err := resolver.Sync(ctx, requirements)
		return resolved, VendorResolutionSynced, err
	}
}
