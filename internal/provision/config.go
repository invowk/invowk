// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidProvisionConfig is the sentinel error wrapped by InvalidProvisionConfigError.
var ErrInvalidProvisionConfig = errors.New("invalid provision config")

type (
	// Config holds configuration for auto-provisioning invowk resources into containers.
	Config struct {
		// Enabled controls whether auto-provisioning is active
		Enabled bool

		// Strict makes provisioning failure a hard error instead of falling
		// back to the unprovisioned base image.
		Strict bool

		// ForceRebuild bypasses cached images and forces a rebuild
		ForceRebuild bool

		// InvowkBinaryPath is the path to the invowk binary on the host.
		// If empty, os.Executable() will be used.
		InvowkBinaryPath types.FilesystemPath

		// ModulesPaths are paths to module directories on the host.
		// These are discovered from config search paths and user commands dir.
		ModulesPaths []types.FilesystemPath

		// InvowkfilePath is the path to the current invowkfile being executed.
		// This is used to determine what needs to be provisioned.
		InvowkfilePath types.FilesystemPath

		// BinaryMountPath is where to place the binary in the container.
		// Default: /invowk/bin
		BinaryMountPath container.MountTargetPath

		// ModulesMountPath is where to place modules in the container.
		// Default: /invowk/modules
		ModulesMountPath container.MountTargetPath

		// CacheDir is where to store cached provisioned images metadata.
		// Default: ~/.cache/invowk/provision
		CacheDir types.FilesystemPath

		// TagSuffix is an optional suffix appended to provisioned image tags.
		// This enables test isolation by making each test's images unique.
		// When empty (default), standard tag format is used.
		// Can be set via INVOWK_PROVISION_TAG_SUFFIX environment variable.
		TagSuffix string
	}

	// Option is a functional option for configuring a Config.
	Option func(*Config)

	// InvalidProvisionConfigError is returned when a Config has one or more
	// invalid typed fields. FieldErrors contains the per-field validation errors.
	InvalidProvisionConfigError struct {
		FieldErrors []error
	}
)

// IsValid returns whether all typed fields in the Config are valid.
// Boolean fields and TagSuffix (free-form test-only string) are skipped.
// Path fields are only validated when non-empty, since empty paths indicate
// "use default" semantics (e.g., os.Executable() for InvowkBinaryPath).
func (c Config) IsValid() (bool, []error) {
	var errs []error
	if c.InvowkBinaryPath != "" {
		if valid, fieldErrs := c.InvowkBinaryPath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	if c.InvowkfilePath != "" {
		if valid, fieldErrs := c.InvowkfilePath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	if c.BinaryMountPath != "" {
		if valid, fieldErrs := c.BinaryMountPath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	if c.ModulesMountPath != "" {
		if valid, fieldErrs := c.ModulesMountPath.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	if c.CacheDir != "" {
		if valid, fieldErrs := c.CacheDir.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	for i, mp := range c.ModulesPaths {
		if mp != "" {
			if valid, fieldErrs := mp.IsValid(); !valid {
				errs = append(errs, fmt.Errorf("ModulesPaths[%d]: %w", i, errors.Join(fieldErrs...)))
			}
		}
	}
	if len(errs) > 0 {
		return false, []error{&InvalidProvisionConfigError{FieldErrors: errs}}
	}
	return true, nil
}

// Error implements the error interface for InvalidProvisionConfigError.
func (e *InvalidProvisionConfigError) Error() string {
	if len(e.FieldErrors) == 1 {
		return fmt.Sprintf("invalid provision config: %v", e.FieldErrors[0])
	}
	return fmt.Sprintf("invalid provision config: %d field errors", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidProvisionConfig for errors.Is() compatibility.
func (e *InvalidProvisionConfigError) Unwrap() error { return ErrInvalidProvisionConfig }

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	binaryPath, _ := os.Executable()

	// Discover module paths from user commands dir
	var modulesPaths []types.FilesystemPath
	if userDir, err := config.CommandsDir(); err == nil {
		if info, err := os.Stat(string(userDir)); err == nil && info.IsDir() {
			modulesPaths = append(modulesPaths, userDir)
		}
	}

	var cacheDir types.FilesystemPath
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir = types.FilesystemPath(filepath.Join(home, ".cache", "invowk", "provision"))
	}

	// Read tag suffix from environment (for test isolation)
	tagSuffix := os.Getenv("INVOWK_PROVISION_TAG_SUFFIX")

	return &Config{
		Enabled:          true,
		ForceRebuild:     false,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
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
func WithInvowkBinaryPath(path types.FilesystemPath) Option {
	return func(c *Config) {
		c.InvowkBinaryPath = path
	}
}

// WithModulesPaths returns an Option that sets ModulesPaths on the config.
func WithModulesPaths(paths []types.FilesystemPath) Option {
	return func(c *Config) {
		c.ModulesPaths = paths
	}
}

// WithInvowkfilePath returns an Option that sets InvowkfilePath on the config.
func WithInvowkfilePath(path types.FilesystemPath) Option {
	return func(c *Config) {
		c.InvowkfilePath = path
	}
}

// WithCacheDir returns an Option that sets CacheDir on the config.
func WithCacheDir(dir types.FilesystemPath) Option {
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
