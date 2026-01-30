// SPDX-License-Identifier: MPL-2.0

package serverbase

// Option configures a Base instance.
type Option func(*Base)

// WithErrorChannel sets a custom error channel buffer size.
// Default buffer size is 1.
func WithErrorChannel(size int) Option {
	return func(b *Base) {
		// Create a new channel with the specified buffer size
		b.errCh = make(chan error, size)
	}
}
