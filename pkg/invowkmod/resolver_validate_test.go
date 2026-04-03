// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestModuleRef_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ref       ModuleRef
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete ref",
			ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "myalias",
				Path:    "modules/tools",
			},
			true, false, 0,
		},
		{
			"valid minimal ref (zero values)",
			ModuleRef{},
			true, false, 0,
		},
		{
			"valid with only required-like fields",
			ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			true, false, 0,
		},
		{
			"invalid git url",
			ModuleRef{
				GitURL:  "not-a-url",
				Version: "^1.0.0",
			},
			false, true, 1,
		},
		{
			"invalid version constraint",
			ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "not-semver",
			},
			false, true, 1,
		},
		{
			"invalid alias (whitespace)",
			ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "   ",
			},
			false, true, 1,
		},
		{
			"invalid path (traversal)",
			ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Path:    "../escape",
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			ModuleRef{
				GitURL:  "not-a-url",
				Version: "not-semver",
				Alias:   "   ",
				Path:    "../escape",
			},
			false, true, 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.ref.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleRef.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleRef.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidModuleRef) {
					t.Errorf("error should wrap ErrInvalidModuleRef, got: %v", err)
				}
				var refErr *InvalidModuleRefError
				if !errors.As(err, &refErr) {
					t.Fatalf("error should be *InvalidModuleRefError, got: %T", err)
				}
				if len(refErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(refErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("ModuleRef.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestResolvedModule_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		resolved  ResolvedModule
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete resolved module",
			ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL:  "https://github.com/user/repo.git",
					Version: "^1.0.0",
				},
				ResolvedVersion: "1.2.3",
				GitCommit:       "abc123def456789012345678901234567890abcd",
				CachePath:       types.FilesystemPath(filepath.Join(tmpDir, "modules", "repo")),
				Namespace:       "repo@1.2.3",
				ModuleName:      "repo",
				ModuleID:        "io.example.repo",
			},
			true, false, 0,
		},
		{
			"valid zero value",
			ResolvedModule{},
			true, false, 0,
		},
		{
			"valid minimal (only module ref)",
			ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL:  "https://github.com/user/repo.git",
					Version: "^1.0.0",
				},
			},
			true, false, 0,
		},
		{
			"invalid module ref (nested)",
			ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL: "not-a-url",
				},
			},
			false, true, 1,
		},
		{
			"invalid module name",
			ResolvedModule{
				ModuleName: ModuleShortName("1invalid"),
			},
			false, true, 1,
		},
		{
			"invalid module ID",
			ResolvedModule{
				ModuleID: ModuleID("1bad"),
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL: "not-a-url",
				},
				ModuleName: ModuleShortName("1invalid"),
				ModuleID:   ModuleID("1bad"),
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.resolved.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ResolvedModule.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolvedModule.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidResolvedModule) {
					t.Errorf("error should wrap ErrInvalidResolvedModule, got: %v", err)
				}
				var resolvedErr *InvalidResolvedModuleError
				if !errors.As(err, &resolvedErr) {
					t.Fatalf("error should be *InvalidResolvedModuleError, got: %T", err)
				}
				if len(resolvedErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(resolvedErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("ResolvedModule.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestAmbiguousMatch_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		match     AmbiguousMatch
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete match",
			AmbiguousMatch{
				LockKey:   ModuleRefKey("https://github.com/user/repo.git"),
				Namespace: ModuleNamespace("repo@1.2.3"),
				GitURL:    GitURL("https://github.com/user/repo.git"),
			},
			true, false, 0,
		},
		{
			"valid zero value",
			AmbiguousMatch{},
			true, false, 0,
		},
		{
			"valid with partial fields",
			AmbiguousMatch{
				LockKey: ModuleRefKey("somekey"),
			},
			true, false, 0,
		},
		{
			"invalid git url (non-empty but bad format)",
			AmbiguousMatch{
				LockKey:   ModuleRefKey("somekey"),
				Namespace: ModuleNamespace("ns"),
				GitURL:    GitURL("not-a-url"),
			},
			false, true, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.match.Validate()
			if (err == nil) != tt.want {
				t.Errorf("AmbiguousMatch.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("AmbiguousMatch.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidAmbiguousMatch) {
					t.Errorf("error should wrap ErrInvalidAmbiguousMatch, got: %v", err)
				}
				var matchErr *InvalidAmbiguousMatchError
				if !errors.As(err, &matchErr) {
					t.Fatalf("error should be *InvalidAmbiguousMatchError, got: %T", err)
				}
				if len(matchErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(matchErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("AmbiguousMatch.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  RemoveResult
		want    bool
		wantErr bool
	}{
		{
			"valid complete result",
			RemoveResult{
				LockKey: ModuleRefKey("https://github.com/user/repo.git"),
				RemovedEntry: LockedModule{
					GitURL:          "https://github.com/user/repo.git",
					Version:         "^1.0.0",
					ResolvedVersion: "1.2.3",
					GitCommit:       "abc123def456789012345678901234567890abcd",
					Namespace:       "repo@1.2.3",
					ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			true, false,
		},
		{
			"invalid lock key (empty)",
			RemoveResult{
				LockKey: ModuleRefKey(""),
				RemovedEntry: LockedModule{
					GitURL:          "https://github.com/user/repo.git",
					Version:         "^1.0.0",
					ResolvedVersion: "1.2.3",
					GitCommit:       "abc123def456789012345678901234567890abcd",
					Namespace:       "repo@1.2.3",
					ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			false, true,
		},
		{
			"invalid removed entry (invalid git url)",
			RemoveResult{
				LockKey: ModuleRefKey("somekey"),
				RemovedEntry: LockedModule{
					GitURL:          "not-a-url",
					Version:         "^1.0.0",
					ResolvedVersion: "1.2.3",
					GitCommit:       "abc123def456789012345678901234567890abcd",
					Namespace:       "repo@1.2.3",
					ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			false, true,
		},
		{
			"both fields invalid",
			RemoveResult{
				LockKey: ModuleRefKey(""),
				RemovedEntry: LockedModule{
					GitURL: "not-a-url",
				},
			},
			false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err == nil) != tt.want {
				t.Errorf("RemoveResult.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RemoveResult.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidRemoveResult) {
					t.Errorf("error should wrap ErrInvalidRemoveResult, got: %v", err)
				}
				var removeErr *InvalidRemoveResultError
				if !errors.As(err, &removeErr) {
					t.Fatalf("error should be *InvalidRemoveResultError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("RemoveResult.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
