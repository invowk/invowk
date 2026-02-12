// SPDX-License-Identifier: MPL-2.0

package cueutil

// DefaultMaxFileSize is the default maximum file size for CUE parsing (5MB).
// This limit prevents OOM attacks from maliciously large configuration files.
const DefaultMaxFileSize int64 = 5 * 1024 * 1024

type (
	// parseOptions holds configuration for CUE parsing.
	parseOptions struct {
		maxFileSize int64
		concrete    bool
		filename    string
	}

	// Option configures parsing behavior.
	Option func(*parseOptions)
)

// defaultOptions returns the default parse options.
func defaultOptions() parseOptions {
	return parseOptions{
		maxFileSize: DefaultMaxFileSize,
		concrete:    true,
		filename:    "",
	}
}

// WithMaxFileSize sets the maximum allowed file size.
// Default is DefaultMaxFileSize (5MB).
func WithMaxFileSize(size int64) Option {
	return func(o *parseOptions) {
		o.maxFileSize = size
	}
}

// WithConcrete sets whether all values must be concrete after unification.
// Default is true (require concrete values).
//
// Set to false for config files where some fields may be optional and
// unset values are acceptable.
func WithConcrete(concrete bool) Option {
	return func(o *parseOptions) {
		o.concrete = concrete
	}
}

// WithFilename sets the filename for error messages.
// This appears in CUE error output to help users locate issues.
func WithFilename(name string) Option {
	return func(o *parseOptions) {
		o.filename = name
	}
}
