// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const defaultGlobalModulesMountPath container.MountTargetPath = "/invowk/global-modules"

// ErrInvalidProvisionConfig is the sentinel error wrapped by InvalidProvisionConfigError.
var ErrInvalidProvisionConfig = errors.New("invalid provision config")

type (
	// Config holds configuration for auto-provisioning invowk resources into containers.
	Config struct { //nolint:recvcheck // Validate() uses value receiver (DDD immutable validation), Apply() uses pointer receiver (mutation)

		// Enabled controls whether auto-provisioning is active
		Enabled bool

		// Strict makes provisioning failure a hard error instead of falling
		// back to the unprovisioned base image.
		Strict bool

		// ForceRebuild bypasses cached images and forces a rebuild
		ForceRebuild bool

		// InvowkBinaryPath is the path to the invowk binary on the host.
		// If empty, the runtime adapter may supply the current executable.
		InvowkBinaryPath types.FilesystemPath

		// ModulesPaths are paths to module directories on the host.
		// These are discovered from config search paths.
		ModulesPaths []types.FilesystemPath

		// ModuleEntries are paths to provisioned modules with optional command
		// namespace metadata. They preserve config include aliases when modules
		// are copied into deterministic container paths.
		ModuleEntries ModuleEntries

		// GlobalModulesPaths are paths to globally trusted user command modules on the host.
		GlobalModulesPaths []types.FilesystemPath

		// GlobalModuleEntries are globally trusted user command modules with
		// optional command namespace metadata.
		GlobalModuleEntries ModuleEntries

		// BinaryMountPath is where to place the binary in the container.
		// Default: /invowk/bin
		BinaryMountPath container.MountTargetPath

		// ModulesMountPath is where to place modules in the container.
		// Default: /invowk/modules
		ModulesMountPath container.MountTargetPath

		// CacheDir is the parent directory for provision build contexts and
		// cached image metadata.
		// Default is supplied by the runtime adapter when host discovery is available.
		CacheDir types.FilesystemPath

		// TagSuffix is an optional suffix appended to provisioned image tags.
		// This enables test isolation by making each test's images unique.
		// When empty (default), standard tag format is used.
		// Can be set via INVOWK_PROVISION_TAG_SUFFIX environment variable.
		TagSuffix string
	}

	//goplint:validate-all
	//
	// ModuleEntry identifies one host module path to provision into a container.
	ModuleEntry struct {
		// Path is a host module path or a directory containing modules.
		Path types.FilesystemPath
		// CommandNamespace preserves the command-publication namespace for copied modules.
		CommandNamespace invowkmod.ModuleNamespace
	}

	// ModuleEntries is a validated collection of module provisioning entries.
	ModuleEntries []ModuleEntry

	// Option is a functional option for configuring a Config.
	Option func(*Config)

	// InvalidProvisionConfigError is returned when a Config has one or more
	// invalid typed fields. FieldErrors contains the per-field validation errors.
	InvalidProvisionConfigError struct {
		FieldErrors []error
	}
)

// Validate returns nil if all typed fields in the Config are valid,
// or an error wrapping ErrInvalidProvisionConfig if any are invalid.
// Boolean fields and TagSuffix (free-form test-only string) are skipped.
// Path fields are only validated when non-empty, since empty paths indicate
// "use default" semantics (e.g., os.Executable() for InvowkBinaryPath).
func (c Config) Validate() error {
	var errs []error
	if c.InvowkBinaryPath != "" {
		if err := c.InvowkBinaryPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.BinaryMountPath != "" {
		if err := c.BinaryMountPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.ModulesMountPath != "" {
		if err := c.ModulesMountPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.CacheDir != "" {
		if err := c.CacheDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for i, mp := range c.ModulesPaths {
		if mp != "" {
			if err := mp.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("ModulesPaths[%d]: %w", i, err))
			}
		}
	}
	if err := c.ModuleEntries.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("ModuleEntries: %w", err))
	}
	for i, mp := range c.GlobalModulesPaths {
		if mp != "" {
			if err := mp.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("GlobalModulesPaths[%d]: %w", i, err))
			}
		}
	}
	if err := c.GlobalModuleEntries.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("GlobalModuleEntries: %w", err))
	}
	if len(errs) > 0 {
		return &InvalidProvisionConfigError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil when the entry's path and optional namespace are valid.
func (e ModuleEntry) Validate() error {
	var errs []error
	if err := e.Path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if e.CommandNamespace != "" {
		if err := e.CommandNamespace.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Validate returns nil when all module provisioning entries are valid.
func (e ModuleEntries) Validate() error {
	var errs []error
	for i, entry := range e {
		if err := entry.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
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

// DefaultConfig returns pure provisioning defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
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

// WithModuleEntries returns an Option that sets ModuleEntries on the config.
func WithModuleEntries(entries ModuleEntries) Option {
	return func(c *Config) {
		c.ModuleEntries = entries
	}
}

// WithGlobalModulesPaths returns an Option that sets GlobalModulesPaths on the config.
func WithGlobalModulesPaths(paths []types.FilesystemPath) Option {
	return func(c *Config) {
		c.GlobalModulesPaths = paths
	}
}

// WithGlobalModuleEntries returns an Option that sets GlobalModuleEntries on the config.
func WithGlobalModuleEntries(entries ModuleEntries) Option {
	return func(c *Config) {
		c.GlobalModuleEntries = entries
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
