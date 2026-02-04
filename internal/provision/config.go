// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"os"
	"path/filepath"

	"invowk-cli/internal/config"
)

type (
	// Config holds configuration for auto-provisioning invowk resources into containers.
	Config struct {
		// Enabled controls whether auto-provisioning is active
		Enabled bool

		// ForceRebuild bypasses cached images and forces a rebuild
		ForceRebuild bool

		// InvowkBinaryPath is the path to the invowk binary on the host.
		// If empty, os.Executable() will be used.
		InvowkBinaryPath string

		// ModulesPaths are paths to module directories on the host.
		// These are discovered from config search paths and user commands dir.
		ModulesPaths []string

		// InvkfilePath is the path to the current invkfile being executed.
		// This is used to determine what needs to be provisioned.
		InvkfilePath string

		// BinaryMountPath is where to place the binary in the container.
		// Default: /invowk/bin
		BinaryMountPath string

		// ModulesMountPath is where to place modules in the container.
		// Default: /invowk/modules
		ModulesMountPath string

		// CacheDir is where to store cached provisioned images metadata.
		// Default: ~/.cache/invowk/provision
		CacheDir string

		// TagSuffix is an optional suffix appended to provisioned image tags.
		// This enables test isolation by making each test's images unique.
		// When empty (default), standard tag format is used.
		// Can be set via INVOWK_PROVISION_TAG_SUFFIX environment variable.
		TagSuffix string
	}

	// Option is a functional option for configuring a Config.
	Option func(*Config)
)

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	binaryPath, _ := os.Executable()

	// Discover module paths from user commands dir and config
	var modulesPaths []string
	if userDir, err := config.CommandsDir(); err == nil {
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			modulesPaths = append(modulesPaths, userDir)
		}
	}

	cacheDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir = filepath.Join(home, ".cache", "invowk", "provision")
	}

	// Read tag suffix from environment (for test isolation)
	tagSuffix := os.Getenv("INVOWK_PROVISION_TAG_SUFFIX")

	return &Config{
		Enabled:          true,
		ForceRebuild:     false,
		InvowkBinaryPath: binaryPath,
		ModulesPaths:     modulesPaths,
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
		CacheDir:         cacheDir,
		TagSuffix:        tagSuffix,
	}
}

// WithForceRebuild returns an Option that sets ForceRebuild on the config.
func WithForceRebuild(force bool) Option {
	return func(c *Config) {
		c.ForceRebuild = force
	}
}

// WithEnabled returns an Option that sets Enabled on the config.
func WithEnabled(enabled bool) Option {
	return func(c *Config) {
		c.Enabled = enabled
	}
}

// WithInvowkBinaryPath returns an Option that sets InvowkBinaryPath on the config.
func WithInvowkBinaryPath(path string) Option {
	return func(c *Config) {
		c.InvowkBinaryPath = path
	}
}

// WithModulesPaths returns an Option that sets ModulesPaths on the config.
func WithModulesPaths(paths []string) Option {
	return func(c *Config) {
		c.ModulesPaths = paths
	}
}

// WithInvkfilePath returns an Option that sets InvkfilePath on the config.
func WithInvkfilePath(path string) Option {
	return func(c *Config) {
		c.InvkfilePath = path
	}
}

// WithCacheDir returns an Option that sets CacheDir on the config.
func WithCacheDir(dir string) Option {
	return func(c *Config) {
		c.CacheDir = dir
	}
}

// WithTagSuffix returns an Option that sets TagSuffix on the config.
// This is primarily used for test isolation to ensure parallel tests
// don't compete for the same provisioned image tags.
func WithTagSuffix(suffix string) Option {
	return func(c *Config) {
		c.TagSuffix = suffix
	}
}

// Apply applies the given options to the config.
func (c *Config) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}
