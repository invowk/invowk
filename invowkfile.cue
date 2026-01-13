// Invowkfile - Command definitions for invowk
// See https://github.com/invowk/invowk for documentation
//
// Available runtimes:
//   - "native": Use system shell (default)
//   - "virtual": Use built-in sh interpreter
//   - "container": Run in Docker/Podman container
//
// Script can be:
//   - Inline shell commands (single or multi-line using triple quotes)
//   - A path to a script file (e.g., "./scripts/build.sh")
//
// Example command with inline script:
//   {
//     name: "build"
//     description: "Build the project"
//     script: """
//       echo "Building..."
//       go build ./...
//       """
//   }
//
// Example command with script file:
//   {
//     name: "deploy"
//     script: "./scripts/deploy.sh"
//   }
//
// Use spaces in names for subcommand-like behavior:
//   {
//     name: "test unit"
//     script: "go test ./..."
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

commands: [{
		name:        "test unit"
		description: "Run unit tests"
		runtimes:    ["native", "virtual"]
		scripts:     [
			{
				runtime: "native"
				host_os: "windows"
				script: "go test -v ./..."
			},
			{
				runtime: "native"
				target_os: "linux"
				script: "go test -v ./..."
			},
			{
				runtime: "native"
				target_os: "windows"
				script: "go test -v ./..."
			}
		]
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
]

commands: [
	{
		name:        "build"
		description: "Build the project"
		runtimes:    ["native", "container"]
		script: """
			echo "Building $PROJECT_NAME..."
			go build -o bin/app ./...
			"""
		env: {
			CGO_ENABLED: "0"
		}
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name:        "test unit"
		description: "Run unit tests"
		runtimes:    ["native", "virtual"]
		script:      "go test -v ./..."
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name:        "test integration"
		description: "Run integration tests"
		runtimes:    ["native"]
		script:      "go test -v -tags=integration ./..."
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name:        "clean"
		description: "Clean build artifacts"
		runtimes:    ["native"]
		script:      "rm -rf bin/ dist/"
		works_on: {
			hosts: ["linux", "mac"]
		}
	},
	{
		name:        "docker-build"
		description: "Build using container runtime"
		runtimes:    ["container"]
		script:      "go build -o /workspace/bin/app ./..."
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name:        "container hello-invowk"
		description: "Print a greeting from a container"
		runtimes:    ["container"]
		script:      "echo \"Hello, Invowk!\""
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name:        "container host-access"
		description: "Run a command in container with SSH access to host"
		runtimes:    ["container"]
		host_ssh:    true
		script: """
			echo "Container can SSH to host using:"
			echo "  Host: $INVOWK_SSH_HOST"
			echo "  Port: $INVOWK_SSH_PORT"
			echo "  User: $INVOWK_SSH_USER"
			echo ""
			echo "Example: sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT 'hostname'"
			"""
		works_on: {
			hosts: ["linux", "mac"]
		}
	},
	{
		name:        "release"
		description: "Create a release"
		runtimes:    ["native"]
		script:      "echo 'Creating release...'"
		works_on: {
			hosts: ["linux", "mac"]
		}
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

