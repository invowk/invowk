// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const (
	// LockFileVersionV1 is the original lock file format version.
	LockFileVersionV1 LockFileVersion = "1.0"
	// LockFileVersionV2 is the current lock file format version (adds content_hash).
	LockFileVersionV2 LockFileVersion = "2.0"

	unknownLockFileVersionErrMsg    = "unknown lock file version"
	lockFileV1UpgradeRequiredErrMsg = "lock file uses deprecated v1.0 format"
)

var (
	// ErrUnknownLockFileVersion is returned when a lock file declares a version
	// that has no registered parser. Callers can use errors.Is() for programmatic detection.
	ErrUnknownLockFileVersion = errors.New(unknownLockFileVersionErrMsg)

	// ErrLockFileV1UpgradeRequired is returned when Add, Remove, or Update
	// encounters a v1.0 lock file that must be upgraded to v2.0 first.
	// v1.0 lacks content hashes, so modifying it without upgrading would
	// silently disable tamper detection (supply-chain risk).
	ErrLockFileV1UpgradeRequired = errors.New(lockFileV1UpgradeRequiredErrMsg)

	// knownLockFileVersions is the ordered list of recognized lock file format versions.
	// Used for validation and error messages.
	knownLockFileVersions = []LockFileVersion{LockFileVersionV1, LockFileVersionV2}
)

type (
	// UnknownLockFileVersionError is returned when a lock file declares a version
	// that has no registered parser.
	UnknownLockFileVersionError struct {
		Version LockFileVersion
		Known   []LockFileVersion
	}

	lockFileCUE struct {
		Version   LockFileVersion     `json:"version"`
		Generated lockFileGeneratedAt `json:"generated"`
		Modules   lockFileModules     `json:"modules"`
	}

	lockFileModules map[ModuleRefKey]lockedModuleCUE

	lockedModuleCUE struct {
		GitURL          GitURL           `json:"git_url"`
		Version         SemVerConstraint `json:"version"`
		ResolvedVersion SemVer           `json:"resolved_version"`
		GitCommit       GitCommit        `json:"git_commit"`
		Alias           ModuleAlias      `json:"alias"`
		Path            SubdirectoryPath `json:"path"`
		Namespace       ModuleNamespace  `json:"namespace"`
		CommandSourceID ModuleSourceID   `json:"command_source_id"`
		ModuleID        ModuleID         `json:"module_id"`
		ContentHash     ContentHash      `json:"content_hash"`
	}

	lockFileGeneratedAt string
	lockFileCUEContent  string
)

// Error implements the error interface.
func (e *UnknownLockFileVersionError) Error() string {
	known := make([]string, len(e.Known))
	for i, v := range e.Known {
		known[i] = string(v)
	}
	return fmt.Sprintf("unknown lock file version %q (known: %s)", e.Version, strings.Join(known, ", "))
}

// Unwrap returns ErrUnknownLockFileVersion for errors.Is() compatibility.
func (e *UnknownLockFileVersionError) Unwrap() error { return ErrUnknownLockFileVersion }

// IsKnownLockFileVersion reports whether the given version is supported.
func IsKnownLockFileVersion(v LockFileVersion) bool {
	return slices.Contains(knownLockFileVersions, v)
}

// parseLockFile parses invowkmod.lock.cue with the CUE parser/evaluator and
// then maps the decoded data into domain value types. CUE owns syntax,
// balancing, and concrete-value validation; domain validation owns lock-format
// versions and module field semantics.
func parseLockFile(content string) (*LockFile, error) {
	decoded, err := decodeLockFileCUE(lockFileCUEContent(content))
	if err != nil {
		return nil, err
	}

	version := decoded.Version
	if version == "" {
		return nil, &InvalidLockFileVersionError{Value: ""}
	}
	if !IsKnownLockFileVersion(version) {
		return nil, &UnknownLockFileVersionError{
			Version: version,
			Known:   knownLockFileVersions,
		}
	}

	if strings.TrimSpace(decoded.Generated.String()) == "" {
		return nil, errors.New("lock file generated: must be non-empty")
	}
	generated, err := time.Parse(time.RFC3339, decoded.Generated.String())
	if err != nil {
		return nil, fmt.Errorf("lock file generated: %w", err)
	}

	lock := &LockFile{
		Version:   version,
		Generated: generated,
		Modules:   make(map[ModuleRefKey]LockedModule, len(decoded.Modules)),
	}
	requireContentHash := version == LockFileVersionV2
	for key := range decoded.Modules {
		if err := key.Validate(); err != nil {
			return nil, fmt.Errorf("lock file module key: %w", err)
		}

		decodedModule := decoded.Modules[key]
		mod := lockedModuleFromCUE(decodedModule)
		if err := validateLockedModuleForVersion(key, mod, requireContentHash); err != nil {
			return nil, err
		}
		lock.Modules[key] = mod
	}

	return lock, nil
}

func decodeLockFileCUE(content lockFileCUEContent) (lockFileCUE, error) {
	ctx := cuecontext.New()
	value := ctx.CompileString(content.String(), cue.Filename(LockFileName))
	if err := value.Err(); err != nil {
		return lockFileCUE{}, fmt.Errorf("parse lock file CUE: %w", err)
	}
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return lockFileCUE{}, fmt.Errorf("validate lock file CUE: %w", err)
	}

	var decoded lockFileCUE
	if err := value.Decode(&decoded); err != nil {
		return lockFileCUE{}, fmt.Errorf("decode lock file CUE: %w", err)
	}
	if decoded.Modules == nil {
		decoded.Modules = lockFileModules{}
	}
	return decoded, nil
}

func lockedModuleFromCUE(module lockedModuleCUE) LockedModule {
	return LockedModule(module)
}

func (g lockFileGeneratedAt) String() string { return string(g) }

func (g lockFileGeneratedAt) Validate() error {
	if strings.TrimSpace(g.String()) == "" {
		return errors.New("lock file generated: must be non-empty")
	}
	if _, err := time.Parse(time.RFC3339, g.String()); err != nil {
		return fmt.Errorf("lock file generated: %w", err)
	}
	return nil
}

func (c lockFileCUEContent) String() string { return string(c) }

func (c lockFileCUEContent) Validate() error {
	if strings.TrimSpace(c.String()) == "" {
		return errors.New("lock file content: must be non-empty")
	}
	return nil
}

func (l lockFileCUE) Validate() error {
	if err := l.Version.Validate(); err != nil {
		return err
	}
	if err := l.Generated.Validate(); err != nil {
		return err
	}
	if err := l.Modules.Validate(); err != nil {
		return err
	}
	return nil
}

func (m lockFileModules) Validate() error {
	for key := range m {
		if err := key.Validate(); err != nil {
			return err
		}
		mod := m[key]
		if err := mod.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (m lockedModuleCUE) Validate() error {
	var errs []error
	if err := m.GitURL.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Version.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.ResolvedVersion.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.GitCommit.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Alias.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Namespace.Validate(); err != nil {
		errs = append(errs, err)
	}
	if m.CommandSourceID != "" {
		if err := m.CommandSourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.ModuleID != "" {
		if err := m.ModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := m.ContentHash.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidLockedModuleError{FieldErrors: errs}
	}
	return nil
}

// validateLockedModuleForVersion validates a parsed LockedModule, respecting
// version-specific field requirements. Delegates to LockedModule.Validate() as
// the single source of truth. For v1.0 lock files, ContentHash errors are
// filtered out because the field predates v2.0.
func validateLockedModuleForVersion(key ModuleRefKey, mod LockedModule, requireContentHash bool) error {
	err := mod.Validate()
	if err == nil {
		return nil
	}

	lockedErr, ok := errors.AsType[*InvalidLockedModuleError](err)
	if !ok {
		return fmt.Errorf("lock file module %q: %w", key, err)
	}

	lockedErr.ModuleKey = key

	// v1.0: filter out ContentHash errors (field not present in v1.0 format).
	if !requireContentHash {
		filtered := lockedErr.FieldErrors[:0]
		for _, fieldErr := range lockedErr.FieldErrors {
			if !errors.Is(fieldErr, ErrInvalidContentHash) {
				filtered = append(filtered, fieldErr)
			}
		}
		if len(filtered) == 0 {
			return nil
		}
		lockedErr.FieldErrors = filtered
	}

	return fmt.Errorf("lock file module %q: %w", key, lockedErr)
}
