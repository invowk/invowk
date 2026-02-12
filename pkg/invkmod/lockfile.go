// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type (
	// LockFile represents the invkmod.lock.cue file structure.
	LockFile struct {
		// Version is the lock file format version.
		Version string

		// Generated is the timestamp when the lock file was generated.
		Generated time.Time

		// Modules maps module keys to their locked versions.
		Modules map[string]LockedModule
	}

	// LockedModule represents a locked module entry in the lock file.
	LockedModule struct {
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
)

// NewLockFile creates a new empty lock file.
func NewLockFile() *LockFile {
	return &LockFile{
		Version:   "1.0",
		Generated: time.Now(),
		Modules:   make(map[string]LockedModule),
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write atomically using temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best-effort cleanup of temp file
		return fmt.Errorf("failed to rename lock file: %w", err)
	}

	return nil
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
	}
}

// HasModule checks if a module is in the lock file.
func (l *LockFile) HasModule(key string) bool {
	_, ok := l.Modules[key]
	return ok
}

// GetModule returns a module from the lock file.
func (l *LockFile) GetModule(key string) (LockedModule, bool) {
	mod, ok := l.Modules[key]
	return mod, ok
}

// toCUE serializes the lock file to CUE format.
func (l *LockFile) toCUE() string {
	var sb strings.Builder

	sb.WriteString("// invkmod.lock.cue - Auto-generated lock file for module dependencies\n")
	sb.WriteString("// DO NOT EDIT MANUALLY\n\n")

	sb.WriteString(fmt.Sprintf("version: %q\n", l.Version))
	sb.WriteString(fmt.Sprintf("generated: %q\n\n", l.Generated.Format(time.RFC3339)))

	if len(l.Modules) == 0 {
		sb.WriteString("modules: {}\n")
		return sb.String()
	}

	sb.WriteString("modules: {\n")
	for key, mod := range l.Modules {
		sb.WriteString(fmt.Sprintf("\t%q: {\n", key))
		sb.WriteString(fmt.Sprintf("\t\tgit_url:          %q\n", mod.GitURL))
		sb.WriteString(fmt.Sprintf("\t\tversion:          %q\n", mod.Version))
		sb.WriteString(fmt.Sprintf("\t\tresolved_version: %q\n", mod.ResolvedVersion))
		sb.WriteString(fmt.Sprintf("\t\tgit_commit:       %q\n", mod.GitCommit))
		if mod.Alias != "" {
			sb.WriteString(fmt.Sprintf("\t\talias:            %q\n", mod.Alias))
		}
		if mod.Path != "" {
			sb.WriteString(fmt.Sprintf("\t\tpath:             %q\n", mod.Path))
		}
		sb.WriteString(fmt.Sprintf("\t\tnamespace:        %q\n", mod.Namespace))
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
	var currentModuleKey string
	var currentModule LockedModule
	inModules := false
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
				lock.Modules[currentModuleKey] = currentModule
				currentModuleKey = ""
			}
			braceDepth--
			if braceDepth == 0 {
				inModules = false
			}
		}

		// Parse module fields
		if braceDepth == 2 && currentModuleKey != "" {
			switch {
			case strings.HasPrefix(line, "git_url:"):
				currentModule.GitURL = parseStringValue(line)
			case strings.HasPrefix(line, "version:"):
				currentModule.Version = parseStringValue(line)
			case strings.HasPrefix(line, "resolved_version:"):
				currentModule.ResolvedVersion = parseStringValue(line)
			case strings.HasPrefix(line, "git_commit:"):
				currentModule.GitCommit = parseStringValue(line)
			case strings.HasPrefix(line, "alias:"):
				currentModule.Alias = parseStringValue(line)
			case strings.HasPrefix(line, "path:"):
				currentModule.Path = parseStringValue(line)
			case strings.HasPrefix(line, "namespace:"):
				currentModule.Namespace = parseStringValue(line)
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
func parseModuleKey(line string) string {
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
