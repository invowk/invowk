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

	// LoadResult contains the loaded configuration and the source path that
	// produced it. SourcePath is empty when defaults were used.
	LoadResult struct {
		Config     *Config
		SourcePath types.FilesystemPath
	}

	// Provider loads configuration from explicit options.
	// This abstraction enables testing with custom config sources or mock implementations.
	Provider interface {
		Load(ctx context.Context, opts LoadOptions) (*Config, error)
		LoadWithSource(ctx context.Context, opts LoadOptions) (LoadResult, error)
	}

	// fileProvider is the production Provider that loads configuration from the filesystem.
	fileProvider struct{}
)

// Validate returns an error if any typed fields in the LoadOptions are invalid.
// All three fields use zero-value-is-valid semantics: empty means "use default".
// Only non-empty values are validated via their respective Validate() methods.
func (o LoadOptions) Validate() error {
	var errs []error

	if o.ConfigFilePath != "" {
		if err := o.ConfigFilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	if o.ConfigDirPath != "" {
		if err := o.ConfigDirPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	if o.BaseDir != "" {
		if err := o.BaseDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return &InvalidLoadOptionsError{FieldErrors: errs}
	}

	return nil
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

// Validate returns nil when the result contains a config and the optional
// source path is valid.
func (r LoadResult) Validate() error {
	var errs []error
	if r.Config == nil {
		errs = append(errs, errors.New("config is required"))
	} else if err := r.Config.Validate(); err != nil {
		errs = append(errs, err)
	}
	if r.SourcePath != "" {
		if err := r.SourcePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// NewProvider creates a configuration provider.
func NewProvider() Provider {
	return &fileProvider{}
}

// Load reads configuration from the requested source.
// It validates LoadOptions before proceeding and delegates to loadWithOptions
// which validates the resulting Config after unmarshalling.
func (p *fileProvider) Load(ctx context.Context, opts LoadOptions) (*Config, error) {
	result, err := p.LoadWithSource(ctx, opts)
	if err != nil {
		return nil, err
	}

	return result.Config, nil
}

// LoadWithSource reads configuration and returns the resolved source metadata.
// It validates LoadOptions before proceeding and delegates to loadWithOptions
// which validates the resulting Config after decoding.
func (p *fileProvider) LoadWithSource(ctx context.Context, opts LoadOptions) (LoadResult, error) {
	if err := opts.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("config load: %w", err)
	}

	cfg, sourcePath, err := loadWithOptions(ctx, opts)
	if err != nil {
		return LoadResult{}, err
	}

	result := LoadResult{Config: cfg, SourcePath: sourcePath}
	if err := result.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("config load result: %w", err)
	}
	return result, nil
}
