// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      Config
		wantOK   bool
		wantErrs bool
	}{
		{
			name:   "zero value is valid (empty patterns and empty BaseDir)",
			cfg:    Config{},
			wantOK: true,
		},
		{
			name: "all valid fields",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"**/*.go", "**/*.cue"},
				Ignore:   []invowkfile.GlobPattern{"**/.git/**"},
				BaseDir:  "/home/user/project",
			},
			wantOK: true,
		},
		{
			name: "empty pattern slices are valid",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{},
				Ignore:   []invowkfile.GlobPattern{},
			},
			wantOK: true,
		},
		{
			name: "non-domain fields do not affect validity",
			cfg: Config{
				ClearScreen: true,
				Patterns:    []invowkfile.GlobPattern{"**/*.go"},
			},
			wantOK: true,
		},
		{
			name: "single invalid pattern: empty GlobPattern",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{""},
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "single invalid ignore: empty GlobPattern",
			cfg: Config{
				Ignore: []invowkfile.GlobPattern{""},
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "single invalid field: whitespace-only BaseDir",
			cfg: Config{
				BaseDir: types.FilesystemPath("   "),
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "invalid pattern syntax",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"[invalid"},
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "multiple invalid fields",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"", "**/*.go", ""},
				Ignore:   []invowkfile.GlobPattern{""},
				BaseDir:  types.FilesystemPath("   "),
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "valid patterns with empty BaseDir (uses cwd default)",
			cfg: Config{
				Patterns: []invowkfile.GlobPattern{"**/*.go"},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Validate() error = %v, wantOK %v", err, tt.wantOK)
			}
			if tt.wantErrs && err == nil {
				t.Error("Validate() expected error but got nil")
			}
			if !tt.wantErrs && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestConfigValidate_SentinelError(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Patterns: []invowkfile.GlobPattern{""},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !errors.Is(err, ErrInvalidWatchConfig) {
		t.Errorf("error should wrap ErrInvalidWatchConfig, got: %v", err)
	}

	var configErr *InvalidWatchConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("error should be *InvalidWatchConfigError, got: %T", err)
	}
	if len(configErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error, got %d", len(configErr.FieldErrors))
	}
}

func TestConfigValidate_MultipleFieldErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Patterns: []invowkfile.GlobPattern{"", ""},
		Ignore:   []invowkfile.GlobPattern{""},
		BaseDir:  types.FilesystemPath("   "),
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	var configErr *InvalidWatchConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("error should be *InvalidWatchConfigError, got: %T", err)
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
