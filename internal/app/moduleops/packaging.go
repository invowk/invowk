// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	defaultExtractedDirectoryMode os.FileMode = 0o755
	legacyExtractedDirectoryMode  os.FileMode = 0o666
	zipCreatorPlatformShift                   = 8
)

var (
	// ErrInvalidZIPPath is returned when a ZIP archive entry has an unsafe path
	// (e.g., path traversal, absolute path, or empty path).
	ErrInvalidZIPPath = errors.New("invalid path in ZIP")

	// ErrNoValidModuleInZIP is returned when a ZIP archive does not contain
	// a directory ending with .invowkmod.
	ErrNoValidModuleInZIP = errors.New("no valid module found in ZIP")
)

type (
	// ArchiveSourceFetcher resolves an unpack source to a local ZIP file.
	ArchiveSourceFetcher interface {
		FetchArchiveSource(ctx context.Context, source string) (path types.FilesystemPath, cleanup func(), err error)
	}

	// UnpackOptions contains options for unpacking a module.
	UnpackOptions struct {
		// Source is the path to the ZIP file or URL; intentionally untyped (mixed path/URL).
		Source string
		// DestDir is the destination directory (defaults to current directory)
		DestDir types.FilesystemPath
		// Overwrite allows overwriting an existing module
		Overwrite bool
		// SourceFetcher resolves local and remote sources for production or tests.
		SourceFetcher ArchiveSourceFetcher
	}

	// UnpackResult contains the completed module import result.
	UnpackResult struct {
		modulePath types.FilesystemPath
		moduleName invowkmod.ModuleID
	}

	// DefaultArchiveSourceFetcher resolves local paths and HTTP(S) URLs.
	DefaultArchiveSourceFetcher struct {
		HTTPClient *http.Client
	}
)

// NewUnpackResult creates a validated module import result.
func NewUnpackResult(modulePath types.FilesystemPath, moduleName invowkmod.ModuleID) (UnpackResult, error) {
	result := UnpackResult{
		modulePath: modulePath,
		moduleName: moduleName,
	}
	if err := result.Validate(); err != nil {
		return UnpackResult{}, err
	}
	return result, nil
}

// ModulePath returns the imported module path.
func (r UnpackResult) ModulePath() types.FilesystemPath { return r.modulePath }

// ModuleName returns the imported module name.
func (r UnpackResult) ModuleName() invowkmod.ModuleID { return r.moduleName }

// Validate returns nil when the module import result is structurally valid.
func (r UnpackResult) Validate() error {
	var errs []error
	if err := r.modulePath.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("module path: %w", err))
	}
	if err := r.moduleName.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("module name: %w", err))
	}
	return errors.Join(errs...)
}

// Archive creates a ZIP archive of a module.
// Returns the path to the created ZIP file or an error.
func Archive(modulePath, outputPath types.FilesystemPath) (archivePath types.FilesystemPath, err error) {
	// Load and validate the module first
	m, err := invowkmod.Load(modulePath)
	if err != nil {
		return "", fmt.Errorf("invalid module: %w", err)
	}

	// Determine output path
	outputStr := string(outputPath)
	if outputStr == "" {
		outputStr = string(m.Name()) + invowkmod.ModuleSuffix + ".zip"
	}

	// Resolve absolute output path
	absOutputPath, err := filepath.Abs(outputStr)
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
	moduleDirName := filepath.Base(string(m.Path))

	// Walk the module directory and add files to the ZIP
	walkErr := filepath.WalkDir(string(m.Path), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Get relative path from module root
		relPath, relErr := filepath.Rel(string(m.Path), path)
		if relErr != nil {
			return fmt.Errorf("failed to get relative path: %w", relErr)
		}

		// Create ZIP path with module directory as root
		zipPath := filepath.Join(moduleDirName, relPath)
		// Use forward slashes for ZIP compatibility
		zipPath = filepath.ToSlash(zipPath)

		if d.IsDir() {
			if relPath != "." {
				dirInfo, infoErr := d.Info()
				if infoErr != nil {
					return fmt.Errorf("failed to get directory info: %w", infoErr)
				}

				header, headerErr := zip.FileInfoHeader(dirInfo)
				if headerErr != nil {
					return fmt.Errorf("failed to create directory header: %w", headerErr)
				}
				header.Name = zipPath + "/"
				header.Method = zip.Store

				_, createErr := zipWriter.CreateHeader(header)
				if createErr != nil {
					return fmt.Errorf("failed to create directory entry: %w", createErr)
				}
			}
			return nil
		}

		// Read file contents
		fileData, readErr := os.ReadFile(path) //nolint:gosec // G122 — caller-controlled module dir; WalkDir does not follow symlinks
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

	archiveResult := types.FilesystemPath(absOutputPath)
	if err := archiveResult.Validate(); err != nil {
		return "", fmt.Errorf("archive output path: %w", err)
	}
	return archiveResult, nil
}

// Unpack extracts a module from a ZIP archive.
// The context controls cancellation for network-based sources (URL downloads).
// Returns the validated module import result or an error.
func Unpack(ctx context.Context, opts UnpackOptions) (result UnpackResult, err error) {
	if opts.Source == "" {
		return UnpackResult{}, errors.New("source cannot be empty")
	}

	absDestDir, err := resolveUnpackDestination(opts.DestDir)
	if err != nil {
		return UnpackResult{}, err
	}

	sourceFetcher := opts.SourceFetcher
	if sourceFetcher == nil {
		sourceFetcher = DefaultArchiveSourceFetcher{}
	}
	zipPath, cleanup, err := sourceFetcher.FetchArchiveSource(ctx, opts.Source)
	if err != nil {
		return UnpackResult{}, err
	}
	defer cleanup()

	zipReader, err := zip.OpenReader(string(zipPath))
	if err != nil {
		return UnpackResult{}, fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer func() {
		if closeErr := zipReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	moduleRoot, err := findModuleRoot(zipReader.File)
	if err != nil {
		return UnpackResult{}, err
	}

	modulePath, err := prepareModuleDestination(absDestDir, moduleRoot, opts.Overwrite)
	if err != nil {
		return UnpackResult{}, err
	}
	modulePathValue := types.FilesystemPath(modulePath)
	if pathErr := modulePathValue.Validate(); pathErr != nil {
		return UnpackResult{}, fmt.Errorf("module destination path: %w", pathErr)
	}

	directories, extractErr := extractModuleFiles(zipReader.File, moduleRoot, absDestDir)
	if extractErr != nil {
		return UnpackResult{}, cleanupUnpackFailure(modulePathValue, directories, extractErr)
	}

	mod, err := validateExtractedModule(modulePathValue)
	if err != nil {
		validationErr := fmt.Errorf("extracted module is invalid: %w", err)
		return UnpackResult{}, cleanupUnpackFailure(modulePathValue, directories, validationErr)
	}
	if restoreErr := restoreExtractedDirectoryModes(directories); restoreErr != nil {
		return UnpackResult{}, cleanupUnpackFailure(modulePathValue, directories, restoreErr)
	}

	result, resultErr := NewUnpackResult(modulePathValue, mod.Name())
	if resultErr != nil {
		return UnpackResult{}, cleanupUnpackFailure(modulePathValue, directories, resultErr)
	}
	return result, nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func resolveUnpackDestination(destDir types.FilesystemPath) (string, error) {
	destination := string(destDir)
	if destination == "" {
		var err error
		destination, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	absDestDir, err := filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("failed to resolve destination directory: %w", err)
	}
	if err := os.MkdirAll(absDestDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}
	return absDestDir, nil
}

// FetchArchiveSource resolves a local ZIP path or downloads a remote ZIP URL.
func (f DefaultArchiveSourceFetcher) FetchArchiveSource(ctx context.Context, source string) (zipPath types.FilesystemPath, cleanup func(), err error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		zipPath, err = f.downloadFile(ctx, source)
		if err != nil {
			return "", nil, fmt.Errorf("failed to download module: %w", err)
		}
		return zipPath, func() { _ = os.Remove(string(zipPath)) }, nil
	}

	absZipPath, err := filepath.Abs(source)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve source path: %w", err)
	}
	zipPath = types.FilesystemPath(absZipPath) //goplint:ignore -- validated before return.
	if err := zipPath.Validate(); err != nil {
		return "", nil, fmt.Errorf("source path: %w", err)
	}
	return zipPath, func() { /* source is user-provided path — no cleanup needed */ }, nil
}

func (f DefaultArchiveSourceFetcher) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

//goplint:ignore -- unpack helpers operate on transient ZIP member path strings.
func normalizeZIPPath(name string) (string, error) {
	cleanPath := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if cleanPath == "." || cleanPath == "" ||
		strings.HasPrefix(cleanPath, "/") ||
		cleanPath == ".." ||
		strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%w: %s", ErrInvalidZIPPath, name)
	}
	return cleanPath, nil
}

//goplint:ignore -- unpack helpers operate on transient ZIP member path strings.
func findModuleRoot(files []*zip.File) (string, error) {
	for _, file := range files {
		cleanPath, err := normalizeZIPPath(file.Name)
		if err != nil {
			return "", err
		}
		parts := strings.Split(cleanPath, "/")
		if len(parts) > 0 && strings.HasSuffix(parts[0], invowkmod.ModuleSuffix) {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("%w (expected directory ending with %s)", ErrNoValidModuleInZIP, invowkmod.ModuleSuffix)
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func prepareModuleDestination(absDestDir, moduleRoot string, overwrite bool) (string, error) {
	modulePath := filepath.Join(absDestDir, filepath.FromSlash(moduleRoot))
	if err := validateDestinationPath(absDestDir, modulePath, moduleRoot); err != nil {
		return "", err
	}

	if _, statErr := os.Stat(modulePath); statErr == nil {
		if !overwrite {
			return "", fmt.Errorf("%w at %s (use overwrite option to replace)", invowkmod.ErrModuleAlreadyExists, modulePath)
		}
		if err := os.RemoveAll(modulePath); err != nil {
			return "", fmt.Errorf("failed to remove existing module: %w", err)
		}
	}

	return modulePath, nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func extractModuleFiles(files []*zip.File, moduleRoot, absDestDir string) (map[types.FilesystemPath]os.FileMode, error) {
	directories := make(map[types.FilesystemPath]os.FileMode)
	for _, file := range files {
		cleanPath, err := normalizeZIPPath(file.Name)
		if err != nil {
			return directories, err
		}
		if cleanPath != moduleRoot && !strings.HasPrefix(cleanPath, moduleRoot+"/") {
			continue
		}
		if file.FileInfo().IsDir() {
			destPath, directoryMode, prepareErr := prepareExtractedDirectory(file, cleanPath, absDestDir)
			if prepareErr != nil {
				return directories, prepareErr
			}
			directories[destPath] = directoryMode
			continue
		}
		if err := extractSingleEntry(file, cleanPath, absDestDir); err != nil {
			return directories, err
		}
	}
	return directories, nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func prepareExtractedDirectory(file *zip.File, cleanPath, absDestDir string) (types.FilesystemPath, os.FileMode, error) {
	destPath := filepath.Join(absDestDir, filepath.FromSlash(cleanPath))
	if err := validateDestinationPath(absDestDir, destPath, file.Name); err != nil {
		return "", 0, err
	}

	directoryMode := file.Mode().Perm()
	// Older Invowk archives used zip.Writer.Create for directories, leaving the
	// creator platform and external attributes unset. Distinguish that legacy
	// signature from an explicitly authored Unix directory with mode 0666.
	if directoryMode == legacyExtractedDirectoryMode &&
		file.CreatorVersion>>zipCreatorPlatformShift == 0 &&
		file.ExternalAttrs == 0 {
		directoryMode = defaultExtractedDirectoryMode
	}
	temporaryMode := directoryMode | 0o700
	if err := os.MkdirAll(filepath.Dir(destPath), defaultExtractedDirectoryMode); err != nil {
		return "", 0, fmt.Errorf("failed to create parent directory: %w", err)
	}
	if err := os.Mkdir(destPath, temporaryMode); err != nil && !errors.Is(err, os.ErrExist) {
		return "", 0, fmt.Errorf("failed to create directory: %w", err)
	}
	dirInfo, statErr := os.Stat(destPath)
	if statErr != nil {
		return "", 0, fmt.Errorf("failed to inspect directory: %w", statErr)
	}
	if !dirInfo.IsDir() {
		return "", 0, fmt.Errorf("failed to create directory: path is not a directory: %s", destPath)
	}
	// MkdirAll does not update a directory created implicitly by an earlier file entry.
	if err := os.Chmod(destPath, temporaryMode); err != nil {
		return "", 0, fmt.Errorf("failed to prepare directory permissions: %w", err)
	}
	directoryPath := types.FilesystemPath(destPath)
	if err := directoryPath.Validate(); err != nil {
		return "", 0, fmt.Errorf("failed to validate extracted directory path: %w", err)
	}
	return directoryPath, directoryMode, nil
}

func restoreExtractedDirectoryModes(directories map[types.FilesystemPath]os.FileMode) error {
	paths := make([]types.FilesystemPath, 0, len(directories))
	for directoryPath := range directories {
		paths = append(paths, directoryPath)
	}
	// Restore children before parents so restrictive parent modes cannot block traversal.
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(string(paths[i]), string(filepath.Separator)) > strings.Count(string(paths[j]), string(filepath.Separator))
	})
	for _, directoryPath := range paths {
		if err := os.Chmod(string(directoryPath), directories[directoryPath]); err != nil {
			return fmt.Errorf("failed to restore directory permissions: %w", err)
		}
	}
	return nil
}

func cleanupExtractedModule(modulePath types.FilesystemPath, directories map[types.FilesystemPath]os.FileMode) error {
	paths := make([]types.FilesystemPath, 0, len(directories))
	for directoryPath := range directories {
		paths = append(paths, directoryPath)
	}
	// Regrant parent access before children so cleanup can traverse restrictive trees.
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(string(paths[i]), string(filepath.Separator)) < strings.Count(string(paths[j]), string(filepath.Separator))
	})
	var cleanupErrs []error
	for _, directoryPath := range paths {
		if err := os.Chmod(string(directoryPath), directories[directoryPath]|0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to restore cleanup access for %s: %w", directoryPath, err))
		}
	}
	if err := os.RemoveAll(string(modulePath)); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to remove partial module: %w", err))
	}
	return errors.Join(cleanupErrs...)
}

func cleanupUnpackFailure(modulePath types.FilesystemPath, directories map[types.FilesystemPath]os.FileMode, cause error) error {
	cleanupErr := cleanupExtractedModule(modulePath, directories)
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(cause, fmt.Errorf("failed to clean up rejected module: %w", cleanupErr))
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func extractSingleEntry(file *zip.File, cleanPath, absDestDir string) error {
	destPath := filepath.Join(absDestDir, filepath.FromSlash(cleanPath))
	if err := validateDestinationPath(absDestDir, destPath, file.Name); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	if err := extractFile(file, destPath); err != nil {
		return fmt.Errorf("failed to extract %s: %w", file.Name, err)
	}
	return nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func validateDestinationPath(root, candidate, value string) error {
	relPath, err := filepath.Rel(root, candidate)
	if err != nil ||
		relPath == ".." ||
		strings.HasPrefix(relPath, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relPath) {
		return fmt.Errorf("%w: %s", ErrInvalidZIPPath, value)
	}
	return nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native module paths.
func validateExtractedModule(modulePath types.FilesystemPath) (*invowkmod.Module, error) {
	if err := modulePath.Validate(); err != nil {
		return nil, fmt.Errorf("extracted module path: %w", err)
	}
	mod, err := invowkmod.Load(modulePath)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

// downloadFile downloads a file from a URL and returns the path to the temporary file.
// The context controls cancellation and timeout for the HTTP request.
func (f DefaultArchiveSourceFetcher) downloadFile(ctx context.Context, url string) (_ types.FilesystemPath, err error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "invowk-module-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpPath := types.FilesystemPath(tmpFilePath)
	if validateErr := tmpPath.Validate(); validateErr != nil {
		return "", fmt.Errorf("temporary archive path: %w", validateErr)
	}

	// Clean up temp file on any error
	defer func() {
		if err != nil {
			_ = os.Remove(tmpFilePath) // Best-effort cleanup
		}
	}()

	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Download the file
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := f.httpClient().Do(req)
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
