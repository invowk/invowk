// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestLockFileVersion_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version LockFileVersion
		want    bool
		wantErr bool
	}{
		{"standard version", LockFileVersion("1.0"), true, false},
		{"semver version", LockFileVersion("2.1.0"), true, false},
		{"single digit", LockFileVersion("1"), true, false},
		{"arbitrary string", LockFileVersion("v1-beta"), true, false},
		{"empty is invalid", LockFileVersion(""), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.version.IsValid()
			if isValid != tt.want {
				t.Errorf("LockFileVersion(%q).IsValid() = %v, want %v", tt.version, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("LockFileVersion(%q).IsValid() returned no errors, want error", tt.version)
				}
				if !errors.Is(errs[0], ErrInvalidLockFileVersion) {
					t.Errorf("error should wrap ErrInvalidLockFileVersion, got: %v", errs[0])
				}
				var lvErr *InvalidLockFileVersionError
				if !errors.As(errs[0], &lvErr) {
					t.Errorf("error should be *InvalidLockFileVersionError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("LockFileVersion(%q).IsValid() returned unexpected errors: %v", tt.version, errs)
			}
		})
	}
}

func TestLockFileVersion_String(t *testing.T) {
	t.Parallel()
	v := LockFileVersion("1.0")
	if v.String() != "1.0" {
		t.Errorf("LockFileVersion.String() = %q, want %q", v.String(), "1.0")
	}
}

func TestModuleRefKey_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     ModuleRefKey
		want    bool
		wantErr bool
	}{
		{"git URL key", ModuleRefKey("https://github.com/user/repo.git"), true, false},
		{"git URL with subpath", ModuleRefKey("https://github.com/user/repo.git#packages/mod"), true, false},
		{"simple string", ModuleRefKey("mymodule"), true, false},
		{"empty is invalid", ModuleRefKey(""), false, true},
		{"whitespace-only is invalid", ModuleRefKey("   "), false, true},
		{"tab-only is invalid", ModuleRefKey("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.key.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleRefKey(%q).IsValid() = %v, want %v", tt.key, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleRefKey(%q).IsValid() returned no errors, want error", tt.key)
				}
				if !errors.Is(errs[0], ErrInvalidModuleRefKey) {
					t.Errorf("error should wrap ErrInvalidModuleRefKey, got: %v", errs[0])
				}
				var mrkErr *InvalidModuleRefKeyError
				if !errors.As(errs[0], &mrkErr) {
					t.Errorf("error should be *InvalidModuleRefKeyError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleRefKey(%q).IsValid() returned unexpected errors: %v", tt.key, errs)
			}
		})
	}
}

func TestModuleRefKey_String(t *testing.T) {
	t.Parallel()
	k := ModuleRefKey("https://github.com/user/repo.git")
	if k.String() != "https://github.com/user/repo.git" {
		t.Errorf("ModuleRefKey.String() = %q, want %q", k.String(), "https://github.com/user/repo.git")
	}
}
