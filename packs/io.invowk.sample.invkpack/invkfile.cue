// Sample invowk pack demonstrating a minimal cross-platform command.
// This pack serves as a reference implementation and validation test.

group: "io.invowk.sample"
version: "1.0"
description: "Sample invowk pack with a simple cross-platform greeting command"

commands: [
	{
		name:        "hello"
		description: "Print a friendly greeting from invowk"
		implementations: [
			{
				script: "echo \"Hello, I'm invowk!\""
				target: {
					// Compatible with all runtimes
					runtimes: [
						{name: "native"},
						{name: "virtual"},
						{name: "container", image: "alpine:latest"},
					]
					// No platforms specified = runs on all platforms (linux, macos, windows)
				}
			},
		]
	},
]
