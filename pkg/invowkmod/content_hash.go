// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

const (
	invalidContentHashErrMsg  = "invalid content hash"
	contentHashMismatchErrMsg = "content hash mismatch"
	contentHashPrefix         = "sha256:"
	contentHashExpectedHexLen = 64
)

var (
	// ErrInvalidContentHash is the sentinel error wrapped by InvalidContentHashError.
	ErrInvalidContentHash = errors.New(invalidContentHashErrMsg)
	// ErrContentHashMismatch is the sentinel error wrapped by ContentHashMismatchError.
	ErrContentHashMismatch = errors.New(contentHashMismatchErrMsg)

	// contentHashPattern validates a SHA-256 content hash in "sha256:<64 hex chars>" format.
	contentHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type (
	// ContentHash is a hex-encoded SHA-256 content hash of a cached module tree.
	// Format: "sha256:<64-hex-chars>". Used for tamper detection of vendored modules.
	ContentHash string

	// InvalidContentHashError is returned when a ContentHash value does not match
	// the expected "sha256:<64-hex-chars>" format.
	InvalidContentHashError struct {
		Value ContentHash
	}

	// ContentHashMismatchError is returned when a cached module's content hash
	// does not match the expected hash from the lock file.
	ContentHashMismatchError struct {
		ModuleKey ModuleRefKey
		Expected  ContentHash
		Actual    ContentHash
	}
)

// Error implements the error interface.
func (e *InvalidContentHashError) Error() string {
	return fmt.Sprintf("invalid content hash %q (must be sha256:<64 hex chars>)", e.Value)
}

// Unwrap returns ErrInvalidContentHash so callers can use errors.Is for programmatic detection.
func (e *InvalidContentHashError) Unwrap() error { return ErrInvalidContentHash }

// Error implements the error interface.
func (e *ContentHashMismatchError) Error() string {
	return fmt.Sprintf("content hash mismatch for module %q: expected %s, got %s", e.ModuleKey, e.Expected, e.Actual)
}

// Unwrap returns ErrContentHashMismatch so callers can use errors.Is for programmatic detection.
func (e *ContentHashMismatchError) Unwrap() error { return ErrContentHashMismatch }

//goplint:nonzero

// Validate returns nil if the ContentHash is a valid SHA-256 content hash,
// or an error describing the validation failure.
// A valid content hash has the format "sha256:<64 lowercase hex characters>".
func (h ContentHash) Validate() error {
	if !contentHashPattern.MatchString(string(h)) {
		return &InvalidContentHashError{Value: h}
	}
	return nil
}

// String returns the string representation of the ContentHash.
func (h ContentHash) String() string { return string(h) }

// ComputeModuleHash computes a deterministic SHA-256 hash of a module directory tree.
// This is the exported accessor for computeModuleHash, enabling use by the
// audit scanner (internal/audit/) which cannot access unexported functions.
//
//goplint:ignore -- exported adapter accepts OS-resolved path text for the internal hashing helper.
func ComputeModuleHash(dir string) (ContentHash, error) {
	return computeModuleHash(dir)
}

// computeModuleHash computes a deterministic SHA-256 hash of a module directory tree.
// Files are walked in sorted lexicographic order by their relative path to ensure
// reproducibility across platforms. Each file contributes its relative path (as bytes)
// followed by its content. Symlinks and non-regular files are skipped.
// Returns a ContentHash in "sha256:<hex>" format.
func computeModuleHash(dir string) (ContentHash, error) {
	var relPaths []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip symlinks and non-regular files (consistent with copyDir security policy).
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}
		// Normalize to forward slashes for cross-platform determinism.
		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking module directory %s: %w", dir, err)
	}

	// Sort for deterministic ordering regardless of filesystem walk order.
	sort.Strings(relPaths)

	hasher := sha256.New()

	for _, rel := range relPaths {
		// Write relative path as a length-prefixed string to avoid ambiguity
		// between path and content boundaries.
		fmt.Fprintf(hasher, "%d:%s\n", len(rel), rel)

		absPath := filepath.Join(dir, filepath.FromSlash(rel))
		if err := hashFileContent(hasher, absPath); err != nil {
			return "", fmt.Errorf("hashing file %s: %w", rel, err)
		}
	}

	sum := hasher.Sum(nil)
	hash := ContentHash(contentHashPrefix + hex.EncodeToString(sum)) //goplint:ignore -- constructed from sha256 output, format is guaranteed valid
	return hash, nil
}

// hashFileContent streams a file's content into the hasher.
// Uses Lstat before Open and fstat after Open to close the TOCTOU window
// between the WalkDir entry check and the actual file read (L-01). This
// prevents a symlink race where a regular file is swapped for a symlink
// after the walk reports it as regular but before the Open call resolves it.
func hashFileContent(hasher io.Writer, path string) (err error) {
	// Defense-in-depth: Lstat confirms the path is still a regular file.
	info, lstatErr := os.Lstat(path)
	if lstatErr != nil {
		return fmt.Errorf("lstat %s: %w", path, lstatErr)
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Final check: verify the opened fd is still a regular file (closes
	// the remaining Lstat-to-Open TOCTOU window).
	fInfo, statErr := f.Stat()
	if statErr != nil {
		return fmt.Errorf("fstat %s: %w", path, statErr)
	}
	if !fInfo.Mode().IsRegular() {
		return nil
	}

	_, err = io.Copy(hasher, f)
	return err
}
