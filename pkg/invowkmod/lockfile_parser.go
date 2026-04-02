// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	// LockFileVersionV1 is the original lock file format version.
	LockFileVersionV1 LockFileVersion = "1.0"
	// LockFileVersionV2 is the current lock file format version (adds content_hash).
	LockFileVersionV2 LockFileVersion = "2.0"

	unknownLockFileVersionErrMsg = "unknown lock file version"
)

var (
	// ErrUnknownLockFileVersion is returned when a lock file declares a version
	// that has no registered parser. Callers can use errors.Is() for programmatic detection.
	ErrUnknownLockFileVersion = errors.New(unknownLockFileVersionErrMsg)

	// knownLockFileVersions is the ordered list of recognized lock file format versions.
	// Used for validation, error messages, and parser registration.
	knownLockFileVersions = []LockFileVersion{LockFileVersionV1, LockFileVersionV2}

	// lockFileParsers maps known versions to their parser implementations.
	lockFileParsers = map[LockFileVersion]LockFileParser{
		LockFileVersionV1: &lockFileParserV1V2{version: LockFileVersionV1, requireContentHash: false},
		LockFileVersionV2: &lockFileParserV1V2{version: LockFileVersionV2, requireContentHash: true},
	}
)

type (
	// UnknownLockFileVersionError is returned when a lock file declares a version
	// that has no registered parser.
	UnknownLockFileVersionError struct {
		Version LockFileVersion
		Known   []LockFileVersion
	}

	// LockFileParser parses lock file content for a specific format version.
	// Each version has its own implementation, enabling format evolution without
	// modifying existing parsers. The dispatcher extracts the version field first,
	// then delegates to the matching parser.
	LockFileParser interface {
		// ParseVersion returns the lock file format version this parser handles.
		ParseVersion() LockFileVersion
		// Parse parses the lock file content and returns a LockFile.
		Parse(content string) (*LockFile, error)
	}

	// lockFileParserV1V2 handles both v1.0 and v2.0 lock file formats.
	// The two formats share the same line-by-line CUE parser; the only difference
	// is that v2.0 requires the content_hash field while v1.0 does not (it was
	// introduced in v2.0). The requireContentHash flag controls validation.
	lockFileParserV1V2 struct {
		version            LockFileVersion
		requireContentHash bool
	}
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

// IsKnownLockFileVersion reports whether the given version has a registered parser.
func IsKnownLockFileVersion(v LockFileVersion) bool {
	_, ok := lockFileParsers[v]
	return ok
}

// parseLockFile extracts the version from the content, selects the matching
// parser, and delegates parsing. Unknown versions fail with an actionable error
// listing the known versions.
func parseLockFile(content string) (*LockFile, error) {
	version := extractLockFileVersion(content)
	if version == "" {
		return nil, &InvalidLockFileVersionError{Value: ""}
	}

	parser, ok := lockFileParsers[version]
	if !ok {
		return nil, &UnknownLockFileVersionError{
			Version: version,
			Known:   knownLockFileVersions,
		}
	}

	lock, err := parser.Parse(content)
	if err != nil {
		return nil, err
	}

	// Emit deprecation warning for v1 lock files. v1 lacks content_hash
	// fields, so tamper detection is not available. A downgrade from v2→v1
	// silently disables hash enforcement (supply-chain risk).
	if lock.Version == LockFileVersionV1 {
		slog.Warn("lock file uses deprecated v1.0 format without content hashes; "+
			"run 'invowk module sync' to upgrade to v2.0 for tamper detection",
			"version", string(lock.Version))
	}

	return lock, nil
}

// extractLockFileVersion reads the version field from lock file content
// without fully parsing it. This lightweight pre-parse avoids committing
// to a parser before the version is known.
func extractLockFileVersion(content string) LockFileVersion {
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "version:") {
			return LockFileVersion(parseStringValue(trimmed)) //goplint:ignore -- validated by parseLockFile dispatcher
		}
		// version: must be the first non-comment, non-empty field.
		// If we hit a different field first, the file has no version.
		if strings.Contains(trimmed, ":") {
			return ""
		}
	}
	return ""
}

// ParseVersion returns the version this parser handles.
func (p *lockFileParserV1V2) ParseVersion() LockFileVersion {
	return p.version
}

// Parse parses v1.0 or v2.0 lock file content.
func (p *lockFileParserV1V2) Parse(content string) (*LockFile, error) {
	lock := NewLockFile()

	lines := strings.Split(content, "\n")
	var currentModuleKey ModuleRefKey
	var currentModule LockedModule
	inModules := false
	braceDepth := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		// Top-level fields are parsed only outside the modules block.
		// Without this guard, module-level `version:` fields would be consumed
		// by the top-level parser (the field names collide).
		if !inModules {
			if strings.HasPrefix(line, "version:") {
				lock.Version = LockFileVersion(parseStringValue(line))
				if err := lock.Version.Validate(); err != nil {
					return nil, fmt.Errorf("lock file version: %w", err)
				}
				continue
			}

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
				if err := p.validateModule(currentModuleKey, currentModule); err != nil {
					return nil, err
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

// validateModule validates a parsed LockedModule, respecting version-specific
// field requirements. Delegates to LockedModule.Validate() as the single source
// of truth. For v1.0 lock files (which predate content_hash), ContentHash
// errors are filtered out.
func (p *lockFileParserV1V2) validateModule(key ModuleRefKey, mod LockedModule) error {
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
	if !p.requireContentHash {
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
