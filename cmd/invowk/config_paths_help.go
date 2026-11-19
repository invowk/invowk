// SPDX-License-Identifier: MPL-2.0

package cmd

// Help text that names the per-OS user config paths. It lives in its own
// file because it spells base-directory variables ($XDG_CONFIG_HOME,
// %APPDATA%), which the internal/basedir guard allowlists only here; keep
// this file free of code.
const (
	// userConfigPathsHelp is the user-config entry of 'invowk config --help'.
	userConfigPathsHelp = `     - Linux: $XDG_CONFIG_HOME/invowk/config.cue, or ~/.config/invowk/config.cue
     - macOS: ~/Library/Application Support/invowk/config.cue
     - Windows: %APPDATA%\invowk\config.cue`

	// userConfigPathsFlagHelp is the path list in the --ivk-config usage.
	userConfigPathsFlagHelp = "Linux: $XDG_CONFIG_HOME/invowk/config.cue or ~/.config/invowk/config.cue, " +
		"macOS: ~/Library/Application Support/invowk/config.cue, Windows: %APPDATA%\\invowk\\config.cue"
)
