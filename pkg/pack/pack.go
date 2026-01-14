// SPDX-License-Identifier: EPL-2.0

// Package pack provides functionality for working with invowk packs.
//
// A pack is a self-contained folder with a ".invkpack" suffix that contains
// an invkfile and optionally script files. Packs enable portable distribution
// of invowk commands with their associated scripts.
//
// Pack naming follows these rules:
//   - Folder name must end with ".invkpack"
//   - Prefix (before .invkpack) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//
// Pack structure:
//   - Must contain exactly one invkfile.cue at the root
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other packs
package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"invowk-cli/pkg/invkfile"
)

// PackSuffix is the standard suffix for invowk pack directories
const PackSuffix = ".invkpack"

// VendoredPacksDir is the directory name for vendored pack dependencies
const VendoredPacksDir = "invk_packs"

// packNameRegex validates the pack folder name prefix (before .invkpack)
// Must start with a letter, contain only alphanumeric chars, with optional dot-separated segments
// Compatible with RDNS naming (e.g., "com.example.mycommands")
var packNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)

// ValidationIssue represents a single validation problem in a pack
type ValidationIssue struct {
	// Type categorizes the issue (e.g., "structure", "naming", "invkfile")
	Type string
	// Message describes the specific problem
	Message string
	// Path is the relative path within the pack where the issue was found (optional)
	Path string
}

// Error implements the error interface for ValidationIssue
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// ValidationResult contains the result of pack validation
type ValidationResult struct {
	// Valid is true if the pack passed all validation checks
	Valid bool
	// PackPath is the absolute path to the validated pack
	PackPath string
	// PackName is the extracted name from the folder (without .invkpack suffix)
	PackName string
	// InvkpackPath is the path to the invkpack.cue within the pack (required)
	InvkpackPath string
	// InvkfilePath is the path to the invkfile.cue within the pack (optional for library-only packs)
	InvkfilePath string
	// IsLibraryOnly is true if the pack has no invkfile.cue
	IsLibraryOnly bool
	// Issues contains all validation problems found
	Issues []ValidationIssue
}

// AddIssue adds a validation issue to the result
func (r *ValidationResult) AddIssue(issueType, message, path string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Type:    issueType,
		Message: message,
		Path:    path,
	})
	r.Valid = false
}

// Pack represents a validated invowk pack
type Pack struct {
	// Path is the absolute path to the pack directory
	Path string
	// Name is the pack name (folder name without .invkpack suffix)
	Name string
	// InvkpackPath is the absolute path to the invkpack.cue (required)
	InvkpackPath string
	// InvkfilePath is the absolute path to the invkfile.cue (optional for library-only packs)
	InvkfilePath string
	// IsLibraryOnly is true if the pack has no invkfile.cue
	IsLibraryOnly bool
}

// IsPack checks if the given path is a valid invowk pack directory.
// This is a quick check that only verifies the folder name format and existence.
// For full validation, use Validate().
func IsPack(path string) bool {
	// Check if the path ends with .invkpack
	base := filepath.Base(path)
	if !strings.HasSuffix(base, PackSuffix) {
		return false
	}

	// Check if the prefix is valid
	prefix := strings.TrimSuffix(base, PackSuffix)
	if !packNameRegex.MatchString(prefix) {
		return false
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ParsePackName extracts and validates the pack name from a folder name.
// The folder name must end with .invkpack and have a valid prefix.
// Returns the pack name (without suffix) or an error if invalid.
func ParsePackName(folderName string) (string, error) {
	// Must end with .invkpack
	if !strings.HasSuffix(folderName, PackSuffix) {
		return "", fmt.Errorf("folder name must end with '%s'", PackSuffix)
	}

	// Extract prefix
	prefix := strings.TrimSuffix(folderName, PackSuffix)
	if prefix == "" {
		return "", fmt.Errorf("pack name cannot be empty (folder name cannot be just '%s')", PackSuffix)
	}

	// Must not start with a dot (hidden folder)
	if strings.HasPrefix(prefix, ".") {
		return "", fmt.Errorf("pack name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	if !packNameRegex.MatchString(prefix) {
		return "", fmt.Errorf("pack name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	return prefix, nil
}

// Validate performs comprehensive validation of a pack at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(packPath string) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(packPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:    true,
		PackPath: absPath,
		Issues:   []ValidationIssue{},
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

	// Validate folder name and extract pack name
	base := filepath.Base(absPath)
	packName, err := ParsePackName(base)
	if err != nil {
		result.AddIssue("naming", err.Error(), "")
	} else {
		result.PackName = packName
	}

	// Check for invkpack.cue (required)
	invkpackPath := filepath.Join(absPath, "invkpack.cue")
	invkpackInfo, err := os.Stat(invkpackPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue("structure", "missing required invkpack.cue", "")
		} else {
			result.AddIssue("structure", fmt.Sprintf("cannot access invkpack.cue: %v", err), "")
		}
	} else if invkpackInfo.IsDir() {
		result.AddIssue("structure", "invkpack.cue must be a file, not a directory", "")
	} else {
		result.InvkpackPath = invkpackPath

		// Parse invkpack.cue and validate pack field matches folder name
		if result.PackName != "" {
			meta, parseErr := invkfile.ParseInvkpack(invkpackPath)
			if parseErr != nil {
				result.AddIssue("invkpack", fmt.Sprintf("failed to parse invkpack.cue: %v", parseErr), "invkpack.cue")
			} else if meta.Pack != result.PackName {
				result.AddIssue("naming", fmt.Sprintf(
					"pack field '%s' in invkpack.cue must match folder name '%s'",
					meta.Pack, result.PackName), "invkpack.cue")
			}
		}
	}

	// Check for invkfile.cue (optional - may be a library-only pack)
	invkfilePath := filepath.Join(absPath, "invkfile.cue")
	invkfileInfo, err := os.Stat(invkfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Library-only pack - no commands
			result.IsLibraryOnly = true
		} else {
			result.AddIssue("structure", fmt.Sprintf("cannot access invkfile.cue: %v", err), "")
		}
	} else if invkfileInfo.IsDir() {
		result.AddIssue("structure", "invkfile.cue must be a file, not a directory", "")
	} else {
		result.InvkfilePath = invkfilePath
	}

	// Check for nested packs (not allowed, except in invk_packs/ for vendored deps)
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Continue walking even on errors
		}

		// Skip the root directory itself
		if path == absPath {
			return nil
		}

		// Skip the vendored packs directory (invk_packs/) - nested packs are allowed there
		if d.IsDir() && d.Name() == VendoredPacksDir {
			return filepath.SkipDir
		}

		// Check if any other subdirectory is a pack
		if d.IsDir() && strings.HasSuffix(d.Name(), PackSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("structure", "nested packs are not allowed (except in invk_packs/)", relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk pack directory: %w", err)
	}

	return result, nil
}

// Load loads and validates a pack at the given path.
// Returns a Pack struct if valid, or an error with validation details.
func Load(packPath string) (*Pack, error) {
	result, err := Validate(packPath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		return nil, fmt.Errorf("invalid pack: %s", strings.Join(msgs, "; "))
	}

	return &Pack{
		Path:          result.PackPath,
		Name:          result.PackName,
		InvkpackPath:  result.InvkpackPath,
		InvkfilePath:  result.InvkfilePath,
		IsLibraryOnly: result.IsLibraryOnly,
	}, nil
}

// ResolveScriptPath resolves a script path relative to the pack root.
// Script paths in packs should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (b *Pack) ResolveScriptPath(scriptPath string) string {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(scriptPath)

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return nativePath
	}

	// Resolve relative to pack root
	return filepath.Join(b.Path, nativePath)
}

// ValidateScriptPath checks if a script path is valid for this pack.
// Returns an error if the path is invalid (e.g., escapes pack directory).
func (b *Pack) ValidateScriptPath(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(scriptPath)

	// Absolute paths are not allowed in packs
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in packs; use paths relative to pack root")
	}

	// Resolve the full path
	fullPath := filepath.Join(b.Path, nativePath)

	// Ensure the resolved path is within the pack (prevent directory traversal)
	relPath, err := filepath.Rel(b.Path, fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for path escaping (e.g., "../something")
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("script path '%s' escapes the pack directory", scriptPath)
	}

	return nil
}

// ContainsPath checks if the given path is inside this pack.
func (b *Pack) ContainsPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(b.Path, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, "..")
}

// GetInvkfileDir returns the directory containing the invkfile.
// For packs, this is always the pack root.
func (b *Pack) GetInvkfileDir() string {
	return b.Path
}

// CreateOptions contains options for creating a new pack
type CreateOptions struct {
	// Name is the pack name (e.g., "com.example.mytools")
	Name string
	// ParentDir is the directory where the pack will be created
	ParentDir string
	// Pack is the pack identifier for the invkfile (defaults to Name if empty)
	Pack string
	// Description is an optional description for the invkfile
	Description string
	// CreateScriptsDir creates a scripts/ subdirectory if true
	CreateScriptsDir bool
}

// Create creates a new pack with the given options.
// Returns the path to the created pack or an error.
func Create(opts CreateOptions) (string, error) {
	// Validate pack name
	if opts.Name == "" {
		return "", fmt.Errorf("pack name cannot be empty")
	}

	// Validate the name format
	if !packNameRegex.MatchString(opts.Name) {
		return "", fmt.Errorf("pack name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", opts.Name)
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

	// Create pack directory
	packDirName := opts.Name + PackSuffix
	packPath := filepath.Join(absParentDir, packDirName)

	// Check if pack already exists
	if _, err := os.Stat(packPath); err == nil {
		return "", fmt.Errorf("pack already exists at %s", packPath)
	}

	if err := os.MkdirAll(packPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create pack directory: %w", err)
	}

	// Use name as pack identifier if not specified
	packID := opts.Pack
	if packID == "" {
		packID = opts.Name
	}

	// Create description
	description := opts.Description
	if description == "" {
		description = fmt.Sprintf("Commands from %s pack", opts.Name)
	}

	// Create invkpack.cue (pack metadata)
	invkpackContent := fmt.Sprintf(`// Invkpack - Pack metadata for %s
// See https://github.com/invowk/invowk for documentation

pack: %q
version: "1.0"
description: %q

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invkpack.git"
//         version: "^1.0.0"
//     },
// ]
`, opts.Name, packID, description)

	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	if err := os.WriteFile(invkpackPath, []byte(invkpackContent), 0644); err != nil {
		// Clean up on failure
		os.RemoveAll(packPath)
		return "", fmt.Errorf("failed to create invkpack.cue: %w", err)
	}

	// Create invkfile.cue (command definitions only)
	invkfileContent := fmt.Sprintf(`// Invkfile - Command definitions for %s pack
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

	invkfilePath := filepath.Join(packPath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0644); err != nil {
		// Clean up on failure
		os.RemoveAll(packPath)
		return "", fmt.Errorf("failed to create invkfile.cue: %w", err)
	}

	// Optionally create scripts directory
	if opts.CreateScriptsDir {
		scriptsDir := filepath.Join(packPath, "scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			// Clean up on failure
			os.RemoveAll(packPath)
			return "", fmt.Errorf("failed to create scripts directory: %w", err)
		}

		// Create a placeholder .gitkeep file
		gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), 0644); err != nil {
			// Clean up on failure
			os.RemoveAll(packPath)
			return "", fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	return packPath, nil
}

// Archive creates a ZIP archive of a pack.
// Returns the path to the created ZIP file or an error.
func Archive(packPath, outputPath string) (string, error) {
	// Load and validate the pack first
	b, err := Load(packPath)
	if err != nil {
		return "", fmt.Errorf("invalid pack: %w", err)
	}

	// Determine output path
	if outputPath == "" {
		outputPath = b.Name + PackSuffix + ".zip"
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
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Get the pack directory name for the ZIP root
	packDirName := filepath.Base(b.Path)

	// Walk the pack directory and add files to the ZIP
	err = filepath.WalkDir(b.Path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Get relative path from pack root
		relPath, err := filepath.Rel(b.Path, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create ZIP path with pack directory as root
		zipPath := filepath.Join(packDirName, relPath)
		// Use forward slashes for ZIP compatibility
		zipPath = filepath.ToSlash(zipPath)

		if d.IsDir() {
			// Add directory entry
			if relPath != "." {
				_, err := zipWriter.Create(zipPath + "/")
				if err != nil {
					return fmt.Errorf("failed to create directory entry: %w", err)
				}
			}
			return nil
		}

		// Read file contents
		fileData, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Get file info for permissions
		fileInfo, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		// Create file header with proper attributes
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return fmt.Errorf("failed to create file header: %w", err)
		}
		header.Name = zipPath
		header.Method = zip.Deflate

		// Create file in ZIP
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create ZIP entry: %w", err)
		}

		_, err = writer.Write(fileData)
		if err != nil {
			return fmt.Errorf("failed to write file data: %w", err)
		}

		return nil
	})

	if err != nil {
		// Clean up on failure
		zipWriter.Close()
		zipFile.Close()
		os.Remove(absOutputPath)
		return "", fmt.Errorf("failed to pack pack: %w", err)
	}

	return absOutputPath, nil
}

// UnpackOptions contains options for unpacking a pack
type UnpackOptions struct {
	// Source is the path to the ZIP file or URL
	Source string
	// DestDir is the destination directory (defaults to current directory)
	DestDir string
	// Overwrite allows overwriting an existing pack
	Overwrite bool
}

// Unpack extracts a pack from a ZIP archive.
// Returns the path to the extracted pack or an error.
func Unpack(opts UnpackOptions) (string, error) {
	if opts.Source == "" {
		return "", fmt.Errorf("source cannot be empty")
	}

	// Default destination to current directory
	destDir := opts.DestDir
	if destDir == "" {
		var err error
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
	if err := os.MkdirAll(absDestDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Check if source is a URL
	var zipPath string
	var cleanup func()
	if strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://") {
		// Download the file
		tmpFile, err := downloadFile(opts.Source)
		if err != nil {
			return "", fmt.Errorf("failed to download pack: %w", err)
		}
		zipPath = tmpFile
		cleanup = func() { os.Remove(tmpFile) }
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
	defer zipReader.Close()

	// Find the pack root directory in the ZIP
	var packRoot string
	for _, file := range zipReader.File {
		// Look for the .invkpack directory
		parts := strings.Split(file.Name, "/")
		if len(parts) > 0 && strings.HasSuffix(parts[0], PackSuffix) {
			packRoot = parts[0]
			break
		}
	}

	if packRoot == "" {
		return "", fmt.Errorf("no valid pack found in ZIP (expected directory ending with %s)", PackSuffix)
	}

	// Check if pack already exists
	packPath := filepath.Join(absDestDir, packRoot)
	if _, err := os.Stat(packPath); err == nil {
		if !opts.Overwrite {
			return "", fmt.Errorf("pack already exists at %s (use overwrite option to replace)", packPath)
		}
		// Remove existing pack
		if err := os.RemoveAll(packPath); err != nil {
			return "", fmt.Errorf("failed to remove existing pack: %w", err)
		}
	}

	// Extract files
	for _, file := range zipReader.File {
		// Skip files not in the pack root
		if !strings.HasPrefix(file.Name, packRoot) {
			continue
		}

		// Construct destination path
		destPath := filepath.Join(absDestDir, filepath.FromSlash(file.Name))

		// Validate path doesn't escape destination (security check)
		relPath, err := filepath.Rel(absDestDir, destPath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return "", fmt.Errorf("invalid path in ZIP: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, file.Mode()); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create parent directory if needed
		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Extract file
		if err := extractFile(file, destPath); err != nil {
			return "", fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
	}

	// Validate the extracted pack
	_, err = Load(packPath)
	if err != nil {
		// Clean up on validation failure
		os.RemoveAll(packPath)
		return "", fmt.Errorf("extracted pack is invalid: %w", err)
	}

	return packPath, nil
}

// downloadFile downloads a file from a URL and returns the path to the temporary file
func downloadFile(url string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "invowk-pack-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Copy response body to file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save downloaded file: %w", err)
	}

	return tmpFile.Name(), nil
}

// extractFile extracts a single file from the ZIP archive
func extractFile(file *zip.File, destPath string) error {
	// Open the file in the ZIP
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create the destination file
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy contents
	_, err = io.Copy(destFile, rc)
	return err
}

// ValidateName checks if a pack name is valid.
// Returns nil if valid, or an error describing the problem.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("pack name cannot be empty")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("pack name cannot start with a dot")
	}

	if !packNameRegex.MatchString(name) {
		return fmt.Errorf("pack name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", name)
	}

	return nil
}

// GetVendoredPacksDir returns the path to the vendored packs directory for a given pack.
// Returns the path whether or not the directory exists.
func GetVendoredPacksDir(packPath string) string {
	return filepath.Join(packPath, VendoredPacksDir)
}

// HasVendoredPacks checks if a pack has vendored dependencies.
// Returns true only if the invk_packs/ directory exists AND contains at least one valid pack.
func HasVendoredPacks(packPath string) bool {
	packs, err := ListVendoredPacks(packPath)
	if err != nil {
		return false
	}
	return len(packs) > 0
}

// ListVendoredPacks returns a list of vendored packs in the given pack directory.
// Returns nil if no invk_packs/ directory exists or it's empty.
func ListVendoredPacks(packPath string) ([]*Pack, error) {
	vendorDir := GetVendoredPacksDir(packPath)

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

	var packs []*Pack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a pack
		entryPath := filepath.Join(vendorDir, entry.Name())
		if !IsPack(entryPath) {
			continue
		}

		// Load the pack
		p, err := Load(entryPath)
		if err != nil {
			// Skip invalid packs
			continue
		}

		packs = append(packs, p)
	}

	return packs, nil
}
