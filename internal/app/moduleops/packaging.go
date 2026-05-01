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
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
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

	// DefaultArchiveSourceFetcher resolves local paths and HTTP(S) URLs.
	DefaultArchiveSourceFetcher struct {
		HTTPClient *http.Client
	}
)

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
// Returns the path to the extracted module or an error.
func Unpack(ctx context.Context, opts UnpackOptions) (extractedPath string, err error) {
	if opts.Source == "" {
		return "", errors.New("source cannot be empty")
	}

	absDestDir, err := resolveUnpackDestination(opts.DestDir)
	if err != nil {
		return "", err
	}

	sourceFetcher := opts.SourceFetcher
	if sourceFetcher == nil {
		sourceFetcher = DefaultArchiveSourceFetcher{}
	}
	zipPath, cleanup, err := sourceFetcher.FetchArchiveSource(ctx, opts.Source)
	if err != nil {
		return "", err
	}
	defer cleanup()

	zipReader, err := zip.OpenReader(string(zipPath))
	if err != nil {
		return "", fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer func() {
		if closeErr := zipReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	moduleRoot, err := findModuleRoot(zipReader.File)
	if err != nil {
		return "", err
	}

	modulePath, err := prepareModuleDestination(absDestDir, moduleRoot, opts.Overwrite)
	if err != nil {
		return "", err
	}

	if err := extractModuleFiles(zipReader.File, moduleRoot, absDestDir); err != nil {
		return "", err
	}

	if err := validateExtractedModule(modulePath); err != nil {
		_ = os.RemoveAll(modulePath)
		return "", fmt.Errorf("extracted module is invalid: %w", err)
	}

	return modulePath, nil
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
func extractModuleFiles(files []*zip.File, moduleRoot, absDestDir string) error {
	for _, file := range files {
		cleanPath, err := normalizeZIPPath(file.Name)
		if err != nil {
			return err
		}
		if cleanPath != moduleRoot && !strings.HasPrefix(cleanPath, moduleRoot+"/") {
			continue
		}
		if err := extractSingleEntry(file, cleanPath, absDestDir); err != nil {
			return err
		}
	}
	return nil
}

//goplint:ignore -- unpack helpers operate on transient OS-native and ZIP path strings.
func extractSingleEntry(file *zip.File, cleanPath, absDestDir string) error {
	destPath := filepath.Join(absDestDir, filepath.FromSlash(cleanPath))
	if err := validateDestinationPath(absDestDir, destPath, file.Name); err != nil {
		return err
	}

	if file.FileInfo().IsDir() {
		if err := os.MkdirAll(destPath, file.Mode()); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		return nil
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
func validateExtractedModule(modulePath string) error {
	modLoadPath := types.FilesystemPath(modulePath)
	if err := modLoadPath.Validate(); err != nil {
		return fmt.Errorf("extracted module path: %w", err)
	}
	if _, err := invowkmod.Load(modLoadPath); err != nil {
		return err
	}
	return nil
}

// downloadFile downloads a file from a URL and returns the path to the temporary file.
// The context controls cancellation and timeout for the HTTP request.
func (f DefaultArchiveSourceFetcher) downloadFile(ctx context.Context, url string) (tmpPath types.FilesystemPath, err error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "invowk-module-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath = types.FilesystemPath(tmpFile.Name()) //goplint:ignore -- validated before return.
	if validateErr := tmpPath.Validate(); validateErr != nil {
		return "", fmt.Errorf("temporary archive path: %w", validateErr)
	}

	// Clean up temp file on any error
	defer func() {
		if err != nil {
			_ = os.Remove(string(tmpPath)) // Best-effort cleanup
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
