// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestFilesystemPath_Validate(t *testing.T) {
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
		{"path with spaces", FilesystemPath("/path/to/my file.txt"), true, false},
		{"dot path", FilesystemPath("."), true, false},
		{"empty is invalid", FilesystemPath(""), false, true},
		{"whitespace only is invalid", FilesystemPath("   "), false, true},
		{"tab only is invalid", FilesystemPath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("FilesystemPath(%q).Validate() error = %v, wantValid %v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FilesystemPath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidFilesystemPath) {
					t.Errorf("error should wrap ErrInvalidFilesystemPath, got: %v", err)
				}
				var fpErr *InvalidFilesystemPathError
				if !errors.As(err, &fpErr) {
					t.Errorf("error should be *InvalidFilesystemPathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("FilesystemPath(%q).Validate() returned unexpected error: %v", tt.path, err)
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
