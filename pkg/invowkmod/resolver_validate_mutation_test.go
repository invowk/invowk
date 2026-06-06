// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestInvalidModuleRefErrorMutationFormatsFieldErrors(t *testing.T) {
	t.Parallel()

	err := ModuleRef{GitURL: "not-a-url"}.Validate()
	refErr := requireModuleRefMutationError(t, err, 1)
	got := refErr.Error()
	if got == "" {
		t.Fatal("InvalidModuleRefError.Error() = empty string, want formatted field error")
	}
	if !strings.Contains(got, "module ref") || !strings.Contains(got, "1 field error") {
		t.Fatalf("InvalidModuleRefError.Error() = %q, want module ref field detail", got)
	}
}

func TestModuleRefValidateDeclarationMutationRetainsBaseValidation(t *testing.T) {
	t.Parallel()

	err := ModuleRef{
		GitURL:  "not-a-url",
		Version: "^1.0.0",
	}.ValidateDeclaration()

	refErr := requireModuleRefMutationError(t, err, 1)
	requireMutationFieldError(t, refErr.FieldErrors, ErrInvalidGitURL)
}

func TestResolvedModuleValidateMutationOptionalFieldErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolved ResolvedModule
		wantErr  error
	}{
		{
			name:     "resolved version",
			resolved: ResolvedModule{ResolvedVersion: "not-semver"},
			wantErr:  ErrInvalidSemVer,
		},
		{
			name:     "git commit",
			resolved: ResolvedModule{GitCommit: "abc123"},
			wantErr:  ErrInvalidGitCommit,
		},
		{
			name:     "cache path",
			resolved: ResolvedModule{CachePath: types.FilesystemPath(" \t ")},
			wantErr:  types.ErrInvalidFilesystemPath,
		},
		{
			name:     "content hash",
			resolved: ResolvedModule{ContentHash: "sha256:bad"},
			wantErr:  ErrInvalidContentHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.resolved.Validate()
			resolvedErr := requireResolvedModuleMutationError(t, err, 1)
			requireMutationFieldError(t, resolvedErr.FieldErrors, tt.wantErr)
		})
	}
}

func TestResolvedModuleValidateMutationRetainsCommandSourceIDError(t *testing.T) {
	t.Parallel()

	err := ResolvedModule{
		CommandSourceID: "1bad",
	}.Validate()

	resolvedErr := requireResolvedModuleMutationError(t, err, 1)
	requireMutationFieldErrorText(t, resolvedErr.FieldErrors, "module source ID")
}

func TestResolverValidateMutationFormatsCompositeErrors(t *testing.T) {
	t.Parallel()

	t.Run("resolved module", func(t *testing.T) {
		t.Parallel()

		err := ResolvedModule{CommandSourceID: "1bad"}.Validate()
		resolvedErr := requireResolvedModuleMutationError(t, err, 1)
		requireMutationErrorString(t, resolvedErr.Error(), "resolved module")
	})

	t.Run("ambiguous match", func(t *testing.T) {
		t.Parallel()

		err := AmbiguousMatch{GitURL: "not-a-url"}.Validate()
		matchErr := requireAmbiguousMatchMutationError(t, err, 1)
		requireMutationErrorString(t, matchErr.Error(), "ambiguous match")
	})

	t.Run("remove result", func(t *testing.T) {
		t.Parallel()

		err := RemoveResult{
			LockKey: ModuleRefKey(""),
			RemovedEntry: LockedModule{
				GitURL: "not-a-url",
			},
		}.Validate()
		removeErr := requireRemoveResultMutationError(t, err, 2)
		requireMutationErrorString(t, removeErr.Error(), "remove result")
	})
}

func TestAmbiguousMatchValidateMutationRetainsLockKeyError(t *testing.T) {
	t.Parallel()

	err := AmbiguousMatch{
		LockKey: ModuleRefKey(" \t "),
	}.Validate()

	matchErr := requireAmbiguousMatchMutationError(t, err, 1)
	requireMutationFieldError(t, matchErr.FieldErrors, ErrInvalidModuleRefKey)
}

func TestRemoveResultValidateMutationRetainsFieldErrors(t *testing.T) {
	t.Parallel()

	err := RemoveResult{
		LockKey: ModuleRefKey(""),
		RemovedEntry: LockedModule{
			GitURL: "not-a-url",
		},
	}.Validate()

	removeErr := requireRemoveResultMutationError(t, err, 2)
	requireMutationFieldError(t, removeErr.FieldErrors, ErrInvalidModuleRefKey)
	requireMutationFieldError(t, removeErr.FieldErrors, ErrInvalidLockedModule)
}

func requireModuleRefMutationError(t *testing.T, err error, wantCount int) *InvalidModuleRefError {
	t.Helper()

	if err == nil {
		t.Fatal("validation error = nil, want *InvalidModuleRefError")
	}
	if !errors.Is(err, ErrInvalidModuleRef) {
		t.Fatalf("validation error = %v, want ErrInvalidModuleRef", err)
	}
	var refErr *InvalidModuleRefError
	if !errors.As(err, &refErr) {
		t.Fatalf("validation error type = %T, want *InvalidModuleRefError", err)
	}
	if len(refErr.FieldErrors) != wantCount {
		t.Fatalf("field error count = %d, want %d: %v", len(refErr.FieldErrors), wantCount, refErr.FieldErrors)
	}
	return refErr
}

func requireResolvedModuleMutationError(t *testing.T, err error, wantCount int) *InvalidResolvedModuleError {
	t.Helper()

	if err == nil {
		t.Fatal("validation error = nil, want *InvalidResolvedModuleError")
	}
	if !errors.Is(err, ErrInvalidResolvedModule) {
		t.Fatalf("validation error = %v, want ErrInvalidResolvedModule", err)
	}
	var resolvedErr *InvalidResolvedModuleError
	if !errors.As(err, &resolvedErr) {
		t.Fatalf("validation error type = %T, want *InvalidResolvedModuleError", err)
	}
	if len(resolvedErr.FieldErrors) != wantCount {
		t.Fatalf("field error count = %d, want %d: %v", len(resolvedErr.FieldErrors), wantCount, resolvedErr.FieldErrors)
	}
	return resolvedErr
}

func requireAmbiguousMatchMutationError(t *testing.T, err error, wantCount int) *InvalidAmbiguousMatchError {
	t.Helper()

	if err == nil {
		t.Fatal("validation error = nil, want *InvalidAmbiguousMatchError")
	}
	if !errors.Is(err, ErrInvalidAmbiguousMatch) {
		t.Fatalf("validation error = %v, want ErrInvalidAmbiguousMatch", err)
	}
	var matchErr *InvalidAmbiguousMatchError
	if !errors.As(err, &matchErr) {
		t.Fatalf("validation error type = %T, want *InvalidAmbiguousMatchError", err)
	}
	if len(matchErr.FieldErrors) != wantCount {
		t.Fatalf("field error count = %d, want %d: %v", len(matchErr.FieldErrors), wantCount, matchErr.FieldErrors)
	}
	return matchErr
}

func requireRemoveResultMutationError(t *testing.T, err error, wantCount int) *InvalidRemoveResultError {
	t.Helper()

	if err == nil {
		t.Fatal("validation error = nil, want *InvalidRemoveResultError")
	}
	if !errors.Is(err, ErrInvalidRemoveResult) {
		t.Fatalf("validation error = %v, want ErrInvalidRemoveResult", err)
	}
	var removeErr *InvalidRemoveResultError
	if !errors.As(err, &removeErr) {
		t.Fatalf("validation error type = %T, want *InvalidRemoveResultError", err)
	}
	if len(removeErr.FieldErrors) != wantCount {
		t.Fatalf("field error count = %d, want %d: %v", len(removeErr.FieldErrors), wantCount, removeErr.FieldErrors)
	}
	return removeErr
}

func requireMutationFieldError(t *testing.T, fieldErrors []error, want error) {
	t.Helper()

	for _, fieldErr := range fieldErrors {
		if errors.Is(fieldErr, want) {
			return
		}
	}
	t.Fatalf("field errors = %v, want one wrapping %v", fieldErrors, want)
}

func requireMutationFieldErrorText(t *testing.T, fieldErrors []error, want string) {
	t.Helper()

	for _, fieldErr := range fieldErrors {
		if strings.Contains(fieldErr.Error(), want) {
			return
		}
	}
	t.Fatalf("field errors = %v, want one containing %q", fieldErrors, want)
}

func requireMutationErrorString(t *testing.T, got, wantContext string) {
	t.Helper()

	if got == "" {
		t.Fatalf("error string = empty, want %q context", wantContext)
	}
	if !strings.Contains(got, wantContext) {
		t.Fatalf("error string = %q, want %q context", got, wantContext)
	}
	if !strings.Contains(got, "field error") {
		t.Fatalf("error string = %q, want field-error count", got)
	}
}
