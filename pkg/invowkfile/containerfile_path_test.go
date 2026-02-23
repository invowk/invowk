// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestContainerfilePath_IsValid(t *testing.T) {
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
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerfilePath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerfilePath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidContainerfilePath) {
					t.Errorf("error should wrap ErrInvalidContainerfilePath, got: %v", errs[0])
				}
				var cpErr *InvalidContainerfilePathError
				if !errors.As(errs[0], &cpErr) {
					t.Errorf("error should be *InvalidContainerfilePathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerfilePath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
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
