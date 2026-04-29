// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"fmt"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// LoadRequirements parses invowkmod.cue and returns the module dependency
// requirements used by module sync/tidy/vendor use cases.
func LoadRequirements(invowkmodPath types.FilesystemPath) ([]ModuleRef, error) {
	meta, err := invowkmod.ParseInvowkmod(invowkmodPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse invowkmod.cue: %w", err)
	}
	return invowkmod.ModuleRefsFromRequirements(meta.Requires), nil
}

// SyncModule resolves the requirements declared in invowkmodPath and updates
// the module lock file.
func SyncModule(ctx context.Context, invowkmodPath types.FilesystemPath) (requirements []ModuleRef, resolved []*ResolvedModule, err error) {
	requirements, err = LoadRequirements(invowkmodPath)
	if err != nil {
		return nil, nil, err
	}
	if len(requirements) == 0 {
		return requirements, nil, nil
	}

	resolver, err := NewResolver("", "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create module resolver: %w", err)
	}
	resolved, err = resolver.Sync(ctx, requirements)
	if err != nil {
		return requirements, nil, err
	}
	return requirements, resolved, nil
}

// TidyModule resolves declared requirements, adds missing transitive
// requirements to invowkmodPath, and returns the added refs.
func TidyModule(ctx context.Context, invowkmodPath types.FilesystemPath) (requirements, missing []ModuleRef, err error) {
	requirements, err = LoadRequirements(invowkmodPath)
	if err != nil {
		return nil, nil, err
	}
	if len(requirements) == 0 {
		return requirements, nil, nil
	}

	resolver, err := NewResolver("", "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create module resolver: %w", err)
	}
	missing, err = resolver.Tidy(ctx, requirements)
	if err != nil {
		return requirements, nil, err
	}
	for _, req := range missing {
		if addErr := invowkmod.AddRequirement(invowkmodPath, req); addErr != nil {
			return requirements, missing, addErr
		}
	}
	return requirements, missing, nil
}
