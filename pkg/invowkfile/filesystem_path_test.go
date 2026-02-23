// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestFilesystemPath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    FilesystemPath
		want    bool
		wantErr bool
	}{
		{"absolute path", FilesystemPath("/usr/bin/bash"), true, false},
		{"relative path", FilesystemPath("run.sh"), true, false},
		{"windows style", FilesystemPath("C:\\Program Files\\app.exe"), true, false},
		{"empty is invalid", FilesystemPath(""), false, true},
		{"whitespace only is invalid", FilesystemPath("   "), false, true},
		{"tab only is invalid", FilesystemPath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("FilesystemPath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("FilesystemPath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidFilesystemPath) {
					t.Errorf("error should wrap ErrInvalidFilesystemPath, got: %v", errs[0])
				}
				var fpErr *InvalidFilesystemPathError
				if !errors.As(errs[0], &fpErr) {
					t.Errorf("error should be *InvalidFilesystemPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("FilesystemPath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestFilesystemPath_String(t *testing.T) {
	t.Parallel()
	p := FilesystemPath("/usr/bin/bash")
	if p.String() != "/usr/bin/bash" {
		t.Errorf("FilesystemPath.String() = %q, want %q", p.String(), "/usr/bin/bash")
	}
}
