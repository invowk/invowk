// Sample invowk pack commands - demonstrates the invkfile.cue format.
// This file contains command definitions.
// Pack metadata (name, version, dependencies) is in invkpack.cue.

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
					{name: "container", image: "alpine:latest"},
				]
				// No platforms specified = runs on all platforms (linux, macos, windows)
			},
		]
	},
]
