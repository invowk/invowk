// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// snapshotMissing: nothing existed at the path; restore removes it.
	snapshotMissing fileSnapshotKind = "missing"
	// snapshotRegular: a regular file; restore writes its bytes back.
	snapshotRegular fileSnapshotKind = "regular"
	// snapshotSymlink: a symbolic link; restore recreates the same link.
	snapshotSymlink fileSnapshotKind = "symlink"
	// snapshotOther: a directory, device or other node; restore leaves it.
	snapshotOther fileSnapshotKind = "other"
)

type (
	fileSnapshotData []byte

	// fileSnapshotKind is what a rollback snapshot found at its path, judged
	// without following symbolic links (os.Lstat).
	fileSnapshotKind string

	fileSnapshot struct {
		path *types.FilesystemPath
		kind fileSnapshotKind
		data fileSnapshotData
		mode os.FileMode
		// linkTarget is the os.Readlink target of a symbolic-link snapshot.
		linkTarget *types.FilesystemPath
	}

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
		lockWritten bool
		declaration DeclarationEditResult
	}

	// RemoveModuleDependencyResult is the structured result of removing matching
	// module dependencies from the lock file and invowkmod.cue.
	RemoveModuleDependencyResult struct {
		removed      []RemoveResult
		lockWritten  bool
		declarations []DeclarationEditResult
	}

	// SyncModuleResult is the structured result of syncing the requirements
	// declared in invowkmod.cue into the lock file.
	SyncModuleResult struct {
		requirements []ModuleRef
		resolved     []*ResolvedModule
		lockWritten  bool
	}

	// UpdateModuleResult is the structured result of updating locked module
	// dependencies to their latest matching versions.
	UpdateModuleResult struct {
		updated     []*ResolvedModule
		lockWritten bool
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
// lockWritten reports whether the lock save wrote the file.
func NewAddModuleDependencyResult(resolved *ResolvedModule, lockWritten bool, declaration DeclarationEditResult) (AddModuleDependencyResult, error) {
	result := AddModuleDependencyResult{resolved: resolved, lockWritten: lockWritten, declaration: declaration}
	if err := result.Validate(); err != nil {
		return AddModuleDependencyResult{}, err
	}
	return result, nil
}

// NewRemoveModuleDependencyResult constructs the result for a module remove operation.
// lockWritten reports whether the lock save wrote the file.
func NewRemoveModuleDependencyResult(removed []RemoveResult, lockWritten bool, declarations []DeclarationEditResult) (RemoveModuleDependencyResult, error) {
	result := RemoveModuleDependencyResult{removed: removed, lockWritten: lockWritten, declarations: declarations}
	if err := result.Validate(); err != nil {
		return RemoveModuleDependencyResult{}, err
	}
	return result, nil
}

// NewSyncModuleResult constructs the result for a module sync operation.
// lockWritten reports whether the lock save wrote the file; it is false when
// there were no requirements and nothing was saved.
func NewSyncModuleResult(requirements []ModuleRef, resolved []*ResolvedModule, lockWritten bool) (SyncModuleResult, error) {
	result := SyncModuleResult{requirements: requirements, resolved: resolved, lockWritten: lockWritten}
	if err := result.Validate(); err != nil {
		return SyncModuleResult{}, err
	}
	return result, nil
}

// NewUpdateModuleResult constructs the result for a module update operation.
// lockWritten reports whether the lock save wrote the file.
func NewUpdateModuleResult(updated []*ResolvedModule, lockWritten bool) (UpdateModuleResult, error) {
	result := UpdateModuleResult{updated: updated, lockWritten: lockWritten}
	if err := result.Validate(); err != nil {
		return UpdateModuleResult{}, err
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

// LockWritten reports whether the lock save wrote the lock file. It reports
// the save itself, also when a later declaration edit failed and the lock was
// rolled back.
func (r AddModuleDependencyResult) LockWritten() bool { return r.lockWritten }

// Declaration returns the paired invowkmod.cue declaration edit result.
func (r AddModuleDependencyResult) Declaration() DeclarationEditResult { return r.declaration }

// Removed returns the lock-file entries removed by the operation.
func (r RemoveModuleDependencyResult) Removed() []RemoveResult { return r.removed }

// LockWritten reports whether the lock save wrote the lock file. It reports
// the save itself, also when a later declaration edit failed and the lock was
// rolled back.
func (r RemoveModuleDependencyResult) LockWritten() bool { return r.lockWritten }

// Declarations returns the declaration edit results paired with Removed().
func (r RemoveModuleDependencyResult) Declarations() []DeclarationEditResult { return r.declarations }

// Requirements returns the requirements declared in invowkmod.cue.
func (r SyncModuleResult) Requirements() []ModuleRef { return r.requirements }

// Resolved returns the modules the sync resolved and locked.
func (r SyncModuleResult) Resolved() []*ResolvedModule { return r.resolved }

// LockWritten reports whether the sync wrote the lock file.
func (r SyncModuleResult) LockWritten() bool { return r.lockWritten }

// Updated returns the modules the update re-resolved.
func (r UpdateModuleResult) Updated() []*ResolvedModule { return r.updated }

// LockWritten reports whether the update wrote the lock file.
func (r UpdateModuleResult) LockWritten() bool { return r.lockWritten }

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
	if err := req.ValidateDeclaration(); err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf("invalid requirement declaration: %w", err)
	}
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.AddModuleDependency(ctx, invowkmodPath, req)
}

// AddModuleDependency resolves req with this resolver, updates the lock file,
// and adds the corresponding requires entry to invowkmodPath.
func (m *Resolver) AddModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, req ModuleRef) (AddModuleDependencyResult, error) {
	if err := req.ValidateDeclaration(); err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf("invalid requirement declaration: %w", err)
	}
	lockSnapshot, snapshotErr := m.readLockSnapshot()
	if snapshotErr != nil {
		return AddModuleDependencyResult{}, snapshotErr
	}
	declarationSnapshot, snapshotErr := readFileSnapshot(invowkmodPath)
	if snapshotErr != nil {
		return AddModuleDependencyResult{}, snapshotErr
	}
	resolved, lockWritten, err := m.Add(ctx, req)
	if err != nil {
		return AddModuleDependencyResult{}, err
	}
	editErr := invowkmod.AddRequirement(invowkmodPath, req)
	if editErr != nil {
		editErr = errors.Join(editErr, declarationSnapshot.restore(), lockSnapshot.restore())
	}
	declaration, err := NewDeclarationEditResult(editErr == nil, editErr)
	if err != nil {
		return AddModuleDependencyResult{}, err
	}
	return NewAddModuleDependencyResult(resolved, lockWritten, declaration)
}

// RemoveModuleDependency removes matching modules from the lock file and
// removes their requires entries from invowkmodPath.
//
//goplint:ignore -- module remove identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func RemoveModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (RemoveModuleDependencyResult, error) {
	resolver, err := newLockOnlyResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return RemoveModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.RemoveModuleDependency(ctx, invowkmodPath, identifier)
}

// RemoveModuleDependency removes matching modules with this resolver and
// removes their requires entries from invowkmodPath.
func (m *Resolver) RemoveModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (RemoveModuleDependencyResult, error) {
	lockSnapshot, snapshotErr := m.readLockSnapshot()
	if snapshotErr != nil {
		return RemoveModuleDependencyResult{}, snapshotErr
	}
	declarationSnapshot, snapshotErr := readFileSnapshot(invowkmodPath)
	if snapshotErr != nil {
		return RemoveModuleDependencyResult{}, snapshotErr
	}
	removed, lockWritten, err := m.Remove(ctx, identifier)
	if err != nil {
		return RemoveModuleDependencyResult{}, err
	}

	declarations := make([]DeclarationEditResult, 0, len(removed))
	for i := range removed {
		written, editErr := invowkmod.RemoveRequirementIfPresent(invowkmodPath, removed[i].RemovedEntry.GitURL, removed[i].RemovedEntry.Path)
		if editErr != nil {
			editErr = errors.Join(editErr, declarationSnapshot.restore(), lockSnapshot.restore())
			declaration, declarationErr := NewDeclarationEditResult(false, editErr)
			if declarationErr != nil {
				return RemoveModuleDependencyResult{}, declarationErr
			}
			declarations = append(declarations, declaration)
			return NewRemoveModuleDependencyResult(removed, lockWritten, declarations)
		}
		declaration, declarationErr := NewDeclarationEditResult(written, nil)
		if declarationErr != nil {
			return RemoveModuleDependencyResult{}, declarationErr
		}
		declarations = append(declarations, declaration)
	}

	return NewRemoveModuleDependencyResult(removed, lockWritten, declarations)
}

// String returns the snapshot kind name.
func (k fileSnapshotKind) String() string { return string(k) }

// Validate returns an error when k is not one of the snapshot kinds.
func (k fileSnapshotKind) Validate() error {
	switch k {
	case snapshotMissing, snapshotRegular, snapshotSymlink, snapshotOther:
		return nil
	default:
		return fmt.Errorf("invalid file snapshot kind %q", string(k))
	}
}

func (s fileSnapshot) Validate() error {
	if s.path == nil {
		return errors.New("file snapshot path is required")
	}
	if err := s.kind.Validate(); err != nil {
		return err
	}
	if s.kind == snapshotSymlink {
		if s.linkTarget == nil {
			return errors.New("file snapshot link target is required")
		}
		if err := s.linkTarget.Validate(); err != nil {
			return fmt.Errorf("file snapshot link target: %w", err)
		}
	}
	return s.path.Validate()
}

func (m *Resolver) readLockSnapshot() (fileSnapshot, error) {
	lockPath := types.FilesystemPath(filepath.Join(string(m.workingDir), LockFileName))
	if err := lockPath.Validate(); err != nil {
		return fileSnapshot{}, fmt.Errorf("validate lock file path: %w", err)
	}
	snapshot, err := readFileSnapshot(lockPath)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot lock file before dependency edit: %w", err)
	}
	return snapshot, nil
}

// readFileSnapshot records what is at path without following a symbolic link:
// a dangling link is a link, not a missing file, and a link to a regular file
// is restored as that link (design D8).
func readFileSnapshot(path types.FilesystemPath) (fileSnapshot, error) {
	snapshot := fileSnapshot{path: &path}
	info, err := os.Lstat(string(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			snapshot.kind = snapshotMissing
			return snapshot, nil
		}
		return fileSnapshot{}, fmt.Errorf("stat file before dependency edit: %w", err)
	}
	snapshot.mode = info.Mode()
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, readErr := os.Readlink(string(path))
		if readErr != nil {
			return fileSnapshot{}, fmt.Errorf("read link before dependency edit: %w", readErr)
		}
		linkTarget := types.FilesystemPath(target) //goplint:ignore -- os.Readlink result, validated with the snapshot.
		snapshot.kind = snapshotSymlink
		snapshot.linkTarget = &linkTarget
	case info.Mode().IsRegular():
		data, readErr := os.ReadFile(string(path))
		if readErr != nil {
			return fileSnapshot{}, fmt.Errorf("read file before dependency edit: %w", readErr)
		}
		snapshot.kind = snapshotRegular
		snapshot.data = fileSnapshotData(data)
	default:
		snapshot.kind = snapshotOther
	}
	return snapshot, nil
}

// restore puts the snapshot's path back as it was: a missing path is removed,
// a regular file gets its bytes and permission bits back, a symbolic link is
// recreated with its target, and any other node is left alone.
func (s fileSnapshot) restore() error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("invalid rollback snapshot: %w", err)
	}
	path := string(*s.path)
	switch s.kind {
	case snapshotMissing:
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("rollback remove %s: %w", *s.path, err)
		}
		return nil
	case snapshotRegular:
		if err := os.WriteFile(path, []byte(s.data), s.mode.Perm()); err != nil {
			return fmt.Errorf("rollback %s: %w", *s.path, err)
		}
		return nil
	case snapshotSymlink:
		return s.restoreLink()
	case snapshotOther:
		return nil
	default:
		return fmt.Errorf("rollback %s: invalid snapshot kind %q", *s.path, string(s.kind))
	}
}

// restoreLink recreates a symbolic-link snapshot. When the path is still the
// same link it does nothing; otherwise it creates the link under a random name
// in the same directory and renames it over the path, one atomic replace.
func (s fileSnapshot) restoreLink() error {
	path := string(*s.path)
	if current, err := os.Readlink(path); err == nil && current == string(*s.linkTarget) {
		return nil
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("rollback link %s: %w", *s.path, err)
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+hex.EncodeToString(suffix)+".link")
	if err := os.Symlink(string(*s.linkTarget), tmp); err != nil {
		return fmt.Errorf("rollback link %s: %w", *s.path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return errors.Join(fmt.Errorf("rollback link %s: %w", *s.path, err), os.Remove(tmp))
	}
	return nil
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

// Validate returns nil if the sync result contains valid requirements and
// resolved modules.
func (r SyncModuleResult) Validate() error {
	var errs []error
	for i := range r.requirements {
		if err := r.requirements[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := validateResolvedModules(r.resolved); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Validate returns nil if the update result contains valid resolved modules.
func (r UpdateModuleResult) Validate() error {
	return validateResolvedModules(r.updated)
}

func validateResolvedModules(modules []*ResolvedModule) error {
	var errs []error
	for _, mod := range modules {
		if mod == nil {
			continue
		}
		if err := mod.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SyncModule resolves the requirements declared in invowkmodPath and updates
// the module lock file. With no requirements it saves nothing and the result
// reports LockWritten() == false.
func SyncModule(ctx context.Context, invowkmodPath types.FilesystemPath) (SyncModuleResult, error) {
	requirements, err := LoadRequirements(invowkmodPath)
	if err != nil {
		return SyncModuleResult{}, err
	}
	if len(requirements) == 0 {
		return NewSyncModuleResult(requirements, nil, false)
	}

	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return SyncModuleResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	resolved, lockWritten, err := resolver.Sync(ctx, requirements)
	if err != nil {
		return SyncModuleResult{}, err
	}
	return NewSyncModuleResult(requirements, resolved, lockWritten)
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
func UpdateModule(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (UpdateModuleResult, error) {
	resolver, err := newResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return UpdateModuleResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	updated, lockWritten, err := resolver.Update(ctx, identifier)
	if err != nil {
		return UpdateModuleResult{}, err
	}
	return NewUpdateModuleResult(updated, lockWritten)
}

// ListModuleDependencies lists resolved module dependencies from the lock file
// next to invowkmodPath.
func ListModuleDependencies(ctx context.Context, invowkmodPath types.FilesystemPath) ([]*ResolvedModule, error) {
	resolver, err := newLockOnlyResolverForInvowkmodPath(invowkmodPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.List(ctx)
}

func newLockOnlyResolverForInvowkmodPath(invowkmodPath types.FilesystemPath) (*Resolver, error) {
	workingDir := types.FilesystemPath(filepath.Dir(string(invowkmodPath)))
	if err := workingDir.Validate(); err != nil {
		return nil, fmt.Errorf("invalid module working directory: %w", err)
	}
	return newLockOnlyResolver(workingDir)
}
