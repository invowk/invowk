// SPDX-License-Identifier: EPL-2.0

// Package invkmod provides module operations: validation, creation, archiving, and dependency management.
// Types and data structures are defined in invkmod.go.
package invkmod

import (
	"archive/zip"
	"context"
	"fmt"
	"invowk-cli/internal/platform"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// moduleNameRegex validates the module folder name prefix (before .invkmod)
// Must start with a letter, contain only alphanumeric chars, with optional dot-separated segments
// Compatible with RDNS naming (e.g., "com.example.mycommands")
var moduleNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)

// IsModule checks if the given path is a valid invowk module directory.
// This is a quick check that only verifies the folder name format and existence.
// For full validation, use Validate().
func IsModule(path string) bool {
	// Check if the path ends with .invkmod
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ModuleSuffix) {
		return false
	}

	// Check if the prefix is valid
	prefix := strings.TrimSuffix(base, ModuleSuffix)
	if !moduleNameRegex.MatchString(prefix) {
		return false
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ParseModuleName extracts and validates the module name from a folder name.
// The folder name must end with .invkmod and have a valid prefix.
// Returns the module name (without suffix) or an error if invalid.
func ParseModuleName(folderName string) (string, error) {
	// Must end with .invkmod
	if !strings.HasSuffix(folderName, ModuleSuffix) {
		return "", fmt.Errorf("folder name must end with '%s'", ModuleSuffix)
	}

	// Extract prefix
	prefix := strings.TrimSuffix(folderName, ModuleSuffix)
	if prefix == "" {
		return "", fmt.Errorf("module name cannot be empty (folder name cannot be just '%s')", ModuleSuffix)
	}

	// Must not start with a dot (hidden folder)
	if strings.HasPrefix(prefix, ".") {
		return "", fmt.Errorf("module name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	if !moduleNameRegex.MatchString(prefix) {
		return "", fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	return prefix, nil
}

// Validate performs comprehensive validation of a module at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(modulePath string) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:      true,
		ModulePath: absPath,
		Issues:     []ValidationIssue{},
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue("structure", "path does not exist", "")
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		result.AddIssue("structure", "path is not a directory", "")
		return result, nil
	}

	// Validate folder name and extract module name
	base := filepath.Base(absPath)
	moduleName, err := ParseModuleName(base)
	if err != nil {
		result.AddIssue("naming", err.Error(), "")
	} else {
		result.ModuleName = moduleName
	}

	// Check for invkmod.cue (required)
	invkmodPath := filepath.Join(absPath, "invkmod.cue")
	invkmodInfo, err := os.Stat(invkmodPath)
	switch {
	case err != nil && os.IsNotExist(err):
		result.AddIssue("structure", "missing required invkmod.cue", "")
	case err != nil:
		result.AddIssue("structure", fmt.Sprintf("cannot access invkmod.cue: %v", err), "")
	case invkmodInfo.IsDir():
		result.AddIssue("structure", "invkmod.cue must be a file, not a directory", "")
	default:
		result.InvkmodPath = invkmodPath

		// Parse invkmod.cue and validate module field matches folder name
		if result.ModuleName != "" {
			meta, parseErr := ParseInvkmod(invkmodPath)
			if parseErr != nil {
				result.AddIssue("invkmod", fmt.Sprintf("failed to parse invkmod.cue: %v", parseErr), "invkmod.cue")
			} else if meta.Module != result.ModuleName {
				result.AddIssue("naming", fmt.Sprintf(
					"module field '%s' in invkmod.cue must match folder name '%s'",
					meta.Module, result.ModuleName), "invkmod.cue")
			}
		}
	}

	// Check for invkfile.cue (optional - may be a library-only module)
	invkfilePath := filepath.Join(absPath, "invkfile.cue")
	invkfileInfo, err := os.Stat(invkfilePath)
	switch {
	case err != nil && os.IsNotExist(err):
		// Library-only module - no commands
		result.IsLibraryOnly = true
	case err != nil:
		result.AddIssue("structure", fmt.Sprintf("cannot access invkfile.cue: %v", err), "")
	case invkfileInfo.IsDir():
		result.AddIssue("structure", "invkfile.cue must be a file, not a directory", "")
	default:
		result.InvkfilePath = invkfilePath
	}

	// Check for nested modules and symlinks (security)
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // Intentionally skip errors to continue walking
		}

		// Skip the root directory itself
		if path == absPath {
			return nil
		}

		// Skip the vendored modules directory (invk_modules/) - nested modules are allowed there
		if d.IsDir() && d.Name() == VendoredModulesDir {
			return filepath.SkipDir
		}

		// Check for symlinks (security issue - could point outside module)
		if d.Type()&os.ModeSymlink != 0 {
			relPath, _ := filepath.Rel(absPath, path)
			// Check if symlink points outside the module
			linkTarget, readErr := os.Readlink(path)
			if readErr != nil {
				result.AddIssue("security", "cannot read symlink target", relPath)
			} else {
				// Resolve the symlink target relative to its location
				var resolvedTarget string
				if filepath.IsAbs(linkTarget) {
					resolvedTarget = linkTarget
				} else {
					resolvedTarget = filepath.Join(filepath.Dir(path), linkTarget)
				}
				// Clean and resolve to check if it escapes
				resolvedTarget = filepath.Clean(resolvedTarget)
				relToRoot, relErr := filepath.Rel(absPath, resolvedTarget)
				if relErr != nil || strings.HasPrefix(relToRoot, "..") {
					result.AddIssue("security", fmt.Sprintf("symlink points outside module directory (target: %s)", linkTarget), relPath)
				} else {
					// Even internal symlinks are a potential security concern during archive extraction
					result.AddIssue("security", "symlinks are not allowed in modules (security risk during extraction)", relPath)
				}
			}
		}

		// Check if any other subdirectory is a module
		if d.IsDir() && strings.HasSuffix(d.Name(), ModuleSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("structure", "nested modules are not allowed (except in invk_modules/)", relPath)
		}

		// Check for Windows reserved filenames (cross-platform compatibility)
		if platform.IsWindowsReservedName(d.Name()) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("compatibility", fmt.Sprintf("filename '%s' is reserved on Windows", d.Name()), relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk module directory: %w", err)
	}

	return result, nil
}

// Load loads and validates a module at the given path.
// Returns a Module (operational wrapper) if valid, or an error with validation details.
// Note: This loads only metadata (invkmod.cue), not commands (invkfile.cue).
// To load commands as well, use pkg/invkfile.ParseModule().
func Load(modulePath string) (*Module, error) {
	result, err := Validate(modulePath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		return nil, fmt.Errorf("invalid module: %s", strings.Join(msgs, "; "))
	}

	// Parse the metadata
	var metadata *Invkmod
	if result.InvkmodPath != "" {
		metadata, err = ParseInvkmod(result.InvkmodPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse module metadata: %w", err)
		}
	}

	return &Module{
		Metadata:      metadata,
		Path:          result.ModulePath,
		IsLibraryOnly: result.IsLibraryOnly,
	}, nil
}

// CreateOptions contains options for creating a new module
type CreateOptions struct {
	// Name is the module name (e.g., "com.example.mytools")
	Name string
	// ParentDir is the directory where the module will be created
	ParentDir string
	// Module is the module identifier for the invkfile (defaults to Name if empty)
	Module string
	// Description is an optional description for the invkfile
	Description string
	// CreateScriptsDir creates a scripts/ subdirectory if true
	CreateScriptsDir bool
}

// Create creates a new module with the given options.
// Returns the path to the created module or an error.
func Create(opts CreateOptions) (string, error) {
	// Validate module name
	if opts.Name == "" {
		return "", fmt.Errorf("module name cannot be empty")
	}

	// Validate the name format
	if !moduleNameRegex.MatchString(opts.Name) {
		return "", fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", opts.Name)
	}

	// Default parent directory to current directory
	parentDir := opts.ParentDir
	if parentDir == "" {
		var err error
		parentDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Resolve absolute path
	absParentDir, err := filepath.Abs(parentDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent directory: %w", err)
	}

	// Create module directory
	moduleDirName := opts.Name + ModuleSuffix
	modulePath := filepath.Join(absParentDir, moduleDirName)

	// Check if module already exists
	if _, err := os.Stat(modulePath); err == nil {
		return "", fmt.Errorf("module already exists at %s", modulePath)
	}

	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create module directory: %w", err)
	}

	// Use name as module identifier if not specified
	moduleID := opts.Module
	if moduleID == "" {
		moduleID = opts.Name
	}

	// Create description
	description := opts.Description
	if description == "" {
		description = fmt.Sprintf("Commands from %s module", opts.Name)
	}

	// Create invkmod.cue (module metadata)
	invkmodContent := fmt.Sprintf(`// Invkmod - Module metadata for %s
// See https://github.com/invowk/invowk for documentation

module: %q
version: "1.0"
description: %q

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invkmod.git"
//         version: "^1.0.0"
//     },
// ]
`, opts.Name, moduleID, description)

	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	if err := os.WriteFile(invkmodPath, []byte(invkmodContent), 0o644); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
		return "", fmt.Errorf("failed to create invkmod.cue: %w", err)
	}

	// Create invkfile.cue (command definitions only)
	invkfileContent := fmt.Sprintf(`// Invkfile - Command definitions for %s module
// See https://github.com/invowk/invowk for documentation

cmds: [
	{
		name:        "hello"
		description: "A sample command"
		implementations: [
			{
				script: "echo \"Hello from %s!\""
				runtimes: [
					{name: "native"},
					{name: "virtual"},
				]
			},
		]
	},
]
`, opts.Name, opts.Name)

	invkfilePath := filepath.Join(modulePath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0o644); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
		return "", fmt.Errorf("failed to create invkfile.cue: %w", err)
	}

	// Optionally create scripts directory
	if opts.CreateScriptsDir {
		scriptsDir := filepath.Join(modulePath, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			// Clean up on failure
			_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
			return "", fmt.Errorf("failed to create scripts directory: %w", err)
		}

		// Create a placeholder .gitkeep file
		gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), 0o644); err != nil {
			// Clean up on failure
			_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
			return "", fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	return modulePath, nil
}

// Archive creates a ZIP archive of a module.
// Returns the path to the created ZIP file or an error.
func Archive(modulePath, outputPath string) (archivePath string, err error) {
	// Load and validate the module first
	m, err := Load(modulePath)
	if err != nil {
		return "", fmt.Errorf("invalid module: %w", err)
	}

	// Determine output path
	if outputPath == "" {
		outputPath = m.Name() + ModuleSuffix + ".zip"
	}

	// Resolve absolute output path
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve output path: %w", err)
	}

	// Create the ZIP file
	zipFile, err := os.Create(absOutputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create ZIP file: %w", err)
	}
	defer func() {
		if closeErr := zipFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		if closeErr := zipWriter.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Get the module directory name for the ZIP root
	moduleDirName := filepath.Base(m.Path)

	// Walk the module directory and add files to the ZIP
	walkErr := filepath.WalkDir(m.Path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Get relative path from module root
		relPath, relErr := filepath.Rel(m.Path, path)
		if relErr != nil {
			return fmt.Errorf("failed to get relative path: %w", relErr)
		}

		// Create ZIP path with module directory as root
		zipPath := filepath.Join(moduleDirName, relPath)
		// Use forward slashes for ZIP compatibility
		zipPath = filepath.ToSlash(zipPath)

		if d.IsDir() {
			// Add directory entry
			if relPath != "." {
				_, createErr := zipWriter.Create(zipPath + "/")
				if createErr != nil {
					return fmt.Errorf("failed to create directory entry: %w", createErr)
				}
			}
			return nil
		}

		// Read file contents
		fileData, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read file %s: %w", path, readErr)
		}

		// Get file info for permissions
		fileInfo, infoErr := d.Info()
		if infoErr != nil {
			return fmt.Errorf("failed to get file info: %w", infoErr)
		}

		// Create file header with proper attributes
		header, headerErr := zip.FileInfoHeader(fileInfo)
		if headerErr != nil {
			return fmt.Errorf("failed to create file header: %w", headerErr)
		}
		header.Name = zipPath
		header.Method = zip.Deflate

		// Create file in ZIP
		writer, writerErr := zipWriter.CreateHeader(header)
		if writerErr != nil {
			return fmt.Errorf("failed to create ZIP entry: %w", writerErr)
		}

		_, writeErr := writer.Write(fileData)
		if writeErr != nil {
			return fmt.Errorf("failed to write file data: %w", writeErr)
		}

		return nil
	})

	if walkErr != nil {
		// Clean up on failure - deferred closes will run, then remove the file
		err = fmt.Errorf("failed to archive module: %w", walkErr)
		// Remove file after closes complete (use a separate defer to ensure order)
		defer func() { _ = os.Remove(absOutputPath) }()
		return "", err
	}

	return absOutputPath, nil
}

// UnpackOptions contains options for unpacking a module
type UnpackOptions struct {
	// Source is the path to the ZIP file or URL
	Source string
	// DestDir is the destination directory (defaults to current directory)
	DestDir string
	// Overwrite allows overwriting an existing module
	Overwrite bool
}

// Unpack extracts a module from a ZIP archive.
// Returns the path to the extracted module or an error.
func Unpack(opts UnpackOptions) (extractedPath string, err error) {
	if opts.Source == "" {
		return "", fmt.Errorf("source cannot be empty")
	}

	// Default destination to current directory
	destDir := opts.DestDir
	if destDir == "" {
		destDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Resolve absolute destination path
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve destination directory: %w", err)
	}

	// Ensure destination exists
	if err = os.MkdirAll(absDestDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Check if source is a URL
	var zipPath string
	var cleanup func()
	if strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://") {
		// Download the file
		var tmpFile string
		tmpFile, err = downloadFile(opts.Source)
		if err != nil {
			return "", fmt.Errorf("failed to download module: %w", err)
		}
		zipPath = tmpFile
		cleanup = func() { _ = os.Remove(tmpFile) } // Best-effort cleanup of temp file
	} else {
		// Local file
		zipPath, err = filepath.Abs(opts.Source)
		if err != nil {
			return "", fmt.Errorf("failed to resolve source path: %w", err)
		}
		cleanup = func() {}
	}
	defer cleanup()

	// Open the ZIP file
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer func() {
		if closeErr := zipReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Find the module root directory in the ZIP
	var moduleRoot string
	for _, file := range zipReader.File {
		// Look for the .invkmod directory
		parts := strings.Split(file.Name, "/")
		if len(parts) > 0 && strings.HasSuffix(parts[0], ModuleSuffix) {
			moduleRoot = parts[0]
			break
		}
	}

	if moduleRoot == "" {
		return "", fmt.Errorf("no valid module found in ZIP (expected directory ending with %s)", ModuleSuffix)
	}

	// Check if module already exists
	modulePath := filepath.Join(absDestDir, moduleRoot)
	if _, statErr := os.Stat(modulePath); statErr == nil {
		if !opts.Overwrite {
			return "", fmt.Errorf("module already exists at %s (use overwrite option to replace)", modulePath)
		}
		// Remove existing module
		if err = os.RemoveAll(modulePath); err != nil {
			return "", fmt.Errorf("failed to remove existing module: %w", err)
		}
	}

	// Extract files
	for _, file := range zipReader.File {
		// Skip files not in the module root
		if !strings.HasPrefix(file.Name, moduleRoot) {
			continue
		}

		// Construct destination path
		destPath := filepath.Join(absDestDir, filepath.FromSlash(file.Name))

		// Validate path doesn't escape destination (security check)
		relPath, relErr := filepath.Rel(absDestDir, destPath)
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			return "", fmt.Errorf("invalid path in ZIP: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if mkdirErr := os.MkdirAll(destPath, file.Mode()); mkdirErr != nil {
				return "", fmt.Errorf("failed to create directory: %w", mkdirErr)
			}
			continue
		}

		// Create parent directory if needed
		parentDir := filepath.Dir(destPath)
		if mkdirErr := os.MkdirAll(parentDir, 0o755); mkdirErr != nil {
			return "", fmt.Errorf("failed to create parent directory: %w", mkdirErr)
		}

		// Extract file
		if extractErr := extractFile(file, destPath); extractErr != nil {
			return "", fmt.Errorf("failed to extract %s: %w", file.Name, extractErr)
		}
	}

	// Validate the extracted module
	_, err = Load(modulePath)
	if err != nil {
		// Clean up on validation failure (best-effort)
		_ = os.RemoveAll(modulePath)
		return "", fmt.Errorf("extracted module is invalid: %w", err)
	}

	return modulePath, nil
}

// downloadFile downloads a file from a URL and returns the path to the temporary file
func downloadFile(url string) (tmpPath string, err error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "invowk-module-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath = tmpFile.Name()

	// Clean up temp file on any error
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath) // Best-effort cleanup
		}
	}()

	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Download the file
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL is validated by caller
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Copy response body to file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save downloaded file: %w", err)
	}

	return tmpPath, nil
}

// extractFile extracts a single file from the ZIP archive
func extractFile(file *zip.File, destPath string) (err error) {
	// Open the file in the ZIP
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Create the destination file
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Copy contents
	//nolint:gosec // G110: ZIP extraction from user-trusted sources; size limits handled by filesystem
	_, err = io.Copy(destFile, rc)
	return err
}

// ValidateName checks if a module name is valid.
// Returns nil if valid, or an error describing the problem.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("module name cannot start with a dot")
	}

	if !moduleNameRegex.MatchString(name) {
		return fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", name)
	}

	return nil
}

// GetVendoredModulesDir returns the path to the vendored modules directory for a given module.
// Returns the path whether or not the directory exists.
func GetVendoredModulesDir(modulePath string) string {
	return filepath.Join(modulePath, VendoredModulesDir)
}

// HasVendoredModules checks if a module has vendored dependencies.
// Returns true only if the invk_modules/ directory exists AND contains at least one valid module.
func HasVendoredModules(modulePath string) bool {
	modules, err := ListVendoredModules(modulePath)
	if err != nil {
		return false
	}
	return len(modules) > 0
}

// ListVendoredModules returns a list of vendored modules in the given module directory.
// Returns nil if no invk_modules/ directory exists or it's empty.
func ListVendoredModules(modulePath string) ([]*Module, error) {
	vendorDir := GetVendoredModulesDir(modulePath)

	// Check if vendor directory exists
	info, err := os.Stat(vendorDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat vendor directory: %w", err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read vendor directory: %w", err)
	}

	var modules []*Module
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a module
		entryPath := filepath.Join(vendorDir, entry.Name())
		if !IsModule(entryPath) {
			continue
		}

		// Load the module
		m, err := Load(entryPath)
		if err != nil {
			// Skip invalid modules
			continue
		}

		modules = append(modules, m)
	}

	return modules, nil
}
