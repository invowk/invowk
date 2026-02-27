// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestLockFileVersion_Validate(t *testing.T) {
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
			err := tt.version.Validate()
			if (err == nil) != tt.want {
				t.Errorf("LockFileVersion(%q).Validate() error = %v, wantValid %v", tt.version, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("LockFileVersion(%q).Validate() returned nil, want error", tt.version)
				}
				if !errors.Is(err, ErrInvalidLockFileVersion) {
					t.Errorf("error should wrap ErrInvalidLockFileVersion, got: %v", err)
				}
				var lvErr *InvalidLockFileVersionError
				if !errors.As(err, &lvErr) {
					t.Errorf("error should be *InvalidLockFileVersionError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("LockFileVersion(%q).Validate() returned unexpected error: %v", tt.version, err)
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

func TestModuleRefKey_Validate(t *testing.T) {
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
			err := tt.key.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleRefKey(%q).Validate() error = %v, wantValid %v", tt.key, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleRefKey(%q).Validate() returned nil, want error", tt.key)
				}
				if !errors.Is(err, ErrInvalidModuleRefKey) {
					t.Errorf("error should wrap ErrInvalidModuleRefKey, got: %v", err)
				}
				var mrkErr *InvalidModuleRefKeyError
				if !errors.As(err, &mrkErr) {
					t.Errorf("error should be *InvalidModuleRefKeyError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ModuleRefKey(%q).Validate() returned unexpected error: %v", tt.key, err)
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
