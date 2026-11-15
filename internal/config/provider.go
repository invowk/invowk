// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"os"

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
	}

	// InvalidLoadOptionsError is returned when LoadOptions has one or more
	// invalid typed fields. FieldErrors contains the per-field validation errors.
	InvalidLoadOptionsError struct {
		FieldErrors []error
	}

	// LoadResult contains the loaded configuration, the kind of source that
	// produced it, and that source's path. SourcePath is empty when defaults
	// were used.
	LoadResult struct {
		Config     *Config
		SourcePath types.FilesystemPath
		Source     ConfigSourceKind
	}

	// Loader loads configuration from explicit options.
	Loader interface {
		Load(ctx context.Context, opts LoadOptions) (*Config, error)
	}

	// SourceLoader loads configuration and reports the source path used.
	SourceLoader interface {
		LoadWithSource(ctx context.Context, opts LoadOptions) (LoadResult, error)
	}

	// Provider exposes the full configuration loading surface for CLI commands
	// that need both config data and source metadata.
	Provider interface {
		Loader
		SourceLoader
	}

	// fileProvider is the production Provider that loads configuration from the
	// filesystem. getenv is the only way it reads the environment (for
	// INVOWK_CONFIG), so tests can inject one; nil behaves as an empty environment.
	fileProvider struct {
		getenv func(string) string
	}
)

// Validate returns an error if any typed fields in the LoadOptions are invalid.
// Both fields use zero-value-is-valid semantics: empty means "use default".
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

// Validate returns nil when the result contains a config, names a loaded
// source kind (never the selection-only implicit kind), and the optional
// source path is valid.
func (r LoadResult) Validate() error {
	var errs []error
	if err := r.Source.Validate(); err != nil {
		errs = append(errs, err)
	} else if r.Source == ConfigSourceImplicit {
		errs = append(errs, errors.New("config load result must name a loaded source, not the implicit selection"))
	}
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

// NewProvider creates a configuration provider that reads INVOWK_CONFIG from
// the process environment.
func NewProvider() Provider {
	return &fileProvider{getenv: os.Getenv}
}

// NewProviderWithEnv creates a configuration provider that reads INVOWK_CONFIG
// through getenv instead of the process environment.
func NewProviderWithEnv(getenv func(string) string) Provider {
	return &fileProvider{getenv: getenv}
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
// After validating the options it selects the source with SelectSource, and
// every failure while the selection is explicit (a missing or invalid file, a
// relative INVOWK_CONFIG, an invalid result) is wrapped in ExplicitConfigError
// so callers can fail closed on ErrExplicitConfig. Invalid options (such as a
// whitespace-only --ivk-config value) return InvalidLoadOptionsError; every
// caller already treats any error for a non-empty --ivk-config as fatal.
func (p *fileProvider) LoadWithSource(ctx context.Context, opts LoadOptions) (LoadResult, error) {
	if err := opts.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("config load: %w", err)
	}
	selection := SelectSource(opts, p.getenv)
	result, err := loadSelected(ctx, opts, selection)
	if err != nil {
		return LoadResult{}, wrapExplicitSourceError(selection, err)
	}
	return result, nil
}

func loadSelected(ctx context.Context, opts LoadOptions, selection SourceSelection) (LoadResult, error) {
	if err := selection.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("config source selection: %w", err)
	}
	if selection.Problem != nil {
		return LoadResult{}, selection.Problem
	}

	cfg, sourcePath, source, err := loadWithOptions(ctx, opts, selection)
	if err != nil {
		return LoadResult{}, err
	}

	result := LoadResult{Config: cfg, SourcePath: sourcePath, Source: source}
	if err := result.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("config load result: %w", err)
	}
	return result, nil
}
