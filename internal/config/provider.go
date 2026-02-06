// SPDX-License-Identifier: MPL-2.0

package config

import "context"

type (
	// LoadOptions defines explicit configuration loading inputs.
	LoadOptions struct {
		// ConfigFilePath forces loading from a specific config file when set.
		ConfigFilePath string
		// ConfigDirPath overrides the config directory lookup when set.
		ConfigDirPath string
	}

	// Provider loads configuration from explicit options rather than package-level
	// state. This replaces the previous global config accessors and enables testing
	// with custom config sources or mock implementations.
	Provider interface {
		Load(ctx context.Context, opts LoadOptions) (*Config, error)
	}

	// fileProvider is the production Provider that loads configuration from the filesystem.
	fileProvider struct{}
)

// NewProvider creates a configuration provider.
func NewProvider() Provider {
	return &fileProvider{}
}

// Load reads configuration from the requested source.
func (p *fileProvider) Load(ctx context.Context, opts LoadOptions) (*Config, error) {
	cfg, _, err := loadWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
