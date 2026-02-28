// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestDotenvFilePath_Validate(t *testing.T) {
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
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("DotenvFilePath(%q).Validate() error = %v, want valid=%v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DotenvFilePath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidDotenvFilePath) {
					t.Errorf("error should wrap ErrInvalidDotenvFilePath, got: %v", err)
				}
				var dpErr *InvalidDotenvFilePathError
				if !errors.As(err, &dpErr) {
					t.Errorf("error should be *InvalidDotenvFilePathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("DotenvFilePath(%q).Validate() returned unexpected error: %v", tt.path, err)
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
