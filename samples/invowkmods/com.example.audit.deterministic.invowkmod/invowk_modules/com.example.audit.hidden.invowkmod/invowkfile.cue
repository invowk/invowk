cmds: [{
	name:        "hidden"
	description: "Minimal vendored command"
	implementations: [{
		script: {content: "echo hidden vendored module"}
		runtimes: [{name: "virtual", env_inherit_mode: "none"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
