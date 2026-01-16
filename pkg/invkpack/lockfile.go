// SPDX-License-Identifier: EPL-2.0

package invkpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LockFile represents the invkpack.lock.cue file structure.
type LockFile struct {
	// Version is the lock file format version.
	Version string

	// Generated is the timestamp when the lock file was generated.
	Generated time.Time

	// Packs maps pack keys to their locked versions.
	Packs map[string]LockedPack
}

// LockedPack represents a locked pack entry in the lock file.
type LockedPack struct {
	// GitURL is the Git repository URL.
	GitURL string

	// Version is the original version constraint from the invkfile.
	Version string

	// ResolvedVersion is the exact version that was resolved.
	ResolvedVersion string

	// GitCommit is the Git commit SHA for the resolved version.
	GitCommit string

	// Alias is the namespace alias (optional).
	Alias string

	// Path is the subdirectory path within the repository (optional).
	Path string

	// Namespace is the computed namespace for commands.
	Namespace string
}

// NewLockFile creates a new empty lock file.
func NewLockFile() *LockFile {
	return &LockFile{
		Version:   "1.0",
		Generated: time.Now(),
		Packs:     make(map[string]LockedPack),
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
func (l *LockFile) Save(path string) error {
	content := l.toCUE()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write atomically using temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best-effort cleanup of temp file
		return fmt.Errorf("failed to rename lock file: %w", err)
	}

	return nil
}

// toCUE serializes the lock file to CUE format.
func (l *LockFile) toCUE() string {
	var sb strings.Builder

	sb.WriteString("// invkpack.lock.cue - Auto-generated lock file for pack dependencies\n")
	sb.WriteString("// DO NOT EDIT MANUALLY\n\n")

	sb.WriteString(fmt.Sprintf("version: %q\n", l.Version))
	sb.WriteString(fmt.Sprintf("generated: %q\n\n", l.Generated.Format(time.RFC3339)))

	if len(l.Packs) == 0 {
		sb.WriteString("packs: {}\n")
		return sb.String()
	}

	sb.WriteString("packs: {\n")
	for key, pack := range l.Packs {
		sb.WriteString(fmt.Sprintf("\t%q: {\n", key))
		sb.WriteString(fmt.Sprintf("\t\tgit_url:          %q\n", pack.GitURL))
		sb.WriteString(fmt.Sprintf("\t\tversion:          %q\n", pack.Version))
		sb.WriteString(fmt.Sprintf("\t\tresolved_version: %q\n", pack.ResolvedVersion))
		sb.WriteString(fmt.Sprintf("\t\tgit_commit:       %q\n", pack.GitCommit))
		if pack.Alias != "" {
			sb.WriteString(fmt.Sprintf("\t\talias:            %q\n", pack.Alias))
		}
		if pack.Path != "" {
			sb.WriteString(fmt.Sprintf("\t\tpath:             %q\n", pack.Path))
		}
		sb.WriteString(fmt.Sprintf("\t\tnamespace:        %q\n", pack.Namespace))
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// parseLockFileCUE parses a CUE-format lock file.
// This is a simplified parser for the lock file format.
// It supports both the current "packs:" key and the legacy "modules:" key for migration.
func parseLockFileCUE(content string) (*LockFile, error) {
	lock := NewLockFile()

	// Parse line by line (simplified parser)
	lines := strings.Split(content, "\n")
	var currentPackKey string
	var currentPack LockedPack
	inPacks := false
	braceDepth := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		// Parse version
		if strings.HasPrefix(line, "version:") {
			lock.Version = parseStringValue(line)
			continue
		}

		// Parse generated timestamp
		if strings.HasPrefix(line, "generated:") {
			if t, err := time.Parse(time.RFC3339, parseStringValue(line)); err == nil {
				lock.Generated = t
			}
			continue
		}

		// Track packs block (supports both "packs:" and legacy "modules:" key)
		if strings.HasPrefix(line, "packs:") || strings.HasPrefix(line, "modules:") {
			inPacks = true
			continue
		}

		if !inPacks {
			continue
		}

		// Track brace depth
		if strings.Contains(line, "{") {
			braceDepth++
			// Check if this is a new pack entry
			if braceDepth == 2 && strings.Contains(line, ":") {
				currentPackKey = parsePackKey(line)
				currentPack = LockedPack{}
			}
		}
		if strings.Contains(line, "}") {
			if braceDepth == 2 && currentPackKey != "" {
				lock.Packs[currentPackKey] = currentPack
				currentPackKey = ""
			}
			braceDepth--
			if braceDepth == 0 {
				inPacks = false
			}
		}

		// Parse pack fields
		if braceDepth == 2 && currentPackKey != "" {
			if strings.HasPrefix(line, "git_url:") {
				currentPack.GitURL = parseStringValue(line)
			} else if strings.HasPrefix(line, "version:") {
				currentPack.Version = parseStringValue(line)
			} else if strings.HasPrefix(line, "resolved_version:") {
				currentPack.ResolvedVersion = parseStringValue(line)
			} else if strings.HasPrefix(line, "git_commit:") {
				currentPack.GitCommit = parseStringValue(line)
			} else if strings.HasPrefix(line, "alias:") {
				currentPack.Alias = parseStringValue(line)
			} else if strings.HasPrefix(line, "path:") {
				currentPack.Path = parseStringValue(line)
			} else if strings.HasPrefix(line, "namespace:") {
				currentPack.Namespace = parseStringValue(line)
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

// parsePackKey extracts the pack key from a CUE line like `"key": {`.
func parsePackKey(line string) string {
	line = strings.TrimSpace(line)
	// Format: "key": {
	if strings.HasPrefix(line, "\"") {
		end := strings.Index(line[1:], "\"")
		if end != -1 {
			return line[1 : end+1]
		}
	}
	return ""
}

// AddPack adds a resolved pack to the lock file.
func (l *LockFile) AddPack(resolved *ResolvedPack) {
	l.Packs[resolved.PackRef.Key()] = LockedPack{
		GitURL:          resolved.PackRef.GitURL,
		Version:         resolved.PackRef.Version,
		ResolvedVersion: resolved.ResolvedVersion,
		GitCommit:       resolved.GitCommit,
		Alias:           resolved.PackRef.Alias,
		Path:            resolved.PackRef.Path,
		Namespace:       resolved.Namespace,
	}
}

// HasPack checks if a pack is in the lock file.
func (l *LockFile) HasPack(key string) bool {
	_, ok := l.Packs[key]
	return ok
}

// GetPack returns a pack from the lock file.
func (l *LockFile) GetPack(key string) (LockedPack, bool) {
	pack, ok := l.Packs[key]
	return pack, ok
}
