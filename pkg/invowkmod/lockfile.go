// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

// maxLockFileSize is the maximum allowed lock file size (5 MiB).
// Matches the CUE file size guard in cueutil.CheckFileSize. Prevents DoS
// via crafted multi-GB lock files that would exhaust process memory (M-01).
const maxLockFileSize = 5 * 1024 * 1024

var (
	// ErrInvalidModuleNamespace is returned when a ModuleNamespace value is empty.
	ErrInvalidModuleNamespace = errors.New("invalid module namespace")
	// ErrInvalidLockFileVersion is the sentinel error wrapped by InvalidLockFileVersionError.
	ErrInvalidLockFileVersion = errors.New("invalid lock file version")
	// ErrInvalidModuleRefKey is returned when a ModuleRefKey value is empty.
	ErrInvalidModuleRefKey = errors.New("invalid module ref key")
	// ErrInvalidLockedModule is the sentinel error wrapped by InvalidLockedModuleError.
	ErrInvalidLockedModule = errors.New("invalid locked module")
)

type (
	// ModuleNamespace is the computed namespace for a module's commands.
	// Format: "<module>@<version>" or the alias if one is specified.
	// Must not be empty — it is always a computed value.
	ModuleNamespace string

	// InvalidModuleNamespaceError is returned when a ModuleNamespace value is empty.
	// It wraps ErrInvalidModuleNamespace for errors.Is() compatibility.
	InvalidModuleNamespaceError struct {
		Value ModuleNamespace
	}

	// LockFileVersion identifies the format version of a lock file.
	// Must be non-empty.
	LockFileVersion string

	// InvalidLockFileVersionError is returned when a LockFileVersion value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidLockFileVersion for errors.Is().
	InvalidLockFileVersionError struct {
		Value LockFileVersion
	}

	// ModuleRefKey is a typed key for the lock file's Modules map.
	// Format: "<git-url>" or "<git-url>#<subpath>" (e.g., "https://github.com/user/repo.git").
	// Must not be empty.
	ModuleRefKey string

	// InvalidModuleRefKeyError is returned when a ModuleRefKey value is empty.
	// DDD Value Type error struct — wraps ErrInvalidModuleRefKey for errors.Is().
	InvalidModuleRefKeyError struct {
		Value ModuleRefKey
	}

	//goplint:mutable
	//
	// LockFile represents the invowkmod.lock.cue file structure.
	LockFile struct {
		// Version is the lock file format version.
		Version LockFileVersion

		// Generated is the timestamp when the lock file was generated.
		Generated time.Time

		// Modules maps module ref keys to their locked versions.
		Modules map[ModuleRefKey]LockedModule
	}

	// InvalidLockedModuleError is returned when a LockedModule has invalid fields.
	// It wraps ErrInvalidLockedModule for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidLockedModuleError struct {
		ModuleKey   ModuleRefKey
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// LockedModule represents a locked module entry in the lock file.
	LockedModule struct {
		// GitURL is the Git repository URL.
		GitURL GitURL

		// Version is the original version constraint from the invowkfile.
		Version SemVerConstraint

		// ResolvedVersion is the exact version that was resolved.
		ResolvedVersion SemVer

		// GitCommit is the Git commit SHA for the resolved version.
		GitCommit GitCommit

		// Alias is the namespace alias (optional).
		Alias ModuleAlias

		// Path is the subdirectory path within the repository (optional).
		Path SubdirectoryPath

		// Namespace is the computed namespace for commands.
		Namespace ModuleNamespace

		// ModuleID is the resolved module identifier from invowkmod.cue.
		ModuleID ModuleID

		// ContentHash is the SHA-256 content hash of the cached module tree.
		// Used for tamper detection of vendored/cached modules.
		ContentHash ContentHash
	}
)

// Error implements the error interface for InvalidModuleNamespaceError.
func (e *InvalidModuleNamespaceError) Error() string {
	return fmt.Sprintf("invalid module namespace %q (must not be empty)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidModuleNamespaceError) Unwrap() error {
	return ErrInvalidModuleNamespace
}

//goplint:nonzero

// Validate returns nil if the ModuleNamespace is valid,
// or an error if it is empty. A valid namespace must not be empty —
// it is always a computed value.
func (n ModuleNamespace) Validate() error {
	if n == "" {
		return &InvalidModuleNamespaceError{Value: n}
	}
	return nil
}

// String returns the string representation of the ModuleNamespace.
func (n ModuleNamespace) String() string { return string(n) }

// String returns the string representation of the LockFileVersion.
func (v LockFileVersion) String() string { return string(v) }

// Validate returns nil if the LockFileVersion is a known format version,
// or an error if it is empty or unrecognized. The known versions allowlist
// is maintained in lockfile_parser.go alongside the parser registry.
func (v LockFileVersion) Validate() error {
	if v == "" {
		return &InvalidLockFileVersionError{Value: v}
	}
	if !IsKnownLockFileVersion(v) {
		return &InvalidLockFileVersionError{Value: v}
	}
	return nil
}

// Error implements the error interface for InvalidLockFileVersionError.
func (e *InvalidLockFileVersionError) Error() string {
	if e.Value == "" {
		return "invalid lock file version: must be non-empty"
	}
	known := make([]string, len(knownLockFileVersions))
	for i, v := range knownLockFileVersions {
		known[i] = string(v)
	}
	return fmt.Sprintf("invalid lock file version %q: must be one of [%s]", e.Value, strings.Join(known, ", "))
}

// Unwrap returns ErrInvalidLockFileVersion for errors.Is() compatibility.
func (e *InvalidLockFileVersionError) Unwrap() error { return ErrInvalidLockFileVersion }

// String returns the string representation of the ModuleRefKey.
func (k ModuleRefKey) String() string { return string(k) }

//goplint:nonzero

// Validate returns nil if the ModuleRefKey is valid (non-empty and not whitespace-only),
// or an error describing the validation failure.
func (k ModuleRefKey) Validate() error {
	if strings.TrimSpace(string(k)) == "" {
		return &InvalidModuleRefKeyError{Value: k}
	}
	return nil
}

// Error implements the error interface for InvalidModuleRefKeyError.
func (e *InvalidModuleRefKeyError) Error() string {
	return fmt.Sprintf("invalid module ref key %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidModuleRefKey for errors.Is() compatibility.
func (e *InvalidModuleRefKeyError) Unwrap() error { return ErrInvalidModuleRefKey }

// Validate returns nil if the LockedModule has valid fields,
// or an error collecting all field-level validation failures.
// Delegates to Validate() on all typed fields.
func (m LockedModule) Validate() error {
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

// Error implements the error interface for InvalidLockedModuleError.
func (e *InvalidLockedModuleError) Error() string {
	if e.ModuleKey != "" {
		return types.FormatFieldErrors(fmt.Sprintf("locked module %q", e.ModuleKey), e.FieldErrors)
	}
	return types.FormatFieldErrors("locked module", e.FieldErrors)
}

// Unwrap returns ErrInvalidLockedModule for errors.Is() compatibility.
func (e *InvalidLockedModuleError) Unwrap() error { return ErrInvalidLockedModule }

// NewLockFile creates a new empty lock file.
func NewLockFile() *LockFile {
	return &LockFile{
		Version:   "2.0",
		Generated: time.Now(),
		Modules:   make(map[ModuleRefKey]LockedModule),
	}
}

// ContentHashes returns a map of module ref keys to their content hashes.
// Used for cache tamper detection when re-resolving modules against a
// previously synced lock file. Entries without a content hash (e.g., v1.0
// lock file entries) are omitted.
func (l *LockFile) ContentHashes() map[ModuleRefKey]ContentHash {
	hashes := make(map[ModuleRefKey]ContentHash, len(l.Modules))
	for key := range l.Modules {
		if h := l.Modules[key].ContentHash; h != "" {
			hashes[key] = h
		}
	}
	return hashes
}

// RequireV2 returns ErrLockFileV1UpgradeRequired if this lock file uses the
// deprecated v1.0 format. v1.0 lacks content hashes, so mutating operations
// (Add, Remove, Update) must not proceed without an upgrade to v2.0 first.
// Returns nil for v2.0 lock files and for newly created (empty) lock files.
func (l *LockFile) RequireV2() error {
	if l.Version == LockFileVersionV1 {
		return fmt.Errorf(
			"%w: run 'invowk module sync' to upgrade to v2.0 for tamper detection",
			ErrLockFileV1UpgradeRequired,
		)
	}
	return nil
}

// LoadLockFile loads a lock file from the given path.
// Returns a new empty lock file if the path does not exist.
// Rejects files exceeding maxLockFileSize to prevent DoS (M-01).
func LoadLockFile(path string) (*LockFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewLockFile(), nil
		}
		return nil, fmt.Errorf("failed to stat lock file: %w", err)
	}
	if info.Size() > maxLockFileSize {
		return nil, fmt.Errorf("lock file exceeds maximum size (%d bytes > %d bytes)", info.Size(), maxLockFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	return parseLockFile(string(data))
}

// Save writes the lock file to disk in CUE format.
// Uses fspath.AtomicWriteFile (temp file + rename) for crash safety.
func (l *LockFile) Save(path string) error {
	content := l.toCUE()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return fspath.AtomicWriteFile(path, []byte(content), fspath.DefaultFilePerm)
}

// AddModule adds a resolved module to the lock file.
func (l *LockFile) AddModule(resolved *ResolvedModule) {
	l.Modules[resolved.ModuleRef.Key()] = LockedModule{
		GitURL:          resolved.ModuleRef.GitURL,
		Version:         resolved.ModuleRef.Version,
		ResolvedVersion: resolved.ResolvedVersion,
		GitCommit:       resolved.GitCommit,
		Alias:           resolved.ModuleRef.Alias,
		Path:            resolved.ModuleRef.Path,
		Namespace:       resolved.Namespace,
		ModuleID:        resolved.ModuleID,
		ContentHash:     resolved.ContentHash,
	}
}

// HasModule checks if a module is in the lock file.
func (l *LockFile) HasModule(key ModuleRefKey) bool {
	_, ok := l.Modules[key]
	return ok
}

// GetModule returns a module from the lock file.
func (l *LockFile) GetModule(key ModuleRefKey) (LockedModule, bool) {
	mod, ok := l.Modules[key]
	return mod, ok
}

// toCUE serializes the lock file to CUE format.
//
//plint:render
func (l *LockFile) toCUE() string {
	var sb strings.Builder

	sb.WriteString("// invowkmod.lock.cue - Auto-generated lock file for module dependencies\n")
	sb.WriteString("// DO NOT EDIT MANUALLY\n\n")

	fmt.Fprintf(&sb, "version: %q\n", l.Version)
	fmt.Fprintf(&sb, "generated: %q\n\n", l.Generated.Format(time.RFC3339))

	if len(l.Modules) == 0 {
		sb.WriteString("modules: {}\n")
		return sb.String()
	}

	sb.WriteString("modules: {\n")
	// Sort keys for deterministic output — prevents spurious VCS diffs when
	// the lock file is regenerated with identical logical content (L-03).
	keys := make([]ModuleRefKey, 0, len(l.Modules))
	for key := range l.Modules {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b ModuleRefKey) int {
		return cmp.Compare(string(a), string(b))
	})
	for _, key := range keys {
		mod := l.Modules[key]
		fmt.Fprintf(&sb, "\t%q: {\n", key)
		fmt.Fprintf(&sb, "\t\tgit_url:          %q\n", mod.GitURL)
		fmt.Fprintf(&sb, "\t\tversion:          %q\n", mod.Version)
		fmt.Fprintf(&sb, "\t\tresolved_version: %q\n", mod.ResolvedVersion)
		fmt.Fprintf(&sb, "\t\tgit_commit:       %q\n", mod.GitCommit)
		if mod.Alias != "" {
			fmt.Fprintf(&sb, "\t\talias:            %q\n", mod.Alias)
		}
		if mod.Path != "" {
			fmt.Fprintf(&sb, "\t\tpath:             %q\n", mod.Path)
		}
		fmt.Fprintf(&sb, "\t\tnamespace:        %q\n", mod.Namespace)
		if mod.ModuleID != "" {
			fmt.Fprintf(&sb, "\t\tmodule_id:        %q\n", mod.ModuleID)
		}
		fmt.Fprintf(&sb, "\t\tcontent_hash:     %q\n", mod.ContentHash)
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// parseStringValue extracts a quoted string value from a CUE line.
func parseStringValue(line string) string {
	_, value, found := strings.Cut(line, ":")
	if !found {
		return ""
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	return value
}

// parseModuleKey extracts the module key from a CUE line like `"key": {`.
func parseModuleKey(line string) ModuleRefKey {
	line = strings.TrimSpace(line)
	// Format: "key": {
	if strings.HasPrefix(line, "\"") {
		end := strings.Index(line[1:], "\"")
		if end != -1 {
			return ModuleRefKey(line[1 : end+1])
		}
	}
	return ""
}
