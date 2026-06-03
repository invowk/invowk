// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

const verifyMutationOtherHash ContentHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func TestVerifyMutationValidationContracts(t *testing.T) {
	t.Parallel()

	t.Run("ambiguity validates module id and every key", func(t *testing.T) {
		t.Parallel()

		err := LockedModuleAmbiguity{
			ModuleID: " ",
			LockKeys: []ModuleRefKey{
				"https://github.com/example/dep.git",
				"",
			},
		}.Validate()
		if !errors.Is(err, ErrInvalidModuleID) {
			t.Fatalf("LockedModuleAmbiguity.Validate() error = %v, want ErrInvalidModuleID", err)
		}
		if !errors.Is(err, ErrInvalidModuleRefKey) {
			t.Fatalf("LockedModuleAmbiguity.Validate() error = %v, want ErrInvalidModuleRefKey", err)
		}
	})

	t.Run("ambiguity requires two lock keys", func(t *testing.T) {
		t.Parallel()

		err := LockedModuleAmbiguity{
			ModuleID: "io.example.dep",
			LockKeys: []ModuleRefKey{
				"https://github.com/example/dep.git",
			},
		}.Validate()
		if err == nil {
			t.Fatal("LockedModuleAmbiguity.Validate() error = nil, want too-few-keys error")
		}
		if errors.Is(err, ErrInvalidModuleID) || errors.Is(err, ErrInvalidModuleRefKey) {
			t.Fatalf("LockedModuleAmbiguity.Validate() error = %v, want only too-few-keys error", err)
		}
	})
}

func TestVerifyMutationEvaluationMissingModulePayloads(t *testing.T) {
	t.Parallel()

	for _, mod := range []*Module{nil, {}} {
		evaluation := EvaluateVendoredModuleHash(NewLockFile(), mod)
		if evaluation.Status != VendoredHashMissing {
			t.Fatalf("EvaluateVendoredModuleHash() status = %q, want %q", evaluation.Status, VendoredHashMissing)
		}
	}
}

func TestVerifyMutationEvaluationMissingLockEntryPayload(t *testing.T) {
	t.Parallel()

	mod := &Module{Metadata: &Invowkmod{Module: "io.example.dep"}}
	evaluation := EvaluateVendoredModuleHash(NewLockFile(), mod)
	if evaluation.Status != VendoredHashMissing {
		t.Fatalf("status = %q, want %q", evaluation.Status, VendoredHashMissing)
	}
	if evaluation.ModuleID != "io.example.dep" {
		t.Fatalf("ModuleID = %q, want io.example.dep", evaluation.ModuleID)
	}
}

func TestVerifyMutationEvaluationAmbiguousLockEntryPayload(t *testing.T) {
	t.Parallel()

	mod := &Module{Metadata: &Invowkmod{Module: "io.example.dep"}}
	lock := NewLockFile()
	lock.Modules["https://github.com/example/dep.git"] = lockedHashTestModule("io.example.dep", verifyMutationOtherHash)
	lock.Modules["https://github.com/example/alias.git"] = lockedHashTestModule("io.example.dep", verifyMutationOtherHash)

	evaluation := EvaluateVendoredModuleHash(lock, mod)
	if evaluation.Status != VendoredHashAmbiguous {
		t.Fatalf("status = %q, want %q", evaluation.Status, VendoredHashAmbiguous)
	}
	got := append([]ModuleRefKey(nil), evaluation.LockKeys...)
	slices.Sort(got)
	want := []ModuleRefKey{"https://github.com/example/alias.git", "https://github.com/example/dep.git"}
	if !slices.Equal(got, want) {
		t.Fatalf("LockKeys = %v, want %v", got, want)
	}
}

func TestVerifyMutationEvaluationUnavailableContentHashPayload(t *testing.T) {
	t.Parallel()

	evaluation := EvaluateModuleContentHash("https://github.com/example/dep.git", "io.example.dep", "", "")
	if evaluation.Status != VendoredHashUnavailable {
		t.Fatalf("Status = %q, want %q", evaluation.Status, VendoredHashUnavailable)
	}
	if evaluation.ModuleID != "io.example.dep" {
		t.Fatalf("ModuleID = %q, want io.example.dep", evaluation.ModuleID)
	}
	if evaluation.ModuleKey != "https://github.com/example/dep.git" {
		t.Fatalf("ModuleKey = %q, want dependency key", evaluation.ModuleKey)
	}
	if evaluation.Expected != "" || evaluation.Actual != "" || evaluation.Err != nil {
		t.Fatalf("evaluation = %+v, want empty hashes and nil error", evaluation)
	}
}

func TestVerifyMutationEvaluationMatchedContentHashPayload(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	writeHashTestModule(t, moduleDir, "io.example.dep")
	hash, err := computeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("computeModuleHash() error = %v", err)
	}

	evaluation := EvaluateModuleContentHash("https://github.com/example/dep.git", "io.example.dep", types.FilesystemPath(moduleDir), hash)
	if evaluation.Status != VendoredHashMatched {
		t.Fatalf("Status = %q, want %q", evaluation.Status, VendoredHashMatched)
	}
	if evaluation.Expected != hash || evaluation.Actual != hash {
		t.Fatalf("hashes = expected %q actual %q, want both %q", evaluation.Expected, evaluation.Actual, hash)
	}
	if evaluation.ModuleID != "io.example.dep" || evaluation.ModuleKey != "https://github.com/example/dep.git" {
		t.Fatalf("identity fields = module %q key %q", evaluation.ModuleID, evaluation.ModuleKey)
	}
}

func TestVerifyMutationVendoredHashVerificationSkipsEmptyContentHashLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, VendoredModulesDir, "io.example.dep"+ModuleSuffix)
	writeHashTestModule(t, moduleDir, "io.example.dep")
	if err := NewLockFile().Save(filepath.Join(root, LockFileName)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := VerifyVendoredModuleHashes(types.FilesystemPath(root)); err != nil {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want nil for empty content hashes", err)
	}
}

func TestVerifyMutationVendoredHashVerificationRejectsMalformedLockFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, VendoredModulesDir), 0o755); err != nil {
		t.Fatalf("mkdir vendor dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, LockFileName), []byte("not: [valid"), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	err := VerifyVendoredModuleHashes(types.FilesystemPath(root))
	if err == nil {
		t.Fatal("VerifyVendoredModuleHashes() error = nil, want malformed lock error")
	}
	if !strings.Contains(err.Error(), "loading lock file for hash verification") {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want loading context", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want wrapped load error", err)
	}
}

func TestVerifyMutationVendoredHashVerificationIgnoresSuffixlessDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ignoredDir := filepath.Join(root, VendoredModulesDir, "io.example.dep")
	writeHashTestModule(t, ignoredDir, "io.example.dep")
	lock := NewLockFile()
	lock.Modules["https://github.com/example/other.git"] = lockedHashTestModule("io.example.other", verifyMutationOtherHash)
	if err := lock.Save(filepath.Join(root, LockFileName)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := VerifyVendoredModuleHashes(types.FilesystemPath(root)); err != nil {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want suffixless directory ignored", err)
	}
}

func TestVerifyMutationVendoredHashVerificationContinuesAfterIgnoredEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vendorDir := filepath.Join(root, VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatalf("mkdir vendor dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "00-ignore.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write ignored vendor entry: %v", err)
	}
	moduleDir := filepath.Join(vendorDir, "zz.example.dep"+ModuleSuffix)
	writeHashTestModule(t, moduleDir, "zz.example.dep")
	lock := NewLockFile()
	lock.Modules["https://github.com/example/dep.git"] = lockedHashTestModule("zz.example.dep", verifyMutationOtherHash)
	if err := lock.Save(filepath.Join(root, LockFileName)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := VerifyVendoredModuleHashes(types.FilesystemPath(root))
	if !errors.Is(err, ErrContentHashMismatch) {
		t.Fatalf("VerifyVendoredModuleHashes() error = %v, want ErrContentHashMismatch", err)
	}
}

func TestVerifyMutationAmbiguousErrorListsKeys(t *testing.T) {
	t.Parallel()

	err := verifyVendoredModuleHashEvaluation("io.example.dep", VendoredHashEvaluation{
		Status: VendoredHashAmbiguous,
		LockKeys: []ModuleRefKey{
			"https://github.com/example/dep.git",
			"https://github.com/example/alias.git",
		},
	})
	if err == nil {
		t.Fatal("verifyVendoredModuleHashEvaluation() error = nil, want ambiguous error")
	}
	want := "https://github.com/example/dep.git, https://github.com/example/alias.git"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want joined lock keys %q", err.Error(), want)
	}
}

func TestVerifyMutationMismatchErrorPreservesHashFields(t *testing.T) {
	t.Parallel()

	err := verifyVendoredModuleHashEvaluation("io.example.dep", VendoredHashEvaluation{
		Status:    VendoredHashMismatch,
		ModuleKey: "https://github.com/example/dep.git",
		Expected:  verifyMutationOtherHash,
		Actual:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
	})
	if !errors.Is(err, ErrContentHashMismatch) {
		t.Fatalf("verifyVendoredModuleHashEvaluation() error = %v, want ErrContentHashMismatch", err)
	}
	var mismatch *ContentHashMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error type = %T, want *ContentHashMismatchError", err)
	}
	if mismatch.ModuleKey != "https://github.com/example/dep.git" {
		t.Fatalf("ModuleKey = %q, want dependency key", mismatch.ModuleKey)
	}
	if mismatch.Expected != verifyMutationOtherHash {
		t.Fatalf("Expected = %q, want %q", mismatch.Expected, verifyMutationOtherHash)
	}
	if mismatch.Actual != "sha256:1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("Actual = %q, want preserved actual hash", mismatch.Actual)
	}
}
