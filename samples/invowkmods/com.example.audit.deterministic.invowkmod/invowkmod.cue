module: "com.example.audit.deterministic"
version: "0.1.0"
description: "Intentionally unsafe module for deterministic audit checker smoke tests"
license: "MPL-2.0"
repository: "https://github.com/invowk/invowk"

requires: [
	{
		git_url: "https://github.com/example/alpha.git"
		version: ">0"
	},
	{
		git_url: "https://github.com/example/beta.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/example/gamma.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/example/delta.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/example/epsilon.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/example/zeta.git"
		version: "^1.0.0"
	},
]
