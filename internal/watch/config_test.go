// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestConfigIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       Config
		wantValid bool
		wantErrs  bool
	}{
		{
			name:      "zero value is valid (empty patterns and empty BaseDir)",
			cfg:       Config{},
			wantValid: true,
		},
		{
			name: "all valid fields",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"**/*.go", "**/*.cue"},
				Ignore:   []invowkfile.GlobPattern{"**/.git/**"},
				BaseDir:  "/home/user/project",
			},
			wantValid: true,
		},
		{
			name: "empty pattern slices are valid",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{},
				Ignore:   []invowkfile.GlobPattern{},
			},
			wantValid: true,
		},
		{
			name: "non-domain fields do not affect validity",
			cfg: Config{
				ClearScreen: true,
				Patterns:    []invowkfile.GlobPattern{"**/*.go"},
			},
			wantValid: true,
		},
		{
			name: "single invalid pattern: empty GlobPattern",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{""},
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "single invalid ignore: empty GlobPattern",
			cfg: Config{
				Ignore: []invowkfile.GlobPattern{""},
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "single invalid field: whitespace-only BaseDir",
			cfg: Config{
				BaseDir: types.FilesystemPath("   "),
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "invalid pattern syntax",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"[invalid"},
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "multiple invalid fields",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"", "**/*.go", ""},
				Ignore:   []invowkfile.GlobPattern{""},
				BaseDir:  types.FilesystemPath("   "),
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "valid patterns with empty BaseDir (uses cwd default)",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"**/*.go"},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			valid, errs := tt.cfg.IsValid()
			if valid != tt.wantValid {
				t.Errorf("IsValid() valid = %v, want %v", valid, tt.wantValid)
			}
			if tt.wantErrs && len(errs) == 0 {
				t.Error("IsValid() expected errors but got none")
			}
			if !tt.wantErrs && len(errs) > 0 {
				t.Errorf("IsValid() unexpected errors: %v", errs)
			}
		})
	}
}

func TestConfigIsValid_SentinelError(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Patterns: []invowkfile.GlobPattern{""},
	}

	valid, errs := cfg.IsValid()
	if valid {
		t.Fatal("expected invalid config")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], ErrInvalidWatchConfig) {
		t.Errorf("error should wrap ErrInvalidWatchConfig, got: %v", errs[0])
	}

	var configErr *InvalidWatchConfigError
	if !errors.As(errs[0], &configErr) {
		t.Fatalf("error should be *InvalidWatchConfigError, got: %T", errs[0])
	}
	if len(configErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error, got %d", len(configErr.FieldErrors))
	}
}

func TestConfigIsValid_MultipleFieldErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Patterns: []invowkfile.GlobPattern{"", ""},
		Ignore:   []invowkfile.GlobPattern{""},
		BaseDir:  types.FilesystemPath("   "),
	}

	valid, errs := cfg.IsValid()
	if valid {
		t.Fatal("expected invalid config")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 wrapped error, got %d", len(errs))
	}

	var configErr *InvalidWatchConfigError
	if !errors.As(errs[0], &configErr) {
		t.Fatalf("error should be *InvalidWatchConfigError, got: %T", errs[0])
	}
	// 2 empty Patterns + 1 empty Ignore + 1 whitespace BaseDir = 4 field errors
	if len(configErr.FieldErrors) != 4 {
		t.Errorf("expected 4 field errors, got %d: %v", len(configErr.FieldErrors), configErr.FieldErrors)
	}

	// Verify Error() message mentions count
	errMsg := configErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}
}

func TestInvalidWatchConfigError_Unwrap(t *testing.T) {
	t.Parallel()

	err := &InvalidWatchConfigError{
		FieldErrors: []error{errors.New("test")},
	}
	if !errors.Is(err, ErrInvalidWatchConfig) {
		t.Error("Unwrap() should return ErrInvalidWatchConfig")
	}
}
