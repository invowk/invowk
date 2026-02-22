// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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
		outputPath = string(m.Name()) + ModuleSuffix + ".zip"
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
		// Look for the .invowkmod directory
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
