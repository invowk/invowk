// SPDX-License-Identifier: MPL-2.0

package config

var (
	// globalConfig holds the loaded configuration.
	globalConfig *Config
	// configPath stores the path where config was loaded from.
	configPath string
	// configDirOverride allows tests to override the config directory.
	// This is necessary because os.UserHomeDir() doesn't reliably respect
	// the HOME environment variable on all platforms (e.g., macOS in CI).
	configDirOverride string
	// configFilePathOverride allows specifying a custom config file path.
	// This is used by the --config CLI flag to load config from a specific file.
	configFilePathOverride string
	// errLastLoad stores the last error from Load() for later retrieval.
	// This allows Get() to return defaults while preserving error information.
	errLastLoad error
)

// Get returns the currently loaded configuration.
// If configuration loading fails, it returns defaults and stores the error
// for retrieval via LastLoadError().
func Get() *Config {
	if globalConfig == nil {
		cfg, err := Load()
		if err != nil {
			errLastLoad = err
			return DefaultConfig()
		}
		return cfg
	}
	return globalConfig
}

// LastLoadError returns the most recent error from configuration loading.
// This is useful for surfacing config errors to users even when defaults are used.
// Returns nil if configuration loaded successfully or was never attempted.
func LastLoadError() error {
	return errLastLoad
}

// ConfigFilePath returns the path to the config file.
//
//nolint:revive // ConfigFilePath is more descriptive than FilePath for external callers
func ConfigFilePath() string {
	return configPath
}

// Reset clears all state including cached configuration and test overrides
func Reset() {
	globalConfig = nil
	configPath = ""
	configDirOverride = ""
	configFilePathOverride = ""
	errLastLoad = nil
}

// ResetCache clears only the cached configuration, preserving any test overrides.
// This is useful when testing scenarios that require reloading the config from disk
// without losing the test's config directory override.
func ResetCache() {
	globalConfig = nil
	configPath = ""
	errLastLoad = nil
}

// SetConfigDirOverride sets a custom config directory path.
// This is primarily intended for testing to bypass os.UserHomeDir() which
// doesn't reliably respect the HOME env var on all platforms (e.g., macOS in CI).
func SetConfigDirOverride(dir string) {
	configDirOverride = dir
}

// SetConfigFilePathOverride sets a custom config file path.
// This is used by the --config CLI flag to load config from a specific file.
// When set, the specified file must exist or Load() will return an error.
// This also clears the cached configuration to force reloading from the new path.
func SetConfigFilePathOverride(path string) {
	configFilePathOverride = path
	// Clear cache to force reload from the new path
	globalConfig = nil
	configPath = ""
	errLastLoad = nil
}
