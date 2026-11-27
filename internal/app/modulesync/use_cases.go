// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invowk/invowk/pkg/fspath"
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

	// DeclarationRollbackNone means no invowkmod.cue edit failed, so nothing
	// was rolled back.
	DeclarationRollbackNone DeclarationRollback = "none"
	// DeclarationRollbackComplete means an invowkmod.cue edit failed and every
	// restore of the rollback succeeded.
	DeclarationRollbackComplete DeclarationRollback = "complete"
	// DeclarationRollbackIncomplete means an invowkmod.cue edit failed and at
	// least one restore failed too; Err carries the restore errors.
	DeclarationRollbackIncomplete DeclarationRollback = "incomplete"
)

type (
	fileSnapshotData []byte

	// fileSnapshotKind is what a rollback snapshot found at its path, judged
	// without following symbolic links (os.Lstat).
	fileSnapshotKind string

	// restoreFileWriter replaces a regular file during a rollback; the default
	// is fspath.AtomicWriteFile (design D8).
	//
	//goplint:ignore -- seam mirrors the os.WriteFile-style primitive it replaces.
	restoreFileWriter func(path string, data []byte, perm os.FileMode) error

	// restoreLinkCreator creates a symbolic link during a rollback; the
	// default is os.Symlink (design D8).
	//
	//goplint:ignore -- seam mirrors the os.Symlink primitive it replaces.
	restoreLinkCreator func(target, link string) error

	fileSnapshot struct {
		path *types.FilesystemPath
		kind fileSnapshotKind
		data fileSnapshotData
		mode os.FileMode
		// linkTarget is the os.Readlink target of a symbolic-link snapshot.
		linkTarget *types.FilesystemPath
		// targetPath is the resolved regular-file destination when kind ==
		// snapshotSymlink and the symlink resolved to a regular file at
		// snapshot time (nil for dangling or non-regular symlinks).
		targetPath *types.FilesystemPath
		// writeFile and createLink are the Resolver's restore seams, carried
		// into the snapshot when it is taken.
		writeFile  restoreFileWriter
		createLink restoreLinkCreator
	}

	// DeclarationRollback is the outcome of the rollback after a failed
	// invowkmod.cue edit, reported by the use case instead of being inferred
	// from the shape of its error.
	DeclarationRollback string

	// DeclarationEditResult describes the invowkmod.cue declaration mutation
	// paired with a lock-file mutation.
	DeclarationEditResult struct {
		updated  bool
		err      error
		rollback DeclarationRollback
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
// rollback is DeclarationRollbackNone when err is nil, and the rollback's
// outcome when the edit failed.
func NewDeclarationEditResult(updated bool, err error, rollback DeclarationRollback) (DeclarationEditResult, error) {
	result := DeclarationEditResult{updated: updated, err: err, rollback: rollback}
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

// Err returns the declaration edit error, if any, joined with the errors of
// the restores that failed.
func (r DeclarationEditResult) Err() error { return r.err }

// Rollback reports the rollback outcome: none when the edit succeeded or
// never ran (the zero result an error return carries), complete or
// incomplete after a failed edit.
func (r DeclarationEditResult) Rollback() DeclarationRollback {
	if r.rollback == "" {
		return DeclarationRollbackNone
	}
	return r.rollback
}

// Validate returns an error when the rollback outcome is unknown or does not
// match the error: a failed edit always rolls back, and a rollback always has
// the edit error.
// The zero result validates: its rollback reads as none, as Rollback says.
func (r DeclarationEditResult) Validate() error {
	if r.rollback != "" {
		if err := r.rollback.Validate(); err != nil {
			return err
		}
	}
	rollback := r.Rollback()
	if r.err != nil && rollback == DeclarationRollbackNone {
		return errors.New("declaration edit failed but reports no rollback")
	}
	if r.err == nil && rollback != DeclarationRollbackNone {
		return fmt.Errorf("declaration edit reports rollback %q without an edit error", rollback)
	}
	return nil
}

// String returns the rollback outcome name.
func (r DeclarationRollback) String() string { return string(r) }

// Validate returns an error when r is not one of the rollback outcomes.
func (r DeclarationRollback) Validate() error {
	switch r {
	case DeclarationRollbackNone, DeclarationRollbackComplete, DeclarationRollbackIncomplete:
		return nil
	default:
		return fmt.Errorf("invalid declaration rollback %q", string(r))
	}
}

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
// corresponding requires entry to invowkmodPath, under the project lock of
// locks.
func AddModuleDependency(ctx context.Context, locks EditLocks, invowkmodPath types.FilesystemPath, req ModuleRef) (AddModuleDependencyResult, error) {
	if err := req.ValidateDeclaration(); err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf("invalid requirement declaration: %w", err)
	}
	if err := locks.Validate(); err != nil {
		return AddModuleDependencyResult{}, err
	}
	resolver, err := newResolverForInvowkmodPath(invowkmodPath, locks)
	if err != nil {
		return AddModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.AddModuleDependency(ctx, invowkmodPath, req)
}

// AddModuleDependency resolves req with this resolver, updates the lock file,
// and adds the corresponding requires entry to invowkmodPath. It takes the
// project lock before it reads either file and releases it after the
// declaration edit or the last restore.
func (m *Resolver) AddModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, req ModuleRef) (result AddModuleDependencyResult, err error) {
	if declErr := req.ValidateDeclaration(); declErr != nil {
		return AddModuleDependencyResult{}, fmt.Errorf("invalid requirement declaration: %w", declErr)
	}
	unlock, err := m.LockProject(ctx)
	if err != nil {
		return AddModuleDependencyResult{}, err
	}
	defer func() { err = joinUnlock(err, unlock) }()

	lockSnapshot, snapshotErr := m.readLockSnapshot()
	if snapshotErr != nil {
		return AddModuleDependencyResult{}, snapshotErr
	}
	declarationSnapshot, snapshotErr := m.readFileSnapshot(invowkmodPath)
	if snapshotErr != nil {
		return AddModuleDependencyResult{}, snapshotErr
	}
	resolved, lockWritten, addErr := m.Add(ctx, req)
	if addErr != nil {
		return AddModuleDependencyResult{}, addErr
	}
	editErr := invowkmod.AddRequirement(invowkmodPath, req)
	rollback := DeclarationRollbackNone
	if editErr != nil {
		// Built exactly as in Resolver.RemoveModuleDependency's rollback; keep
		// the two in step (design D9).
		rollback, editErr = rollBackDeclarationEdit(editErr, declarationSnapshot, lockSnapshot)
	}
	declaration, declarationErr := NewDeclarationEditResult(editErr == nil, editErr, rollback)
	if declarationErr != nil {
		return AddModuleDependencyResult{}, declarationErr
	}
	return NewAddModuleDependencyResult(resolved, lockWritten, declaration)
}

// RemoveModuleDependency removes matching modules from the lock file and
// removes their requires entries from invowkmodPath.
//
//goplint:ignore -- module remove identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func RemoveModuleDependency(ctx context.Context, locks EditLocks, invowkmodPath types.FilesystemPath, identifier string) (RemoveModuleDependencyResult, error) {
	if err := locks.Validate(); err != nil {
		return RemoveModuleDependencyResult{}, err
	}
	resolver, err := newLockOnlyResolverForInvowkmodPath(invowkmodPath, WithEditLocks(locks))
	if err != nil {
		return RemoveModuleDependencyResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.RemoveModuleDependency(ctx, invowkmodPath, identifier)
}

// RemoveModuleDependency removes matching modules with this resolver and
// removes their requires entries from invowkmodPath. It takes the project
// lock before it reads either file and releases it after the last edit or the
// last restore.
func (m *Resolver) RemoveModuleDependency(ctx context.Context, invowkmodPath types.FilesystemPath, identifier string) (result RemoveModuleDependencyResult, err error) {
	unlock, err := m.LockProject(ctx)
	if err != nil {
		return RemoveModuleDependencyResult{}, err
	}
	defer func() { err = joinUnlock(err, unlock) }()

	lockSnapshot, snapshotErr := m.readLockSnapshot()
	if snapshotErr != nil {
		return RemoveModuleDependencyResult{}, snapshotErr
	}
	declarationSnapshot, snapshotErr := m.readFileSnapshot(invowkmodPath)
	if snapshotErr != nil {
		return RemoveModuleDependencyResult{}, snapshotErr
	}
	removed, lockWritten, removeErr := m.Remove(ctx, identifier)
	if removeErr != nil {
		return RemoveModuleDependencyResult{}, removeErr
	}

	declarations := make([]DeclarationEditResult, 0, len(removed))
	for i := range removed {
		written, editErr := invowkmod.RemoveRequirementIfPresent(invowkmodPath, removed[i].RemovedEntry.GitURL, removed[i].RemovedEntry.Path)
		if editErr != nil {
			// Built exactly as in Resolver.AddModuleDependency's rollback; keep
			// the two in step (design D9).
			rollback, joinedErr := rollBackDeclarationEdit(editErr, declarationSnapshot, lockSnapshot)
			declaration, declarationErr := NewDeclarationEditResult(false, joinedErr, rollback)
			if declarationErr != nil {
				return RemoveModuleDependencyResult{}, declarationErr
			}
			declarations = append(declarations, declaration)
			return NewRemoveModuleDependencyResult(removed, lockWritten, declarations)
		}
		declaration, declarationErr := NewDeclarationEditResult(written, nil, DeclarationRollbackNone)
		if declarationErr != nil {
			return RemoveModuleDependencyResult{}, declarationErr
		}
		declarations = append(declarations, declaration)
	}

	return NewRemoveModuleDependencyResult(removed, lockWritten, declarations)
}

// rollBackDeclarationEdit restores invowkmod.cue and then the lock file after
// a failed invowkmod.cue edit. The outcome is complete when both restores
// succeeded and incomplete otherwise; joined is the edit error joined with the
// restore errors, for display.
func rollBackDeclarationEdit(editErr error, declarationSnapshot, lockSnapshot fileSnapshot) (rollback DeclarationRollback, joined error) {
	restoreErr := errors.Join(declarationSnapshot.restore(), lockSnapshot.restore())
	if restoreErr != nil {
		return DeclarationRollbackIncomplete, errors.Join(editErr, restoreErr)
	}
	return DeclarationRollbackComplete, editErr
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
	if s.kind != snapshotSymlink && s.targetPath != nil {
		return errors.New("file snapshot target path is only valid for symlink snapshots")
	}
	if s.kind == snapshotSymlink {
		if s.linkTarget == nil {
			return errors.New("file snapshot link target is required")
		}
		if err := s.linkTarget.Validate(); err != nil {
			return fmt.Errorf("file snapshot link target: %w", err)
		}
		if s.targetPath != nil {
			if err := s.targetPath.Validate(); err != nil {
				return fmt.Errorf("file snapshot target path: %w", err)
			}
		}
	}
	return s.path.Validate()
}

func (m *Resolver) readLockSnapshot() (fileSnapshot, error) {
	lockPath := types.FilesystemPath(filepath.Join(string(m.workingDir), LockFileName))
	if err := lockPath.Validate(); err != nil {
		return fileSnapshot{}, fmt.Errorf("validate lock file path: %w", err)
	}
	snapshot, err := m.readFileSnapshot(lockPath)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("snapshot lock file before dependency edit: %w", err)
	}
	return snapshot, nil
}

// readFileSnapshot takes the snapshot of path and carries the resolver's
// restore seams into it.
func (m *Resolver) readFileSnapshot(path types.FilesystemPath) (fileSnapshot, error) {
	snapshot, err := readFileSnapshot(path)
	if err != nil {
		return fileSnapshot{}, err
	}
	snapshot.writeFile = m.restoreWriter
	snapshot.createLink = m.restoreLinker
	return snapshot, nil
}

// readFileSnapshot records what is at path without following a symbolic link
// for the entry itself: a dangling link is a link-only snapshot, and a link
// that resolves to an existing regular file snapshots both the link target and
// the resolved target file's path, bytes, and permissions (MA5 Option A1).
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
		linkTarget, readErr := readSymlinkTargetPath(path)
		if readErr != nil {
			return fileSnapshot{}, readErr
		}
		snapshot.kind = snapshotSymlink
		snapshot.linkTarget = &linkTarget
		if targetErr := populateResolvedSymlinkTarget(path, &snapshot); targetErr != nil {
			return fileSnapshot{}, targetErr
		}
	case snapshotsBytes(info.Mode(), runtime.GOOS):
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

func readSymlinkTargetPath(path types.FilesystemPath) (types.FilesystemPath, error) {
	target, err := os.Readlink(string(path))
	if err != nil {
		return "", fmt.Errorf("read link before dependency edit: %w", err)
	}
	linkTarget := types.FilesystemPath(target)
	if err := linkTarget.Validate(); err != nil {
		return "", fmt.Errorf("invalid link target before dependency edit: %w", err)
	}
	return linkTarget, nil
}

func populateResolvedSymlinkTarget(path types.FilesystemPath, snapshot *fileSnapshot) error {
	if resolvedTarget, inspectErr := fspath.InspectWritableTarget(string(path)); inspectErr == nil && resolvedTarget.Exists {
		resolvedFSPath, targetData, err := readResolvedSymlinkTarget(resolvedTarget)
		if err != nil {
			return err
		}
		snapshot.targetPath = &resolvedFSPath
		snapshot.data = targetData
		snapshot.mode = resolvedTarget.Perm
	}
	return nil
}

func readResolvedSymlinkTarget(target fspath.FileTarget) (types.FilesystemPath, fileSnapshotData, error) {
	targetData, err := os.ReadFile(target.Path)
	if err != nil {
		return "", nil, fmt.Errorf("read symlink target before dependency edit: %w", err)
	}
	resolvedFSPath := types.FilesystemPath(target.Path)
	if err := resolvedFSPath.Validate(); err != nil {
		return "", nil, fmt.Errorf("invalid resolved symlink target before dependency edit: %w", err)
	}
	return resolvedFSPath, fileSnapshotData(targetData), nil
}

// snapshotsBytes reports whether a file of mode is snapshotted by its bytes:
// a regular file, and on Windows also a non-directory that os.Lstat reports
// as ModeIrregular. Go 1.23+ gives that mode to every reparse point other
// than a symbolic link, a Unix socket or a deduplicated file, which includes
// ordinary data files such as cloud-file placeholders (OneDrive) that
// os.Stat, the base's snapshot, read as regular; leaving them as
// snapshotOther would let a rollback report complete without restoring them.
//
//goplint:ignore -- goos is runtime.GOOS, a parameter only so tests cover every platform.
func snapshotsBytes(mode fs.FileMode, goos string) bool {
	if mode.IsRegular() {
		return true
	}
	return goos == "windows" && mode.Type() == fs.ModeIrregular
}

// restore puts the snapshot's path back as it was: a missing path is removed,
// a regular file is replaced atomically (temporary file and rename) with its
// bytes and permission bits, so a concurrent reader sees the whole old or the
// whole new file and never a truncated one (finding F10), a symbolic link is
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
		if err := s.fileWriter()(path, []byte(s.data), s.mode.Perm()); err != nil {
			return fmt.Errorf("rollback %s: %w", *s.path, err)
		}
		return nil
	case snapshotSymlink:
		var targetErr error
		if s.targetPath != nil {
			if err := s.fileWriter()(string(*s.targetPath), []byte(s.data), s.mode.Perm()); err != nil {
				targetErr = fmt.Errorf("rollback target %s: %w", *s.targetPath, err)
			}
		}
		linkErr := s.restoreLink()
		return errors.Join(targetErr, linkErr)
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
	if err := s.linkCreator()(string(*s.linkTarget), tmp); err != nil {
		return fmt.Errorf("rollback link %s: %w", *s.path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return errors.Join(fmt.Errorf("rollback link %s: %w", *s.path, err), os.Remove(tmp))
	}
	return nil
}

func (s fileSnapshot) fileWriter() restoreFileWriter {
	if s.writeFile != nil {
		return s.writeFile
	}
	return fspath.AtomicWriteFile
}

func (s fileSnapshot) linkCreator() restoreLinkCreator {
	if s.createLink != nil {
		return s.createLink
	}
	return os.Symlink
}

func newResolverForInvowkmodPath(invowkmodPath types.FilesystemPath, locks EditLocks) (*Resolver, error) {
	workingDir := types.FilesystemPath(filepath.Dir(string(invowkmodPath)))
	if err := workingDir.Validate(); err != nil {
		return nil, fmt.Errorf("invalid module working directory: %w", err)
	}
	return NewResolver(workingDir, "", WithEditLocks(locks))
}

// joinUnlock releases a lock taken by a use case and joins the release error
// into the use case's error.
func joinUnlock(err error, unlock func() error) error {
	return errors.Join(err, unlock())
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
// the module lock file, under the project lock of locks. With no requirements
// it saves nothing and the result reports LockWritten() == false.
func SyncModule(ctx context.Context, locks EditLocks, invowkmodPath types.FilesystemPath) (SyncModuleResult, error) {
	if err := locks.Validate(); err != nil {
		return SyncModuleResult{}, err
	}
	resolver, err := newResolverForInvowkmodPath(invowkmodPath, locks)
	if err != nil {
		return SyncModuleResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.SyncModule(ctx, invowkmodPath)
}

// SyncModule resolves the requirements declared in invowkmodPath and updates
// the lock file. It takes the project lock before it reads invowkmod.cue and
// releases it after the lock save, or after finding no requirements (then it
// saves nothing and reports LockWritten() == false).
func (m *Resolver) SyncModule(ctx context.Context, invowkmodPath types.FilesystemPath) (result SyncModuleResult, err error) {
	unlock, err := m.LockProject(ctx)
	if err != nil {
		return SyncModuleResult{}, err
	}
	defer func() { err = joinUnlock(err, unlock) }()

	requirements, loadErr := LoadRequirements(invowkmodPath)
	if loadErr != nil {
		return SyncModuleResult{}, loadErr
	}
	if len(requirements) == 0 {
		return NewSyncModuleResult(requirements, nil, false)
	}
	resolved, lockWritten, syncErr := m.Sync(ctx, requirements)
	if syncErr != nil {
		return SyncModuleResult{}, syncErr
	}
	return NewSyncModuleResult(requirements, resolved, lockWritten)
}

// TidyModule resolves declared requirements, adds missing transitive
// requirements to invowkmodPath, and returns the added refs, under the
// project lock of locks.
func TidyModule(ctx context.Context, locks EditLocks, invowkmodPath types.FilesystemPath) (requirements, missing []ModuleRef, err error) {
	if lockErr := locks.Validate(); lockErr != nil {
		return nil, nil, lockErr
	}
	resolver, err := newResolverForInvowkmodPath(invowkmodPath, locks)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.TidyModule(ctx, invowkmodPath)
}

// TidyModule resolves the requirements declared in invowkmodPath and adds the
// missing transitive requirements to it. It takes the project lock before it
// reads invowkmod.cue and releases it after the last AddRequirement.
func (m *Resolver) TidyModule(ctx context.Context, invowkmodPath types.FilesystemPath) (requirements, missing []ModuleRef, err error) {
	unlock, err := m.LockProject(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = joinUnlock(err, unlock) }()

	requirements, err = LoadRequirements(invowkmodPath)
	if err != nil {
		return nil, nil, err
	}
	if len(requirements) == 0 {
		return requirements, nil, nil
	}
	missing, err = m.Tidy(ctx, requirements)
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

// UpdateModule updates resolved module dependencies in the lock file next to
// invowkmodPath, under the project lock of locks.
//
//goplint:ignore -- module update identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func UpdateModule(ctx context.Context, locks EditLocks, invowkmodPath types.FilesystemPath, identifier string) (UpdateModuleResult, error) {
	if err := locks.Validate(); err != nil {
		return UpdateModuleResult{}, err
	}
	resolver, err := newResolverForInvowkmodPath(invowkmodPath, locks)
	if err != nil {
		return UpdateModuleResult{}, fmt.Errorf(errFmtCreateModuleResolver, err)
	}
	return resolver.UpdateModule(ctx, identifier)
}

// UpdateModule updates the resolved module dependencies in this resolver's
// lock file. It takes the project lock before Update reads the lock file and
// releases it after the lock save.
//
//goplint:ignore -- module update identifier is a CLI-facing selector: URL, namespace, module name, or lock key.
func (m *Resolver) UpdateModule(ctx context.Context, identifier string) (result UpdateModuleResult, err error) {
	unlock, err := m.LockProject(ctx)
	if err != nil {
		return UpdateModuleResult{}, err
	}
	defer func() { err = joinUnlock(err, unlock) }()

	updated, lockWritten, updateErr := m.Update(ctx, identifier)
	if updateErr != nil {
		return UpdateModuleResult{}, updateErr
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

func newLockOnlyResolverForInvowkmodPath(invowkmodPath types.FilesystemPath, opts ...ResolverOption) (*Resolver, error) {
	workingDir := types.FilesystemPath(filepath.Dir(string(invowkmodPath)))
	if err := workingDir.Validate(); err != nil {
		return nil, fmt.Errorf("invalid module working directory: %w", err)
	}
	return newLockOnlyResolver(workingDir, opts...)
}
