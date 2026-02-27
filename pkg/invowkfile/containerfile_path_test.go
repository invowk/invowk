// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestContainerfilePath_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    ContainerfilePath
		want    bool
		wantErr bool
	}{
		{"simple path", ContainerfilePath("Containerfile"), true, false},
		{"relative path", ContainerfilePath("./docker/Dockerfile"), true, false},
		{"absolute path", ContainerfilePath("/project/Containerfile"), true, false},
		{"empty is valid (zero value)", ContainerfilePath(""), true, false},
		{"whitespace only is invalid", ContainerfilePath("   "), false, true},
		{"tab only is invalid", ContainerfilePath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ContainerfilePath(%q).Validate() error = %v, want valid=%v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ContainerfilePath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidContainerfilePath) {
					t.Errorf("error should wrap ErrInvalidContainerfilePath, got: %v", err)
				}
				var cpErr *InvalidContainerfilePathError
				if !errors.As(err, &cpErr) {
					t.Errorf("error should be *InvalidContainerfilePathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ContainerfilePath(%q).Validate() returned unexpected error: %v", tt.path, err)
			}
		})
	}
}

func TestContainerfilePath_String(t *testing.T) {
	t.Parallel()
	p := ContainerfilePath("Containerfile")
	if p.String() != "Containerfile" {
		t.Errorf("ContainerfilePath.String() = %q, want %q", p.String(), "Containerfile")
	}
}
