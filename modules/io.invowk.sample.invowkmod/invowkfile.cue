// Sample invowk module commands - demonstrates the invowkfile.cue format.
// This file contains command definitions.
// Module metadata (name, version, dependencies) is in invowkmod.cue.

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
					// debian:stable-slim is a floating tag (project convention).
					// For production modules, pin to a digest: "debian@sha256:<hash>"
					{name: "container", image: "debian:stable-slim"},
				]
				platforms: [
					{name: "linux"},
					{name: "macos"},
					{name: "windows"},
				]
			},
		]
	},
]
