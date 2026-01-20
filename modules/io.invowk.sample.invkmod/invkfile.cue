// Sample invowk module commands - demonstrates the invkfile.cue format.
// This file contains command definitions.
// Module metadata (name, version, dependencies) is in invkmod.cue.

cmds: [
	{
		name:        "hello"
		description: "Print a friendly greeting from invowk"
		implementations: [
			{
				script: "echo \"Hello, I'm invowk!\""
				// Compatible with all runtimes
				runtimes: [
					{name: "native"},
					{name: "virtual"},
					{name: "container", image: "debian:stable-slim"},
				]
				// No platforms specified = runs on all platforms (linux, macos, windows)
			},
		]
	},
]
