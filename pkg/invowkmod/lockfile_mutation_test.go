// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cueerrors "cuelang.org/go/cue/errors"

	"github.com/invowk/invowk/pkg/types"
)

const (
	lockMutationGitURL          GitURL           = "https://github.com/example/tools.git"
	lockMutationVersion         SemVerConstraint = "^1.0.0"
	lockMutationResolvedVersion SemVer           = "1.2.3"
	lockMutationGitCommit       GitCommit        = "0123456789abcdef0123456789abcdef01234567"
	lockMutationAlias           ModuleAlias      = "tools"
	lockMutationPath            SubdirectoryPath = "modules/tools"
	lockMutationNamespace       ModuleNamespace  = "tools"
	lockMutationCommandSource   ModuleSourceID   = "tools"
	lockMutationModuleID        ModuleID         = "io.example.tools"
)

func TestLockFileMutationValueErrorPayloads(t *testing.T) {
	t.Parallel()

	nsErr := requireInvalidModuleNamespaceError(t, ModuleNamespace("").Validate())
	if nsErr.Value != "" {
		t.Fatalf("InvalidModuleNamespaceError.Value = %q, want empty namespace", nsErr.Value)
	}

	versionErr := requireInvalidLockFileVersionError(t, LockFileVersion("9.9").Validate())
	if versionErr.Value != "9.9" {
		t.Fatalf("InvalidLockFileVersionError.Value = %q, want 9.9", versionErr.Value)
	}

	refErr := requireInvalidModuleRefKeyError(t, ModuleRefKey(" \t").Validate())
	if refErr.Value != " \t" {
		t.Fatalf("InvalidModuleRefKeyError.Value = %q, want original whitespace key", refErr.Value)
	}
}

func TestLockFileMutationLockedModuleFieldValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*LockedModule)
		want     error
		wantText string
	}{
		{
			name:   "git url",
			mutate: func(mod *LockedModule) { mod.GitURL = "not-a-url" },
			want:   ErrInvalidGitURL,
		},
		{
			name:   "version",
			mutate: func(mod *LockedModule) { mod.Version = "not-a-version" },
			want:   ErrInvalidSemVerConstraint,
		},
		{
			name:   "resolved version",
			mutate: func(mod *LockedModule) { mod.ResolvedVersion = "not-a-version" },
			want:   ErrInvalidSemVer,
		},
		{
			name:   "git commit",
			mutate: func(mod *LockedModule) { mod.GitCommit = "abc123" },
			want:   ErrInvalidGitCommit,
		},
		{
			name:   "alias",
			mutate: func(mod *LockedModule) { mod.Alias = "1bad" },
			want:   ErrInvalidModuleAlias,
		},
		{
			name:   "path",
			mutate: func(mod *LockedModule) { mod.Path = "../outside" },
			want:   ErrInvalidSubdirectoryPath,
		},
		{
			name:   "namespace",
			mutate: func(mod *LockedModule) { mod.Namespace = "" },
			want:   ErrInvalidModuleNamespace,
		},
		{
			name:     "command source id",
			mutate:   func(mod *LockedModule) { mod.CommandSourceID = "1bad" },
			wantText: "module source ID",
		},
		{
			name:   "module id",
			mutate: func(mod *LockedModule) { mod.ModuleID = "1bad" },
			want:   ErrInvalidModuleID,
		},
		{
			name:   "content hash",
			mutate: func(mod *LockedModule) { mod.ContentHash = "sha256:bad" },
			want:   ErrInvalidContentHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mod := validLockMutationLockedModule()
			tt.mutate(&mod)

			lockedErr := requireInvalidLockedModuleError(t, mod.Validate())
			if len(lockedErr.FieldErrors) != 1 {
				t.Fatalf("FieldErrors = %v, want exactly one delegated field error", lockedErr.FieldErrors)
			}
			fieldErr := lockedErr.FieldErrors[0]
			if tt.want != nil && !errors.Is(fieldErr, tt.want) {
				t.Fatalf("field error = %v, want %v", fieldErr, tt.want)
			}
			if tt.wantText != "" && !strings.Contains(fieldErr.Error(), tt.wantText) {
				t.Fatalf("field error = %v, want text %q", fieldErr, tt.wantText)
			}
		})
	}
}

func TestLockFileMutationErrorMessagesIncludeModuleKey(t *testing.T) {
	t.Parallel()

	err := (&InvalidLockedModuleError{
		ModuleKey:   ModuleRefKey(lockMutationGitURL),
		FieldErrors: []error{ErrInvalidGitURL},
	}).Error()
	if !strings.Contains(err, `locked module "https://github.com/example/tools.git"`) {
		t.Fatalf("InvalidLockedModuleError.Error() = %q, want module key context", err)
	}
}

func TestLockFileMutationInspectSizeBoundary(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), LockFileName)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", LockFileSizeLimit)), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	snapshot := InspectLockFile(types.FilesystemPath(path))
	if !snapshot.Present {
		t.Fatal("Present = false, want true at size boundary")
	}
	if snapshot.Size != LockFileSizeLimit {
		t.Fatalf("Size = %d, want %d", snapshot.Size, LockFileSizeLimit)
	}
	if snapshot.ParseErr == nil {
		t.Fatal("ParseErr = nil, want parse error for boundary-sized invalid content")
	}
	if strings.Contains(snapshot.ParseErr.Error(), "exceeds maximum size") {
		t.Fatalf("ParseErr = %v, should parse rather than reject exact size boundary", snapshot.ParseErr)
	}
}

func TestLockFileMutationSaveAndValidationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "mkdir failure is wrapped before atomic write", run: func(t *testing.T) {
			t.Helper()

			blocker := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			err := NewLockFile().Save(filepath.Join(blocker, "child", LockFileName))
			if err == nil {
				t.Fatal("Save() error = nil, want parent directory failure")
			}
			if !strings.Contains(err.Error(), "failed to create directory") {
				t.Fatalf("Save() error = %v, want mkdir wrapper", err)
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("Save() error = %v, want wrapped path error", err)
			}
		}},

		{name: "invalid version fails before module validation", run: func(t *testing.T) {
			t.Helper()

			lock := NewLockFile()
			lock.Version = "9.9"
			lock.Modules[ModuleRefKey(lockMutationGitURL)] = validLockMutationLockedModule()
			if err := lock.validateForSave(); !errors.Is(err, ErrInvalidLockFileVersion) {
				t.Fatalf("validateForSave() error = %v, want ErrInvalidLockFileVersion", err)
			}
		}},

		{name: "invalid module key fails before module field validation", run: func(t *testing.T) {
			t.Helper()

			lock := NewLockFile()
			lock.Modules[ModuleRefKey(" \t")] = validLockMutationLockedModule()
			if err := lock.validateForSave(); !errors.Is(err, ErrInvalidModuleRefKey) {
				t.Fatalf("validateForSave() error = %v, want ErrInvalidModuleRefKey", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestLockFileMutationAddModuleCopiesIdentityFields(t *testing.T) {
	t.Parallel()

	lock := NewLockFile()
	resolved := &ResolvedModule{
		ModuleRef: ModuleRef{
			GitURL:  lockMutationGitURL,
			Version: lockMutationVersion,
			Alias:   lockMutationAlias,
			Path:    lockMutationPath,
		},
		ResolvedVersion: lockMutationResolvedVersion,
		GitCommit:       lockMutationGitCommit,
		Namespace:       lockMutationNamespace,
		CommandSourceID: lockMutationCommandSource,
		ModuleID:        lockMutationModuleID,
		ContentHash:     testContentHash,
	}

	lock.AddModule(resolved)

	got, ok := lock.GetModule(ModuleRefKey("https://github.com/example/tools.git#modules/tools"))
	if !ok {
		t.Fatal("GetModule() found = false, want copied resolved module")
	}
	if got.GitURL != resolved.ModuleRef.GitURL ||
		got.Version != resolved.ModuleRef.Version ||
		got.ResolvedVersion != resolved.ResolvedVersion ||
		got.GitCommit != resolved.GitCommit ||
		got.Alias != resolved.ModuleRef.Alias ||
		got.Path != resolved.ModuleRef.Path ||
		got.Namespace != resolved.Namespace ||
		got.CommandSourceID != resolved.CommandSourceID ||
		got.ModuleID != resolved.ModuleID ||
		got.ContentHash != resolved.ContentHash {
		t.Fatalf("locked module = %+v, want all identity fields copied from %+v", got, resolved)
	}
}

func TestLockFileMutationParseModuleKeyBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want ModuleRefKey
	}{
		{
			name: "leading space",
			line: `  "https://github.com/example/tools.git": {`,
			want: ModuleRefKey(lockMutationGitURL),
		},
		{
			name: "unterminated quoted key",
			line: `"https://github.com/example/tools.git`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseModuleKey(tt.line); got != tt.want {
				t.Fatalf("parseModuleKey(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestLockFileMutationParserErrorContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "unknown version preserves payload", run: testLockFileMutationUnknownVersionPayload},
		{name: "generated timestamp wraps parse error", run: testLockFileMutationGeneratedTimestampParseError},
		{name: "cue syntax errors wrap cue error", run: testLockFileMutationCUESyntaxError},
		{name: "non-concrete values fail validation before decode", run: testLockFileMutationNonConcreteValidation},
		{name: "decode errors are returned", run: testLockFileMutationDecodeErrors},
		{name: "v2 missing content hash preserves module key", run: testLockFileMutationMissingContentHashModuleKey},
		{name: "v2 split metadata errors preserve module key", run: testLockFileMutationSplitMetadataModuleKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testLockFileMutationUnknownVersionPayload(t *testing.T) {
	t.Helper()

	_, err := parseLockFile(`version: "99.0"
generated: "2025-01-15T10:30:00Z"
modules: {}`)
	if !errors.Is(err, ErrUnknownLockFileVersion) {
		t.Fatalf("parseLockFile() error = %v, want ErrUnknownLockFileVersion", err)
	}
	var versionErr *UnknownLockFileVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("parseLockFile() error type = %T, want *UnknownLockFileVersionError", err)
	}
	if versionErr.Version != "99.0" {
		t.Fatalf("UnknownLockFileVersionError.Version = %q, want 99.0", versionErr.Version)
	}
	if len(versionErr.Known) != 2 ||
		versionErr.Known[0] != LockFileVersionV1 ||
		versionErr.Known[1] != LockFileVersionV2 {
		t.Fatalf("UnknownLockFileVersionError.Known = %#v, want v1 and v2", versionErr.Known)
	}
}

func testLockFileMutationGeneratedTimestampParseError(t *testing.T) {
	t.Helper()

	_, err := parseLockFile(`version: "2.0"
generated: "not-rfc3339"
modules: {}`)
	var parseErr *time.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("parseLockFile() error = %v, want wrapped *time.ParseError", err)
	}
}

func testLockFileMutationCUESyntaxError(t *testing.T) {
	t.Helper()

	_, err := decodeLockFileCUE(`version: "2.0"
modules: {`)
	requireLockFileMutationCUEError(t, err, "parse lock file CUE")
}

func testLockFileMutationNonConcreteValidation(t *testing.T) {
	t.Helper()

	_, err := decodeLockFileCUE(`version: "2.0"
generated: string
modules: {}`)
	requireLockFileMutationCUEError(t, err, "validate lock file CUE")
}

func testLockFileMutationDecodeErrors(t *testing.T) {
	t.Helper()

	_, err := decodeLockFileCUE(`version: 2.0
generated: "2025-01-15T10:30:00Z"
modules: {}`)
	requireLockFileMutationCUEError(t, err, "decode lock file CUE")
}

func testLockFileMutationMissingContentHashModuleKey(t *testing.T) {
	t.Helper()

	_, err := parseLockFile(`version: "2.0"
generated: "2025-01-15T10:30:00Z"
modules: {
	"https://github.com/example/tools.git": {
		git_url:          "https://github.com/example/tools.git"
		version:          "^1.0.0"
		resolved_version: "1.2.3"
		git_commit:       "0123456789abcdef0123456789abcdef01234567"
		namespace:        "tools"
		command_source_id: "tools"
		module_id:        "io.example.tools"
	}
}`)
	lockedErr := requireInvalidLockedModuleError(t, err)
	if lockedErr.ModuleKey != ModuleRefKey(lockMutationGitURL) {
		t.Fatalf("InvalidLockedModuleError.ModuleKey = %q, want %q", lockedErr.ModuleKey, lockMutationGitURL)
	}
	if len(lockedErr.FieldErrors) != 1 || !errors.Is(lockedErr.FieldErrors[0], ErrInvalidContentHash) {
		t.Fatalf("InvalidLockedModuleError.FieldErrors = %#v, want missing content hash only", lockedErr.FieldErrors)
	}
}

func testLockFileMutationSplitMetadataModuleKey(t *testing.T) {
	t.Helper()

	key := ModuleRefKey(lockMutationGitURL)
	mod := validLockMutationLockedModule()
	mod.CommandSourceID = ""

	lockedErr := requireInvalidLockedModuleError(t, validateLockedModuleForVersion(key, mod, true))
	if lockedErr.ModuleKey != key {
		t.Fatalf("InvalidLockedModuleError.ModuleKey = %q, want %q", lockedErr.ModuleKey, key)
	}
	if len(lockedErr.FieldErrors) != 1 ||
		!strings.Contains(lockedErr.FieldErrors[0].Error(), "command_source_id is required") {
		t.Fatalf("InvalidLockedModuleError.FieldErrors = %#v, want command_source_id requirement", lockedErr.FieldErrors)
	}
}

func requireLockFileMutationCUEError(t *testing.T, err error, wantWrapper string) {
	t.Helper()

	if err == nil {
		t.Fatalf("decodeLockFileCUE() error = nil, want %s error", wantWrapper)
	}
	if !strings.Contains(err.Error(), wantWrapper) {
		t.Fatalf("decodeLockFileCUE() error = %v, want wrapper %q", err, wantWrapper)
	}
	var cueErr cueerrors.Error
	if !errors.As(err, &cueErr) {
		t.Fatalf("decodeLockFileCUE() error = %v, want wrapped CUE error", err)
	}
}

func validLockMutationLockedModule() LockedModule {
	return LockedModule{
		GitURL:          lockMutationGitURL,
		Version:         lockMutationVersion,
		ResolvedVersion: lockMutationResolvedVersion,
		GitCommit:       lockMutationGitCommit,
		Alias:           lockMutationAlias,
		Path:            lockMutationPath,
		Namespace:       lockMutationNamespace,
		CommandSourceID: lockMutationCommandSource,
		ModuleID:        lockMutationModuleID,
		ContentHash:     testContentHash,
	}
}

func requireInvalidModuleNamespaceError(t *testing.T, err error) *InvalidModuleNamespaceError {
	t.Helper()

	if !errors.Is(err, ErrInvalidModuleNamespace) {
		t.Fatalf("error = %v, want ErrInvalidModuleNamespace", err)
	}
	var nsErr *InvalidModuleNamespaceError
	if !errors.As(err, &nsErr) {
		t.Fatalf("error type = %T, want *InvalidModuleNamespaceError", err)
	}
	return nsErr
}

func requireInvalidLockFileVersionError(t *testing.T, err error) *InvalidLockFileVersionError {
	t.Helper()

	if !errors.Is(err, ErrInvalidLockFileVersion) {
		t.Fatalf("error = %v, want ErrInvalidLockFileVersion", err)
	}
	var versionErr *InvalidLockFileVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("error type = %T, want *InvalidLockFileVersionError", err)
	}
	return versionErr
}

func requireInvalidModuleRefKeyError(t *testing.T, err error) *InvalidModuleRefKeyError {
	t.Helper()

	if !errors.Is(err, ErrInvalidModuleRefKey) {
		t.Fatalf("error = %v, want ErrInvalidModuleRefKey", err)
	}
	var refErr *InvalidModuleRefKeyError
	if !errors.As(err, &refErr) {
		t.Fatalf("error type = %T, want *InvalidModuleRefKeyError", err)
	}
	return refErr
}

func requireInvalidLockedModuleError(t *testing.T, err error) *InvalidLockedModuleError {
	t.Helper()

	if !errors.Is(err, ErrInvalidLockedModule) {
		t.Fatalf("error = %v, want ErrInvalidLockedModule", err)
	}
	var lockedErr *InvalidLockedModuleError
	if !errors.As(err, &lockedErr) {
		t.Fatalf("error type = %T, want *InvalidLockedModuleError", err)
	}
	return lockedErr
}
