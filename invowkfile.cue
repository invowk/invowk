// Invowkfile - Command definitions for invowk
// See https://github.com/invowk/invowk for documentation
//
// Available runtimes:
//   - "native": Use system shell (default)
//   - "virtual": Use built-in sh interpreter
//   - "container": Run in Docker/Podman container
//
// Available platforms:
//   - "linux": Linux operating systems
//   - "macos": macOS (Darwin)
//   - "windows": Windows
//
// Scripts can be:
//   - Inline shell commands (single or multi-line using triple quotes)
//   - A path to a script file (e.g., "./scripts/build.sh")
//
// Example command with platform-specific scripts:
//   {
//     name: "build"
//     scripts: [
//       {
//         script: "make build"
//         runtimes: ["native"]
//         platforms: ["linux", "macos"]
//       },
//       {
//         script: "msbuild /p:Configuration=Release"
//         runtimes: ["native"]
//         platforms: ["windows"]
//       }
//     ]
//   }
//
// Example command with script that runs on all platforms:
//   {
//     name: "test"
//     scripts: [
//       {
//         script: "go test ./..."
//         runtimes: ["native", "virtual"]
//         // No platforms = runs on all platforms
//       }
//     ]
//   }

version: "1.0"
description: "Full example project commands"
default_runtime: "native"

container: {
	dockerfile: "Dockerfile"
	image:      "alpine:latest"
}

env: {
	PROJECT_NAME: "myproject"
}

commands: [
	{
		name:        "build"
		description: "Build the project"
		scripts: [
			{
				script: """
					echo "Building $PROJECT_NAME..."
					go build -o bin/app ./...
					"""
				runtimes: ["native", "container"]
				// No platforms = all platforms
			}
		]
		env: {
			CGO_ENABLED: "0"
		}
	},
	{
		name:        "test unit"
		description: "Run unit tests"
		scripts: [
			{
				script:   "go test -v ./..."
				runtimes: ["native", "virtual"]
			}
		]
	},
	{
		name:        "test integration"
		description: "Run integration tests"
		scripts: [
			{
				script:   "go test -v -tags=integration ./..."
				runtimes: ["native"]
			}
		]
	},
	{
		name:        "clean"
		description: "Clean build artifacts"
		scripts: [
			{
				script:    "rm -rf bin/ dist/"
				runtimes:  ["native"]
				platforms: ["linux", "macos"]
			},
			{
				script:    "if exist bin rmdir /s /q bin && if exist dist rmdir /s /q dist"
				runtimes:  ["native"]
				platforms: ["windows"]
			}
		]
	},
	{
		name:        "docker-build"
		description: "Build using container runtime"
		scripts: [
			{
				script:   "go build -o /workspace/bin/app ./..."
				runtimes: ["container"]
			}
		]
	},
	{
		name:        "container hello-invowk"
		description: "Print a greeting from a container"
		scripts: [
			{
				script:   "echo \"Hello, Invowk!\""
				runtimes: ["container"]
			}
		]
	},
	{
		name:        "container host-access"
		description: "Run a command in container with SSH access to host"
		scripts: [
			{
				script: """
					echo "Container can SSH to host using:"
					echo "  Host: $INVOWK_SSH_HOST"
					echo "  Port: $INVOWK_SSH_PORT"
					echo "  User: $INVOWK_SSH_USER"
					echo ""
					echo "Example: sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT 'hostname'"
					"""
				runtimes:  ["container"]
				platforms: ["linux", "macos"]
				host_ssh:  true
			}
		]
	},
	{
		name:        "release"
		description: "Create a release"
		scripts: [
			{
				script:    "echo 'Creating release...'"
				runtimes:  ["native"]
				platforms: ["linux", "macos"]
			}
		]
		depends_on: {
			tools: [
				// Simple tool check - just verify it's in PATH
				{name: "git"},
				// Tool with custom validation script and output regex
				{
					name:            "go"
					check_script:    "go version"
					expected_code:   0
					expected_output: "go1\\."  // Regex: must contain "go1."
				},
			]
			commands: [
				{name: "clean"},
				{name: "build"},
				{name: "test unit"},
			]
			filepaths: [
				// Simple existence check
				{path: "go.mod"},
				// Check with read permission
				{path: "README.md", readable: true},
				// Check with write permission (for output directory)
				{path: ".", writable: true},
			]
		}
	},
]

