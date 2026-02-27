// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidLoadOptions is the sentinel error wrapped by InvalidLoadOptionsError.
var ErrInvalidLoadOptions = errors.New("invalid load options")

type (
	// LoadOptions defines explicit configuration loading inputs.
	LoadOptions struct {
		// ConfigFilePath forces loading from a specific config file when set.
		ConfigFilePath types.FilesystemPath
		// ConfigDirPath overrides the config directory lookup when set.
		ConfigDirPath types.FilesystemPath
		// BaseDir overrides the directory for CWD-relative config file lookup.
		// When empty, the relative path "config.cue" resolves against the
		// process's current working directory (os.Getwd).
		BaseDir types.FilesystemPath
	}

	// InvalidLoadOptionsError is returned when LoadOptions has one or more
	// invalid typed fields. FieldErrors contains the per-field validation errors.
	InvalidLoadOptionsError struct {
		FieldErrors []error
	}

	// Provider loads configuration from explicit options.
	// This abstraction enables testing with custom config sources or mock implementations.
	Provider interface {
		Load(ctx context.Context, opts LoadOptions) (*Config, error)
	}

	// fileProvider is the production Provider that loads configuration from the filesystem.
	fileProvider struct{}
)

// IsValid returns whether all typed fields in the LoadOptions are valid.
// All three fields use zero-value-is-valid semantics: empty means "use default".
// Only non-empty values are validated via their respective IsValid() methods.
func (o LoadOptions) IsValid() (bool, []error) {
	var errs []error

	if o.ConfigFilePath != "" {
		if valid, fieldErrs := o.ConfigFilePath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}

	if o.ConfigDirPath != "" {
		if valid, fieldErrs := o.ConfigDirPath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}

	if o.BaseDir != "" {
		if valid, fieldErrs := o.BaseDir.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}

	if len(errs) > 0 {
		return false, []error{&InvalidLoadOptionsError{FieldErrors: errs}}
	}

	return true, nil
}

// Error implements the error interface for InvalidLoadOptionsError.
func (e *InvalidLoadOptionsError) Error() string {
	if len(e.FieldErrors) == 1 {
		return fmt.Sprintf("invalid load options: %v", e.FieldErrors[0])
	}

	return fmt.Sprintf("invalid load options: %d field errors", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidLoadOptions for errors.Is() compatibility.
func (e *InvalidLoadOptionsError) Unwrap() error { return ErrInvalidLoadOptions }

// NewProvider creates a configuration provider.
func NewProvider() Provider {
	return &fileProvider{}
}

// Load reads configuration from the requested source.
// It validates LoadOptions before proceeding and delegates to loadWithOptions
// which validates the resulting Config after unmarshalling.
func (p *fileProvider) Load(ctx context.Context, opts LoadOptions) (*Config, error) {
	if isValid, errs := opts.IsValid(); !isValid {
		return nil, fmt.Errorf("config load: %w", errors.Join(errs...))
	}

	cfg, _, err := loadWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
