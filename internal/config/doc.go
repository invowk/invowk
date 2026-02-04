// SPDX-License-Identifier: MPL-2.0

// Package config handles application configuration using Viper with CUE as the file format.
//
// Configuration is loaded from ~/.config/invowk/config.cue (or XDG equivalent on Linux,
// ~/Library/Application Support/invowk/config.cue on macOS, %APPDATA%\invowk\config.cue
// on Windows). The package provides type-safe configuration access and supports container
// engine selection, search paths, virtual shell options, UI settings, and auto-provisioning.
//
// Configuration validation is performed against a CUE schema (config_schema.cue) to ensure
// type safety and provide clear error messages for invalid configurations.
package config
