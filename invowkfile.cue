// Invowkfile - Command definitions for invowk
// See https://github.com/invowk/invowk for documentation
//
// Available runtimes:
//   - "native": Use system shell (default)
//   - "virtual": Use built-in sh interpreter
//   - "container": Run in Docker/Podman container (requires containerfile or image)
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
// Example command with platform-specific implementations:
//   {
//     name: "build"
//     implementations: [
//       {
//         script: "make build"
//         target: {
//           runtimes: [{name: "native"}]
//           platforms: [{name: "linux", env: {PROJECT_NAME: "myproject"}}, {name: "macos"}]
//         }
//       },
//       {
//         script: "msbuild /p:Configuration=Release"
//         target: {
//           runtimes: [{name: "native"}]
//           platforms: [{name: "windows"}]
//         }
//       }
//     ]
//   }
//
// Example command with container runtime:
//   {
//     name: "docker-build"
//     implementations: [
//       {
//         script: "go build -o /workspace/bin/app ./..."
//         target: {
//           runtimes: [{name: "container", image: "golang:1.21"}]
//         }
//       }
//     ]
//   }

version: "1.0"
description: "Full example project commands"

commands: [
	{
		name:        "build"
		description: "Build the project"
		implementations: [
			{
				script: """
					echo "Building project..."
					go build -o bin/app ./...
					"""
				target: {
					runtimes: [
						{name: "native"},
						{name: "container", image: "golang:1.21"},
					]
					platforms: [
						{name: "linux", env: {PROJECT_NAME: "myproject"}},
						{name: "macos", env: {PROJECT_NAME: "myproject"}},
						{name: "windows", env: {PROJECT_NAME: "myproject"}},
					]
				}
			}
		]
		env: {
			CGO_ENABLED: "0"
		}
	},
	{
		name:        "test unit"
		description: "Run unit tests"
		implementations: [
			{
				script: "go test -v ./..."
				target: {
					runtimes: [{name: "native"}, {name: "virtual"}]
				}
			}
		]
	},
	{
		name:        "test integration"
		description: "Run integration tests"
		implementations: [
			{
				script: "go test -v -tags=integration ./..."
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
	},
	{
		name:        "clean"
		description: "Clean build artifacts"
		implementations: [
			{
				script: "rm -rf bin/ dist/"
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			},
			{
				script: "if exist bin rmdir /s /q bin && if exist dist rmdir /s /q dist"
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "windows"}]
				}
			}
		]
	},
	{
		name:        "docker-build"
		description: "Build using container runtime"
		implementations: [
			{
				script: "go build -o /workspace/bin/app ./..."
				target: {
					runtimes: [{name: "container", image: "golang:1.21"}]
				}
				// Implementation-level depends_on - validated within the container
				depends_on: {
					tools: [
						{name: "go"},
					]
					filepaths: [
						{alternatives: ["/workspace/go.mod"]},
					]
				}
			}
		]
	},
	{
		name:        "container hello-invowk"
		description: "Print a greeting from a container"
		implementations: [
			{
				script: "echo \"Hello, Invowk!\""
				target: {
					runtimes: [{name: "container", image: "alpine:latest"}]
				}
			}
		]
	},
	{
		name:        "container host-access"
		description: "Run a command in container with SSH access to host"
		implementations: [
			{
				script: """
					echo "Container can SSH to host using:"
					echo "  Host: $INVOWK_SSH_HOST"
					echo "  Port: $INVOWK_SSH_PORT"
					echo "  User: $INVOWK_SSH_USER"
					echo ""
					echo "Example: sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT 'hostname'"
					"""
				target: {
					runtimes:  [{name: "container", image: "alpine:latest", host_ssh: true}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
	},
	{
		name:        "release"
		description: "Create a release"
		implementations: [
			{
				script: "echo 'Creating release...'"
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
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
				// Simple existence check - any of the alternatives satisfies the dependency
				{alternatives: ["go.mod", "go.sum"]},
				// Check with read permission
				{alternatives: ["README.md", "README", "readme.md"], readable: true},
				// Check with write permission (for output directory)
				{alternatives: ["."], writable: true},
			]
		}
	},
]

