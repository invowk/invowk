// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

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

// Validate returns nil if the LockFileVersion is valid (non-empty),
// or an error describing the validation failure.
func (v LockFileVersion) Validate() error {
	if v == "" {
		return &InvalidLockFileVersionError{Value: v}
	}
	return nil
}

// Error implements the error interface for InvalidLockFileVersionError.
func (e *InvalidLockFileVersionError) Error() string {
	return fmt.Sprintf("invalid lock file version %q: must be non-empty", e.Value)
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
// Delegates to Validate() on all 7 typed fields.
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

// LoadLockFile loads a lock file from the given path.
func LoadLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewLockFile(), nil
		}
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	return parseLockFileCUE(string(data))
}

// Save writes the lock file to disk in CUE format.
// Uses atomicWriteFile (temp file + rename) for crash safety.
func (l *LockFile) Save(path string) error {
	content := l.toCUE()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return atomicWriteFile(path, []byte(content))
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
	for key := range l.Modules {
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
		fmt.Fprintf(&sb, "\t\tcontent_hash:     %q\n", mod.ContentHash)
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// parseLockFileCUE parses a CUE-format lock file.
// This is a simplified parser for the lock file format.
func parseLockFileCUE(content string) (*LockFile, error) {
	lock := NewLockFile()

	// Parse line by line (simplified parser)
	lines := strings.Split(content, "\n")
	var currentModuleKey ModuleRefKey
	var currentModule LockedModule
	inModules := false
	braceDepth := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		// Top-level fields are parsed only outside the modules block.
		// Without this guard, module-level `version:` fields would be consumed
		// by the top-level parser (the field names collide).
		if !inModules {
			// Parse version
			if strings.HasPrefix(line, "version:") {
				lock.Version = LockFileVersion(parseStringValue(line))
				if err := lock.Version.Validate(); err != nil {
					return nil, fmt.Errorf("lock file version: %w", err)
				}
				continue
			}

			// Parse generated timestamp
			if strings.HasPrefix(line, "generated:") {
				if t, err := time.Parse(time.RFC3339, parseStringValue(line)); err == nil {
					lock.Generated = t
				}
				continue
			}
		}

		// Track modules block — fall through to process any { on this line
		if strings.HasPrefix(line, "modules:") {
			inModules = true
		}

		if !inModules {
			continue
		}

		// Track brace depth
		if strings.Contains(line, "{") {
			braceDepth++
			// Check if this is a new module entry
			if braceDepth == 2 && strings.Contains(line, ":") {
				currentModuleKey = parseModuleKey(line)
				currentModule = LockedModule{}
			}
		}
		if strings.Contains(line, "}") {
			if braceDepth == 2 && currentModuleKey != "" {
				if err := currentModuleKey.Validate(); err != nil {
					return nil, fmt.Errorf("lock file module key: %w", err)
				}
				if err := currentModule.Validate(); err != nil {
					// Attach the module key to the error for debugging context.
					if lockedErr, ok := errors.AsType[*InvalidLockedModuleError](err); ok {
						lockedErr.ModuleKey = currentModuleKey
					}
					return nil, fmt.Errorf("lock file module %q: %w", currentModuleKey, err)
				}
				lock.Modules[currentModuleKey] = currentModule
				currentModuleKey = ""
			}
			braceDepth--
			if braceDepth == 0 {
				inModules = false
			}
		}

		// Parse module fields — field-level casts are validated by struct-level checks.
		if braceDepth == 2 && currentModuleKey != "" {
			switch {
			case strings.HasPrefix(line, "git_url:"):
				currentModule.GitURL = GitURL(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "version:"):
				currentModule.Version = SemVerConstraint(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "resolved_version:"):
				currentModule.ResolvedVersion = SemVer(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "git_commit:"):
				currentModule.GitCommit = GitCommit(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "alias:"):
				currentModule.Alias = ModuleAlias(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "path:"):
				currentModule.Path = SubdirectoryPath(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "namespace:"):
				currentModule.Namespace = ModuleNamespace(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			case strings.HasPrefix(line, "content_hash:"):
				currentModule.ContentHash = ContentHash(parseStringValue(line)) //goplint:ignore -- validated by LockedModule.Validate()
			}
		}
	}

	return lock, nil
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
