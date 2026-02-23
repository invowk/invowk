// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestDotenvFilePath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    DotenvFilePath
		want    bool
		wantErr bool
	}{
		{"simple env file", DotenvFilePath(".env"), true, false},
		{"relative path", DotenvFilePath("config/.env.local"), true, false},
		{"optional suffix", DotenvFilePath(".env.production?"), true, false},
		{"absolute path", DotenvFilePath("/etc/app/.env"), true, false},
		{"empty is invalid", DotenvFilePath(""), false, true},
		{"whitespace only is invalid", DotenvFilePath("   "), false, true},
		{"tab only is invalid", DotenvFilePath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("DotenvFilePath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("DotenvFilePath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidDotenvFilePath) {
					t.Errorf("error should wrap ErrInvalidDotenvFilePath, got: %v", errs[0])
				}
				var dpErr *InvalidDotenvFilePathError
				if !errors.As(errs[0], &dpErr) {
					t.Errorf("error should be *InvalidDotenvFilePathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("DotenvFilePath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestDotenvFilePath_String(t *testing.T) {
	t.Parallel()
	p := DotenvFilePath(".env")
	if p.String() != ".env" {
		t.Errorf("DotenvFilePath.String() = %q, want %q", p.String(), ".env")
	}
}
