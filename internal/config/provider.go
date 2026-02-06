// SPDX-License-Identifier: MPL-2.0

package config

import "context"

// LoadOptions defines explicit configuration loading inputs.
type LoadOptions struct {
	// ConfigFilePath forces loading from a specific config file when set.
	ConfigFilePath string
	// ConfigDirPath overrides the config directory lookup when set.
	ConfigDirPath string
}

// Provider loads configuration from explicit options.
type Provider interface {
	Load(ctx context.Context, opts LoadOptions) (*Config, error)
}

type fileProvider struct{}

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
