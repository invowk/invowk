// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// DeclarationEditResult describes the invowkmod.cue declaration mutation
	// paired with a lock-file mutation.
	DeclarationEditResult struct {
		updated bool
		err     error
	}

	// AddModuleDependencyResult is the structured result of adding one module
	// dependency to both the lock file and invowkmod.cue.
	AddModuleDependencyResult struct {
		resolved    *ResolvedModule
		declaration DeclarationEditResult
	}

	// RemoveModuleDependencyResult is the structured result of removing matching
	// module dependencies from the lock file and invowkmod.cue.
	RemoveModuleDependencyResult struct {
		removed      []RemoveResult
		declarations []DeclarationEditResult
	}
)

// NewDeclarationEditResult constructs a structured declaration-edit result.
func NewDeclarationEditResult(updated bool, err error) (DeclarationEditResult, error) {
	result := DeclarationEditResult{updated: updated, err: err}
	if validateErr := result.Validate(); validateErr != nil {
		return DeclarationEditResult{}, validateErr
	}
	return result, nil
}

// NewAddModuleDependencyResult constructs the result for a module add operation.
func NewAddModuleDependencyResult(resolved *ResolvedModule, declaration DeclarationEditResult) (AddModuleDependencyResult, error) {
	result := AddModuleDependencyResult{resolved: resolved, declaration: declaration}
	if err := result.Validate(); err != nil {
		return AddModuleDependencyResult{}, err
	}
	return result, nil
}

// NewRemoveModuleDependencyResult constructs the result for a module remove operation.
func NewRemoveModuleDependencyResult(removed []RemoveResult, declarations []DeclarationEditResult) (RemoveModuleDependencyResult, error) {
	result := RemoveModuleDependencyResult{removed: removed, declarations: declarations}
	if err := result.Validate(); err != nil {
		return RemoveModuleDependencyResult{}, err
	}
	return result, nil
}

// Updated reports whether invowkmod.cue was edited.
func (r DeclarationEditResult) Updated() bool { return r.updated }

// Err returns the declaration edit error, if any.
func (r DeclarationEditResult) Err() error { return r.err }

// Validate returns nil because declaration edit results contain only scalar
// status and an optional underlying error.
func (r DeclarationEditResult) Validate() error { return nil }

// Resolved returns the added resolved module.
func (r AddModuleDependencyResult) Resolved() *ResolvedModule { return r.resolved }

// Declaration returns the paired invowkmod.cue declaration edit result.
func (r AddModuleDependencyResult) Declaration() DeclarationEditResult { return r.declaration }

// Removed returns the lock-file entries removed by the operation.
func (r RemoveModuleDependencyResult) Removed() []RemoveResult { return r.removed }

// Declarations returns the declaration edit results paired with Removed().
func (r RemoveModuleDependencyResult) Declarations() []DeclarationEditResult { return r.declarations }

// LoadRequirements parses invowkmod.cue and returns the module dependency
// requirements used by module sync/tidy/vendor use cases.
func LoadRequirements(invowkmodPath types.FilesystemPath) ([]ModuleRef, error) {
	meta, err := invowkmod.ParseInvowkmod(invowkmodPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse invowkmod.cue: %w", err)
	}
	return invowkmod.ModuleRefsFromRequirements(meta.Requires), nil
}

// AddModuleDependency resolves req, updates the lock file, and adds the
// corresponding requires entry to invowkmodPath.
func AddModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, req ModuleRef) (AddModuleDependencyResult, error) {
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.AddModuleDependency(ctx, invowkmodPath, req)
}

// AddModuleDependency resolves req with this resolver, updates the lock file,
// and adds the corresponding requires entry to invowkmodPath.
func (m *Resolver) AddModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, req ModuleRef) (AddModuleDependencyResult, error) {
	resolved, err := m.Add(ctx, req)
	if err != nil {
		return AddModuleDependencyResult{}, err
	}
	editErr := invowkmod.AddRequirement(invowkmodPath, req)
	declaration, err := NewDeclarationEditResult(editErr == nil, editErr)
	if err != nil {
		return AddModuleDependencyResult{}, err
	}
	return NewAddModuleDependencyResult(resolved, declaration)
}

// RemoveModuleDependency removes matching modules from the lock file and
// removes their requires entries from invowkmodPath.
//
//goplint:ignore -- module remove identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func RemoveModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (RemoveModuleDependencyResult, error) {
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return RemoveModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.RemoveModuleDependency(ctx, invowkmodPath, identifier)
}

// RemoveModuleDependency removes matching modules with this resolver and
// removes their requires entries from invowkmodPath.
func (m *Resolver) RemoveModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (RemoveModuleDependencyResult, error) {
	removed, err := m.Remove(ctx, identifier)
	if err != nil {
		return RemoveModuleDependencyResult{}, err
	}

	declarations := make([]DeclarationEditResult, 0, len(removed))
	for i := range removed {
		editErr := invowkmod.RemoveRequirement(invowkmodPath, removed[i].RemovedEntry.GitURL, removed[i].RemovedEntry.Path)
		declaration, declarationErr := NewDeclarationEditResult(editErr == nil, editErr)
		if declarationErr != nil {
			return RemoveModuleDependencyResult{}, declarationErr
		}
		declarations = append(declarations, declaration)
	}

	return NewRemoveModuleDependencyResult(removed, declarations)
}

func newResolverForInvowkmodPath(invowkmodPath types.FilesystemPath) (*Resolver, error) {
	workingDir := types.FilesystemPath(filepath.Dir(string(invowkmodPath)))
	if err := workingDir.Validate(); err != nil {
		return nil, fmt.Errorf("invalid module working directory: %w", err)
	}
	return NewResolver(workingDir, "")
}

// Validate returns nil if the add result contains a valid resolved module and
// declaration edit result.
func (r AddModuleDependencyResult) Validate() error {
	var errs []error
	if r.resolved != nil {
		if err := r.resolved.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.declaration.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Validate returns nil if the remove result contains valid removed modules and
// declaration edit results.
func (r RemoveModuleDependencyResult) Validate() error {
	var errs []error
	for i := range r.removed {
		if err := r.removed[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for i := range r.declarations {
		if err := r.declarations[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtCreateModuleResolver, err)
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

	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtCreateModuleResolver, err)
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

// UpdateModule updates resolved module dependencies in the lock file next to invowkmodPath.
//
//goplint:ignore -- module update identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func UpdateModule(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) ([]*ResolvedModule, error) {
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.Update(ctx, identifier)
}

// ListModuleDependencies lists resolved module dependencies from the lock file
// next to invowkmodPath.
func ListModuleDependencies(ctx context.Context, invowkmodPath types.FilesystemPath) ([]*ResolvedModule, error) {
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.List(ctx)
}
