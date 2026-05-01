// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestContentHashValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hash    ContentHash
		wantErr bool
	}{
		{
			name:    "valid hash",
			hash:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: false,
		},
		{
			name:    "empty string",
			hash:    "",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			hash:    "md5:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: true,
		},
		{
			name:    "too short hex",
			hash:    "sha256:abc123",
			wantErr: true,
		},
		{
			name:    "uppercase hex",
			hash:    "sha256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",
			wantErr: true,
		},
		{
			name:    "too long hex",
			hash:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855aa",
			wantErr: true,
		},
		{
			name:    "no prefix",
			hash:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: true,
		},
		{
			name:    "invalid hex char",
			hash:    "sha256:g3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.hash.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.Is(err, ErrInvalidContentHash) {
					t.Errorf("expected ErrInvalidContentHash, got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestContentHashString(t *testing.T) {
	t.Parallel()

	h := ContentHash("sha256:abc123")
	if h.String() != "sha256:abc123" {
		t.Errorf("String() = %q, want %q", h.String(), "sha256:abc123")
	}
}

func TestInvalidContentHashError(t *testing.T) {
	t.Parallel()

	err := &InvalidContentHashError{Value: "bad"}
	if !errors.Is(err, ErrInvalidContentHash) {
		t.Error("expected errors.Is to match ErrInvalidContentHash")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestContentHashMismatchError(t *testing.T) {
	t.Parallel()

	err := &ContentHashMismatchError{
		ModuleKey: "https://github.com/user/repo.git",
		Expected:  "sha256:aaaa",
		Actual:    "sha256:bbbb",
	}
	if !errors.Is(err, ErrContentHashMismatch) {
		t.Error("expected errors.Is to match ErrContentHashMismatch")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestComputeModuleHash_Determinism(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create some test files in a predictable structure.
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte("module: \"test\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte("cmds: []\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "script.sh"), []byte("#!/bin/sh\necho hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Compute twice — must be identical.
	hash1, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("first computeModuleHash() error = %v", err)
	}
	hash2, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("second computeModuleHash() error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hashes not deterministic: %s != %s", hash1, hash2)
	}

	// Validate the format.
	if err := hash1.Validate(); err != nil {
		t.Errorf("computed hash failed validation: %v", err)
	}
}

func TestComputeModuleHash_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	hash, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}

	// Empty dir should produce a valid hash (SHA-256 of empty input).
	if err := hash.Validate(); err != nil {
		t.Errorf("empty dir hash failed validation: %v", err)
	}
}

func TestComputeModuleHash_SkipsSymlinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a regular file.
	regularFile := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Compute hash with only the regular file.
	hashBefore, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("computeModuleHash() before symlink error = %v", err)
	}

	// Create a symlink — this should be ignored by the hash.
	symlinkPath := filepath.Join(dir, "link.txt")
	if symlinkErr := os.Symlink(regularFile, symlinkPath); symlinkErr != nil {
		t.Skipf("skipping symlink test: %v", symlinkErr)
	}

	hashAfter, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("computeModuleHash() after symlink error = %v", err)
	}

	if hashBefore != hashAfter {
		t.Errorf("symlink affected hash: before=%s, after=%s", hashBefore, hashAfter)
	}
}

func TestComputeModuleHash_ContentChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	file := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(file, []byte("version 1"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	hash1, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("computeModuleHash() v1 error = %v", err)
	}

	// Change the file content — hash must differ.
	if writeErr := os.WriteFile(file, []byte("version 2"), 0o644); writeErr != nil {
		t.Fatalf("failed to overwrite file: %v", writeErr)
	}

	hash2, err := computeModuleHash(dir)
	if err != nil {
		t.Fatalf("computeModuleHash() v2 error = %v", err)
	}

	if hash1 == hash2 {
		t.Error("hashes should differ after content change")
	}
}

func TestVerifyVendoredModuleHashesUsesModuleIDForAliasedModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vendorDir := filepath.Join(root, VendoredModulesDir)
	moduleDir := filepath.Join(vendorDir, "io.example.dep"+ModuleSuffix)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create vendored module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "io.example.dep"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte("cmds: []\n"), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	hash, err := computeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}

	lock := NewLockFile()
	lock.Modules["https://github.com/example/dep.invowkmod.git"] = LockedModule{
		GitURL:          "https://github.com/example/dep.invowkmod.git",
		Version:         "1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Alias:           "tools",
		Namespace:       "tools",
		ModuleID:        "io.example.dep",
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(root, LockFileName)); saveErr != nil {
		t.Fatalf("Save() error = %v", saveErr)
	}

	if err := VerifyVendoredModuleHashes(types.FilesystemPath(root)); err != nil {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v", err)
	}
}

func TestEvaluateVendoredModuleHashAmbiguousLockEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "io.example.dep"+ModuleSuffix)
	writeHashTestModule(t, moduleDir, "io.example.dep")

	mod, err := Load(types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	hash, err := computeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}
	lock := NewLockFile()
	lock.Modules["https://github.com/example/dep.git"] = lockedHashTestModule("io.example.dep", hash)
	lock.Modules["https://github.com/example/alias.git"] = lockedHashTestModule("io.example.dep", hash)

	evaluation := EvaluateVendoredModuleHash(lock, mod)
	if evaluation.Status != VendoredHashAmbiguous {
		t.Fatalf("EvaluateVendoredModuleHash() status = %q, want %q", evaluation.Status, VendoredHashAmbiguous)
	}
	if len(evaluation.LockKeys) != 2 {
		t.Errorf("EvaluateVendoredModuleHash() lock key count = %d, want 2", len(evaluation.LockKeys))
	}
}

func TestVerifyVendoredModuleHashesRejectsAmbiguousLockEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vendorDir := filepath.Join(root, VendoredModulesDir)
	moduleDir := filepath.Join(vendorDir, "io.example.dep"+ModuleSuffix)
	writeHashTestModule(t, moduleDir, "io.example.dep")

	hash, err := computeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}
	lock := NewLockFile()
	lock.Modules["https://github.com/example/dep.git"] = lockedHashTestModule("io.example.dep", hash)
	lock.Modules["https://github.com/example/alias.git"] = lockedHashTestModule("io.example.dep", hash)
	if saveErr := lock.Save(filepath.Join(root, LockFileName)); saveErr != nil {
		t.Fatalf("Save() error = %v", saveErr)
	}

	err = VerifyVendoredModuleHashes(types.FilesystemPath(root))
	if err == nil {
		t.Fatal("VerifyVendoredModuleHashes() returned nil, want ambiguous lock error")
	}
}

func TestVerifyVendoredModuleHashesRejectsMissingLockEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vendorDir := filepath.Join(root, VendoredModulesDir)
	moduleDir := filepath.Join(vendorDir, "io.example.extra"+ModuleSuffix)
	writeHashTestModule(t, moduleDir, "io.example.extra")

	hash, err := computeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}
	lock := NewLockFile()
	lock.Modules["https://github.com/example/other.git"] = lockedHashTestModule("io.example.other", hash)
	if saveErr := lock.Save(filepath.Join(root, LockFileName)); saveErr != nil {
		t.Fatalf("Save() error = %v", saveErr)
	}

	err = VerifyVendoredModuleHashes(types.FilesystemPath(root))
	if err == nil {
		t.Fatal("VerifyVendoredModuleHashes() returned nil, want missing lock entry error")
	}
	if !strings.Contains(err.Error(), "missing lock file entry") {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want missing lock entry", err)
	}
}

func TestComputeModuleHash_FileNameChanges(t *testing.T) {
	t.Parallel()

	// Two dirs with the same content but different file names — hashes should differ.
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir1, "alpha.txt"), []byte("same content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "beta.txt"), []byte("same content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	hash1, err := computeModuleHash(dir1)
	if err != nil {
		t.Fatalf("computeModuleHash(dir1) error = %v", err)
	}
	hash2, err := computeModuleHash(dir2)
	if err != nil {
		t.Fatalf("computeModuleHash(dir2) error = %v", err)
	}

	if hash1 == hash2 {
		t.Error("hashes should differ for different file names with same content")
	}
}

func writeHashTestModule(t *testing.T, moduleDir, moduleID string) {
	t.Helper()

	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "`+moduleID+`"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte("cmds: []\n"), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}
}

func lockedHashTestModule(moduleID string, hash ContentHash) LockedModule {
	return LockedModule{
		GitURL:          GitURL("https://github.com/example/" + moduleID + ".git"),
		Version:         "1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       ModuleNamespace(moduleID),
		ModuleID:        ModuleID(moduleID),
		ContentHash:     hash,
	}
}
