// Package bundle provides functionality for working with invowk bundles.
//
// A bundle is a self-contained folder with a ".invowkbundle" suffix that contains
// an invowkfile and optionally script files. Bundles enable portable distribution
// of invowk commands with their associated scripts.
//
// Bundle naming follows these rules:
//   - Folder name must end with ".invowkbundle"
//   - Prefix (before .invowkbundle) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//
// Bundle structure:
//   - Must contain exactly one invowkfile.cue at the root
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other bundles
package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BundleSuffix is the standard suffix for invowk bundle directories
const BundleSuffix = ".invowkbundle"

// bundleNameRegex validates the bundle folder name prefix (before .invowkbundle)
// Must start with a letter, contain only alphanumeric chars, with optional dot-separated segments
// Compatible with RDNS naming (e.g., "com.example.mycommands")
var bundleNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)

// ValidationIssue represents a single validation problem in a bundle
type ValidationIssue struct {
	// Type categorizes the issue (e.g., "structure", "naming", "invowkfile")
	Type string
	// Message describes the specific problem
	Message string
	// Path is the relative path within the bundle where the issue was found (optional)
	Path string
}

// Error implements the error interface for ValidationIssue
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// ValidationResult contains the result of bundle validation
type ValidationResult struct {
	// Valid is true if the bundle passed all validation checks
	Valid bool
	// BundlePath is the absolute path to the validated bundle
	BundlePath string
	// BundleName is the extracted name from the folder (without .invowkbundle suffix)
	BundleName string
	// InvowkfilePath is the path to the invowkfile.cue within the bundle
	InvowkfilePath string
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

// Bundle represents a validated invowk bundle
type Bundle struct {
	// Path is the absolute path to the bundle directory
	Path string
	// Name is the bundle name (folder name without .invowkbundle suffix)
	Name string
	// InvowkfilePath is the absolute path to the invowkfile.cue
	InvowkfilePath string
}

// IsBundle checks if the given path is a valid invowk bundle directory.
// This is a quick check that only verifies the folder name format and existence.
// For full validation, use Validate().
func IsBundle(path string) bool {
	// Check if the path ends with .invowkbundle
	base := filepath.Base(path)
	if !strings.HasSuffix(base, BundleSuffix) {
		return false
	}

	// Check if the prefix is valid
	prefix := strings.TrimSuffix(base, BundleSuffix)
	if !bundleNameRegex.MatchString(prefix) {
		return false
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ParseBundleName extracts and validates the bundle name from a folder name.
// The folder name must end with .invowkbundle and have a valid prefix.
// Returns the bundle name (without suffix) or an error if invalid.
func ParseBundleName(folderName string) (string, error) {
	// Must end with .invowkbundle
	if !strings.HasSuffix(folderName, BundleSuffix) {
		return "", fmt.Errorf("folder name must end with '%s'", BundleSuffix)
	}

	// Extract prefix
	prefix := strings.TrimSuffix(folderName, BundleSuffix)
	if prefix == "" {
		return "", fmt.Errorf("bundle name cannot be empty (folder name cannot be just '%s')", BundleSuffix)
	}

	// Must not start with a dot (hidden folder)
	if strings.HasPrefix(prefix, ".") {
		return "", fmt.Errorf("bundle name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	if !bundleNameRegex.MatchString(prefix) {
		return "", fmt.Errorf("bundle name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	return prefix, nil
}

// Validate performs comprehensive validation of a bundle at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(bundlePath string) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:      true,
		BundlePath: absPath,
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

	// Validate folder name and extract bundle name
	base := filepath.Base(absPath)
	bundleName, err := ParseBundleName(base)
	if err != nil {
		result.AddIssue("naming", err.Error(), "")
	} else {
		result.BundleName = bundleName
	}

	// Check for invowkfile.cue
	invowkfilePath := filepath.Join(absPath, "invowkfile.cue")
	invowkfileInfo, err := os.Stat(invowkfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue("structure", "missing required invowkfile.cue", "")
		} else {
			result.AddIssue("structure", fmt.Sprintf("cannot access invowkfile.cue: %v", err), "")
		}
	} else if invowkfileInfo.IsDir() {
		result.AddIssue("structure", "invowkfile.cue must be a file, not a directory", "")
	} else {
		result.InvowkfilePath = invowkfilePath
	}

	// Check for nested bundles (not allowed)
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Continue walking even on errors
		}

		// Skip the root directory itself
		if path == absPath {
			return nil
		}

		// Check if any subdirectory is a bundle
		if d.IsDir() && strings.HasSuffix(d.Name(), BundleSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("structure", "nested bundles are not allowed", relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk bundle directory: %w", err)
	}

	return result, nil
}

// Load loads and validates a bundle at the given path.
// Returns a Bundle struct if valid, or an error with validation details.
func Load(bundlePath string) (*Bundle, error) {
	result, err := Validate(bundlePath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		return nil, fmt.Errorf("invalid bundle: %s", strings.Join(msgs, "; "))
	}

	return &Bundle{
		Path:           result.BundlePath,
		Name:           result.BundleName,
		InvowkfilePath: result.InvowkfilePath,
	}, nil
}

// ResolveScriptPath resolves a script path relative to the bundle root.
// Script paths in bundles should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (b *Bundle) ResolveScriptPath(scriptPath string) string {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(scriptPath)

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return nativePath
	}

	// Resolve relative to bundle root
	return filepath.Join(b.Path, nativePath)
}

// ValidateScriptPath checks if a script path is valid for this bundle.
// Returns an error if the path is invalid (e.g., escapes bundle directory).
func (b *Bundle) ValidateScriptPath(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(scriptPath)

	// Absolute paths are not allowed in bundles
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in bundles; use paths relative to bundle root")
	}

	// Resolve the full path
	fullPath := filepath.Join(b.Path, nativePath)

	// Ensure the resolved path is within the bundle (prevent directory traversal)
	relPath, err := filepath.Rel(b.Path, fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for path escaping (e.g., "../something")
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("script path '%s' escapes the bundle directory", scriptPath)
	}

	return nil
}

// ContainsPath checks if the given path is inside this bundle.
func (b *Bundle) ContainsPath(path string) bool {
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

// GetInvowkfileDir returns the directory containing the invowkfile.
// For bundles, this is always the bundle root.
func (b *Bundle) GetInvowkfileDir() string {
	return b.Path
}

// CreateOptions contains options for creating a new bundle
type CreateOptions struct {
	// Name is the bundle name (e.g., "com.example.mytools")
	Name string
	// ParentDir is the directory where the bundle will be created
	ParentDir string
	// Group is the group name for the invowkfile (defaults to Name if empty)
	Group string
	// Description is an optional description for the invowkfile
	Description string
	// CreateScriptsDir creates a scripts/ subdirectory if true
	CreateScriptsDir bool
}

// Create creates a new bundle with the given options.
// Returns the path to the created bundle or an error.
func Create(opts CreateOptions) (string, error) {
	// Validate bundle name
	if opts.Name == "" {
		return "", fmt.Errorf("bundle name cannot be empty")
	}

	// Validate the name format
	if !bundleNameRegex.MatchString(opts.Name) {
		return "", fmt.Errorf("bundle name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", opts.Name)
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

	// Create bundle directory
	bundleDirName := opts.Name + BundleSuffix
	bundlePath := filepath.Join(absParentDir, bundleDirName)

	// Check if bundle already exists
	if _, err := os.Stat(bundlePath); err == nil {
		return "", fmt.Errorf("bundle already exists at %s", bundlePath)
	}

	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create bundle directory: %w", err)
	}

	// Use name as group if not specified
	group := opts.Group
	if group == "" {
		group = opts.Name
	}

	// Create invowkfile.cue template
	description := opts.Description
	if description == "" {
		description = fmt.Sprintf("Commands from %s bundle", opts.Name)
	}

	invowkfileContent := fmt.Sprintf(`// Invowkfile for %s bundle

group: %q
version: "1.0"
description: %q

commands: [
	{
		name:        "hello"
		description: "A sample command"
		implementations: [
			{
				script: "echo \"Hello from %s!\""
				target: {
					runtimes: [
						{name: "native"},
						{name: "virtual"},
					]
				}
			},
		]
	},
]
`, opts.Name, group, description, opts.Name)

	invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0644); err != nil {
		// Clean up on failure
		os.RemoveAll(bundlePath)
		return "", fmt.Errorf("failed to create invowkfile.cue: %w", err)
	}

	// Optionally create scripts directory
	if opts.CreateScriptsDir {
		scriptsDir := filepath.Join(bundlePath, "scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			// Clean up on failure
			os.RemoveAll(bundlePath)
			return "", fmt.Errorf("failed to create scripts directory: %w", err)
		}

		// Create a placeholder .gitkeep file
		gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), 0644); err != nil {
			// Clean up on failure
			os.RemoveAll(bundlePath)
			return "", fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	return bundlePath, nil
}

// Pack creates a ZIP archive of a bundle.
// Returns the path to the created ZIP file or an error.
func Pack(bundlePath, outputPath string) (string, error) {
	// Load and validate the bundle first
	b, err := Load(bundlePath)
	if err != nil {
		return "", fmt.Errorf("invalid bundle: %w", err)
	}

	// Determine output path
	if outputPath == "" {
		outputPath = b.Name + BundleSuffix + ".zip"
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

	// Get the bundle directory name for the ZIP root
	bundleDirName := filepath.Base(b.Path)

	// Walk the bundle directory and add files to the ZIP
	err = filepath.WalkDir(b.Path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Get relative path from bundle root
		relPath, err := filepath.Rel(b.Path, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create ZIP path with bundle directory as root
		zipPath := filepath.Join(bundleDirName, relPath)
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
		return "", fmt.Errorf("failed to pack bundle: %w", err)
	}

	return absOutputPath, nil
}

// UnpackOptions contains options for unpacking a bundle
type UnpackOptions struct {
	// Source is the path to the ZIP file or URL
	Source string
	// DestDir is the destination directory (defaults to current directory)
	DestDir string
	// Overwrite allows overwriting an existing bundle
	Overwrite bool
}

// Unpack extracts a bundle from a ZIP archive.
// Returns the path to the extracted bundle or an error.
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
			return "", fmt.Errorf("failed to download bundle: %w", err)
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

	// Find the bundle root directory in the ZIP
	var bundleRoot string
	for _, file := range zipReader.File {
		// Look for the .invowkbundle directory
		parts := strings.Split(file.Name, "/")
		if len(parts) > 0 && strings.HasSuffix(parts[0], BundleSuffix) {
			bundleRoot = parts[0]
			break
		}
	}

	if bundleRoot == "" {
		return "", fmt.Errorf("no valid bundle found in ZIP (expected directory ending with %s)", BundleSuffix)
	}

	// Check if bundle already exists
	bundlePath := filepath.Join(absDestDir, bundleRoot)
	if _, err := os.Stat(bundlePath); err == nil {
		if !opts.Overwrite {
			return "", fmt.Errorf("bundle already exists at %s (use overwrite option to replace)", bundlePath)
		}
		// Remove existing bundle
		if err := os.RemoveAll(bundlePath); err != nil {
			return "", fmt.Errorf("failed to remove existing bundle: %w", err)
		}
	}

	// Extract files
	for _, file := range zipReader.File {
		// Skip files not in the bundle root
		if !strings.HasPrefix(file.Name, bundleRoot) {
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

	// Validate the extracted bundle
	_, err = Load(bundlePath)
	if err != nil {
		// Clean up on validation failure
		os.RemoveAll(bundlePath)
		return "", fmt.Errorf("extracted bundle is invalid: %w", err)
	}

	return bundlePath, nil
}

// downloadFile downloads a file from a URL and returns the path to the temporary file
func downloadFile(url string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "invowk-bundle-*.zip")
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

// ValidateName checks if a bundle name is valid.
// Returns nil if valid, or an error describing the problem.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("bundle name cannot be empty")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("bundle name cannot start with a dot")
	}

	if !bundleNameRegex.MatchString(name) {
		return fmt.Errorf("bundle name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", name)
	}

	return nil
}
