// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChecksums_ValidFile(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2  invowk_1.1.0_linux_amd64.tar.gz\n" +
			"f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8  invowk_1.1.0_darwin_amd64.tar.gz\n" +
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  invowk_1.1.0_windows_amd64.zip\n",
	)

	entries, err := ParseChecksums(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	tests := []struct {
		index    int
		wantHash string
		wantFile string
	}{
		{0, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "invowk_1.1.0_linux_amd64.tar.gz"},
		{1, "f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8b9c0d1e2f7a8", "invowk_1.1.0_darwin_amd64.tar.gz"},
		{2, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "invowk_1.1.0_windows_amd64.zip"},
	}

	for _, tt := range tests {
		if entries[tt.index].Hash != tt.wantHash {
			t.Errorf("entry[%d].Hash = %q, want %q", tt.index, entries[tt.index].Hash, tt.wantHash)
		}
		if entries[tt.index].Filename != tt.wantFile {
			t.Errorf("entry[%d].Filename = %q, want %q", tt.index, entries[tt.index].Filename, tt.wantFile)
		}
	}
}

func TestParseChecksums_SkipsEmptyAndInvalid(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(
		// Valid entry.
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2  invowk_linux.tar.gz\n" +
			// Empty line.
			"\n" +
			// Wrong hash length (too short).
			"abcdef1234  some_file.tar.gz\n" +
			// Single space separator instead of double.
			"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2 single_space.tar.gz\n" +
			// Non-hex characters in hash.
			"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  bad_hex.tar.gz\n" +
			// Missing filename (hash only with double space).
			"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2  \n" +
			// Another valid entry.
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  invowk_darwin.tar.gz\n",
	)

	entries, err := ParseChecksums(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	if entries[0].Filename != "invowk_linux.tar.gz" {
		t.Errorf("entries[0].Filename = %q, want %q", entries[0].Filename, "invowk_linux.tar.gz")
	}
	if entries[1].Filename != "invowk_darwin.tar.gz" {
		t.Errorf("entries[1].Filename = %q, want %q", entries[1].Filename, "invowk_darwin.tar.gz")
	}
}

func TestParseChecksums_NoValidEntries(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(
		"tooshort  file.tar.gz\n" +
			"not-a-valid-line\n" +
			"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  bad_hex.tar.gz\n",
	)

	_, err := ParseChecksums(input)
	if err == nil {
		t.Fatal("expected error for no valid entries, got nil")
	}
}

func TestParseChecksums_EmptyReader(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("")

	_, err := ParseChecksums(input)
	if err == nil {
		t.Fatal("expected error for empty reader, got nil")
	}
}

func TestFindChecksum_Found(t *testing.T) {
	t.Parallel()

	entries := []ChecksumEntry{
		{Hash: "aaaa000000000000000000000000000000000000000000000000000000000001", Filename: "invowk_linux_amd64.tar.gz"},
		{Hash: "bbbb000000000000000000000000000000000000000000000000000000000002", Filename: "invowk_darwin_amd64.tar.gz"},
		{Hash: "cccc000000000000000000000000000000000000000000000000000000000003", Filename: "invowk_windows_amd64.zip"},
	}

	hash, err := FindChecksum(entries, "invowk_darwin_amd64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "bbbb000000000000000000000000000000000000000000000000000000000002"
	if hash != want {
		t.Errorf("got hash %q, want %q", hash, want)
	}
}

func TestFindChecksum_NotFound(t *testing.T) {
	t.Parallel()

	entries := []ChecksumEntry{
		{Hash: "aaaa000000000000000000000000000000000000000000000000000000000001", Filename: "invowk_linux_amd64.tar.gz"},
	}

	_, err := FindChecksum(entries, "invowk_freebsd_amd64.tar.gz")
	if !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("got error %v, want ErrAssetNotFound", err)
	}
}

func TestVerifyFile_Match(t *testing.T) {
	t.Parallel()

	// SHA256("hello\n") = 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	expectedHash := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

	if err := VerifyFile(path, expectedHash); err != nil {
		t.Errorf("expected nil error for matching hash, got: %v", err)
	}
}

func TestVerifyFile_Mismatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

	err := VerifyFile(path, wrongHash)
	if err == nil {
		t.Fatal("expected error for mismatched hash, got nil")
	}

	// Verify errors.Is works with the sentinel error through Unwrap.
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("errors.Is(err, ErrChecksumMismatch) = false, want true; err = %v", err)
	}

	// Verify the concrete ChecksumError type carries both hashes.
	var checksumErr *ChecksumError
	if !errors.As(err, &checksumErr) {
		t.Fatalf("expected *ChecksumError, got %T", err)
	}

	if checksumErr.Expected != wrongHash {
		t.Errorf("ChecksumError.Expected = %q, want %q", checksumErr.Expected, wrongHash)
	}

	wantGot := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if checksumErr.Got != wantGot {
		t.Errorf("ChecksumError.Got = %q, want %q", checksumErr.Got, wantGot)
	}
}

func TestVerifyFile_FileNotFound(t *testing.T) {
	t.Parallel()

	err := VerifyFile("/nonexistent/path/to/file.tar.gz", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}

	// Should be an os-level error, not a checksum mismatch.
	if errors.Is(err, ErrChecksumMismatch) {
		t.Error("expected non-checksum error for missing file, got ErrChecksumMismatch")
	}
}

func TestComputeFileHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  []byte
		wantHash string
	}{
		{
			name:     "hello newline",
			content:  []byte("hello\n"),
			wantHash: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		},
		{
			name:     "empty file",
			content:  []byte(""),
			wantHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "testfile")
			if err := os.WriteFile(path, tt.content, 0o644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			got, err := ComputeFileHash(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.wantHash {
				t.Errorf("got hash %q, want %q", got, tt.wantHash)
			}
		})
	}
}

func TestIsValidHexHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid lowercase",
			input: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			want:  true,
		},
		{
			name:  "valid uppercase",
			input: "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2",
			want:  true,
		},
		{
			name:  "valid mixed case",
			input: "A1b2C3d4E5f6A1b2C3d4E5f6A1b2C3d4E5f6A1b2C3d4E5f6A1b2C3d4E5f6A1b2",
			want:  true,
		},
		{
			name:  "valid all zeros",
			input: "0000000000000000000000000000000000000000000000000000000000000000",
			want:  true,
		},
		{
			name:  "too short",
			input: "abcdef1234",
			want:  false,
		},
		{
			name:  "too long",
			input: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b200",
			want:  false,
		},
		{
			name:  "non-hex characters",
			input: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "63 chars (one short)",
			input: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b",
			want:  false,
		},
		{
			name:  "contains space",
			input: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4 5f6a1b2c3d4e5f6a1b2",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isValidHexHash(tt.input)
			if got != tt.want {
				t.Errorf("isValidHexHash(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestVerifyFile_CaseInsensitive verifies that hash comparison is case-insensitive,
// allowing uppercase expected hashes to match lowercase computed hashes.
func TestVerifyFile_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Uppercase version of the SHA256 of "hello\n".
	upperHash := "5891B5B522D5DF086D0FF0B110FBD9D21BB4FC7163AF34D08286A2E846F6BE03"

	if err := VerifyFile(path, upperHash); err != nil {
		t.Errorf("expected nil error for case-insensitive match, got: %v", err)
	}
}
