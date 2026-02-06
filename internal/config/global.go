// SPDX-License-Identifier: MPL-2.0

package config

// configDirOverride allows tests to override the config directory.
// This is necessary because os.UserHomeDir() doesn't reliably respect
// the HOME environment variable on all platforms (e.g., macOS in CI).
var configDirOverride string

// Reset clears test overrides. Call from test cleanup to restore defaults.
func Reset() {
	configDirOverride = ""
}

// SetConfigDirOverride sets a custom config directory path.
// This is primarily intended for testing to bypass os.UserHomeDir() which
// doesn't reliably respect the HOME env var on all platforms (e.g., macOS in CI).
func SetConfigDirOverride(dir string) {
	configDirOverride = dir
}
