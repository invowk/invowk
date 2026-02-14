// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	// ErrChecksumMismatch indicates the computed SHA256 hash does not match the expected hash.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrAssetNotFound indicates the requested asset filename was not found in checksums.txt.
	ErrAssetNotFound = errors.New("asset not found in checksums")

	// errNoValidEntries indicates the checksums file contained no parseable entries.
	errNoValidEntries = errors.New("no valid checksum entries found")
)

type (
	// ChecksumEntry represents a SHA256 checksum for a release asset.
	ChecksumEntry struct {
		Hash     string // Hex-encoded SHA256 hash (64 characters)
		Filename string // Asset filename this hash applies to
	}

	// ChecksumError provides details about a checksum verification failure.
	// It wraps ErrChecksumMismatch so callers can use errors.Is for classification.
	ChecksumError struct {
		Filename string
		Expected string
		Got      string
	}
)

// Error returns a human-readable description of the checksum mismatch,
// showing both expected and actual hash values for debugging.
func (e *ChecksumError) Error() string {
	return fmt.Sprintf("checksum verification failed for %s\nExpected: %s\nGot:      %s", e.Filename, e.Expected, e.Got)
}

// Unwrap returns ErrChecksumMismatch so callers can use errors.Is.
func (e *ChecksumError) Unwrap() error { return ErrChecksumMismatch }

// ParseChecksums parses a checksums.txt file in the standard sha256sum output format.
// Each line is expected to be "{sha256_hex}  {filename}" (two spaces between hash
// and filename). Empty lines and lines that don't match the expected format are
// silently skipped. Returns an error if no valid entries are found.
func ParseChecksums(r io.Reader) ([]ChecksumEntry, error) {
	var entries []ChecksumEntry

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// The sha256sum format uses exactly two spaces between hash and filename.
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		filename := strings.TrimSpace(parts[1])

		if filename == "" || !isValidHexHash(hash) {
			continue
		}

		entries = append(entries, ChecksumEntry{
			Hash:     strings.ToLower(hash),
			Filename: filename,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading checksums: %w", err)
	}

	if len(entries) == 0 {
		return nil, errNoValidEntries
	}

	return entries, nil
}

// FindChecksum searches entries for the given filename and returns its hash.
// Returns ErrAssetNotFound if no entry matches the filename.
func FindChecksum(entries []ChecksumEntry, filename string) (string, error) {
	for _, e := range entries {
		if e.Filename == filename {
			return e.Hash, nil
		}
	}
	return "", ErrAssetNotFound
}

// VerifyFile computes the SHA256 hash of the file at path and compares it with
// expectedHash. Returns nil if the hashes match (case-insensitive comparison),
// or a *ChecksumError wrapping ErrChecksumMismatch if they differ.
func VerifyFile(path, expectedHash string) error {
	got, err := ComputeFileHash(path)
	if err != nil {
		return err
	}

	if !strings.EqualFold(got, expectedHash) {
		return &ChecksumError{
			Filename: path,
			Expected: strings.ToLower(expectedHash),
			Got:      got,
		}
	}

	return nil
}

// ComputeFileHash computes and returns the lowercase hex-encoded SHA256 digest
// of the file at path. It streams the file through the hash function to avoid
// loading the entire file into memory.
func ComputeFileHash(path string) (_ string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		// Read-only file handle; close errors are exotic (NFS edge cases).
		_ = f.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file %s: %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// isValidHexHash checks if s is a valid 64-character hex-encoded SHA256 hash.
func isValidHexHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
