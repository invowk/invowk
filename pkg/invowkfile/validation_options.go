// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"io/fs"
	"os"
	"path/filepath"
)

type (
	// validateOptions holds configuration for validation.
	validateOptions struct {
		validators           []Validator
		additionalValidators []Validator
		fs                   fs.FS
		platform             PlatformType
		strictMode           bool
		workDir              string
	}

	// ValidateOption configures validation behavior.
	ValidateOption func(*validateOptions)
)

// defaultValidateOptions returns the default validation options for an invowkfile.
func defaultValidateOptions(inv *Invowkfile) validateOptions {
	workDir := ""
	if inv != nil && inv.FilePath != "" {
		workDir = filepath.Dir(inv.FilePath)
	}

	return validateOptions{
		validators: nil, // nil means use DefaultValidators()
		fs:         nil, // nil means use os.DirFS
		platform:   "",  // empty means current platform
		strictMode: false,
		workDir:    workDir,
	}
}

// WithValidators replaces the default validators with the specified validators.
// Use this to completely customize which validators run.
func WithValidators(validators ...Validator) ValidateOption {
	return func(o *validateOptions) {
		o.validators = validators
	}
}

// WithAdditionalValidators adds validators to run after the default validators.
// Use this to extend validation with custom checks without replacing the defaults.
func WithAdditionalValidators(validators ...Validator) ValidateOption {
	return func(o *validateOptions) {
		o.additionalValidators = append(o.additionalValidators, validators...)
	}
}

// WithFS sets the filesystem to use for file existence checks during validation.
// This is useful for testing or validating against a virtual filesystem.
// The fs.FS should be rooted at the invowkfile's directory.
//
// Example for testing:
//
//	testFS := fstest.MapFS{
//	    "Containerfile": &fstest.MapFile{Data: []byte("FROM debian:stable-slim")},
//	}
//	errs := inv.Validate(WithFS(testFS))
func WithFS(filesystem fs.FS) ValidateOption {
	return func(o *validateOptions) {
		o.fs = filesystem
	}
}

// WithPlatform sets the target platform for validation.
// Use PlatformLinux, PlatformMac, PlatformWindows, or zero value for current platform.
func WithPlatform(platform PlatformType) ValidateOption {
	return func(o *validateOptions) {
		o.platform = platform
	}
}

// WithStrictMode enables strict validation mode.
// In strict mode, warnings are treated as errors.
func WithStrictMode(strict bool) ValidateOption {
	return func(o *validateOptions) {
		o.strictMode = strict
	}
}

// WithWorkDir overrides the working directory for path resolution.
// By default, paths are resolved relative to the invowkfile's directory.
func WithWorkDir(workDir string) ValidateOption {
	return func(o *validateOptions) {
		o.workDir = workDir
	}
}

// buildValidationContext creates a ValidationContext from the options.
func (o *validateOptions) buildValidationContext(inv *Invowkfile) *ValidationContext {
	filesystem := o.fs
	if filesystem == nil {
		// Default to os.DirFS rooted at the working directory
		if o.workDir != "" {
			filesystem = os.DirFS(o.workDir)
		} else {
			filesystem = os.DirFS(".")
		}
	}

	filePath := ""
	if inv != nil {
		filePath = inv.FilePath
	}

	return &ValidationContext{
		WorkDir:    o.workDir,
		FS:         filesystem,
		Platform:   o.platform,
		StrictMode: o.strictMode,
		FilePath:   filePath,
	}
}

// getValidators returns the validators to use based on the options.
func (o *validateOptions) getValidators() []Validator {
	var validators []Validator

	if o.validators != nil {
		// Use explicitly specified validators
		validators = o.validators
	} else {
		// Use default validators
		validators = DefaultValidators()
	}

	// Add any additional validators
	validators = append(validators, o.additionalValidators...)

	return validators
}
