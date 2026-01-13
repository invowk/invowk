// Invowkfile - Example command definitions for invowk
// See https://github.com/invowk/invowk for documentation
//
// This file contains example commands that demonstrate all invowk features.
// All commands are idempotent and do not cause side effects on the host.
//
// The 'group' field is mandatory and prefixes all command names.
// Example: group: "examples" means command "hello" becomes "invowk cmd examples hello"

group:       "examples"
version:     "1.0"
description: "Example commands demonstrating all invowk features"

commands: [
	// ============================================================================
	// SECTION 1: Simple Commands (Native Runtime)
	// ============================================================================
	// These commands demonstrate basic native shell execution with minimal config.

	// Example 1.1: Simplest possible command - native runtime, all platforms
	{
		name:        "hello"
		description: "Print a simple greeting (native runtime)"
		implementations: [
			{
				script: "echo 'Hello from invowk!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
	},

	// Example 1.2: Command with environment variables
	{
		name:        "hello env"
		description: "Print greeting using environment variables"
		implementations: [
			{
				script: "echo \"Hello, $USER_NAME! Today is $DAY_OF_WEEK.\""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		env: {
			USER_NAME:   "World"
			DAY_OF_WEEK: "a great day"
		}
	},

	// Example 1.3: Multi-line script with platform-specific implementations
	{
		name:        "system info"
		description: "Display system information"
		implementations: [
			{
				script: """
					echo "=== System Information ==="
					echo "Hostname: $(hostname)"
					echo "Kernel: $(uname -r)"
					echo "User: $(whoami)"
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			},
			{
				script: """
					echo === System Information ===
					echo Hostname: %COMPUTERNAME%
					echo User: %USERNAME%
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "windows"}]
				}
			}
		]
	},

	// Example 1.4: Platform-specific environment variables
	{
		name:        "platform env"
		description: "Demonstrate platform-specific environment variables"
		implementations: [
			{
				script: "echo \"Platform: $PLATFORM_NAME, Shell: $SHELL_TYPE\""
				target: {
					runtimes: [{name: "native"}]
					platforms: [
						{name: "linux", env: {PLATFORM_NAME: "Linux", SHELL_TYPE: "bash/sh"}},
						{name: "macos", env: {PLATFORM_NAME: "macOS", SHELL_TYPE: "zsh/bash"}},
					]
				}
			},
			{
				script: "echo Platform: %PLATFORM_NAME%, Shell: %SHELL_TYPE%"
				target: {
					runtimes: [{name: "native"}]
					platforms: [
						{name: "windows", env: {PLATFORM_NAME: "Windows", SHELL_TYPE: "PowerShell/cmd"}},
					]
				}
			}
		]
	},

	// ============================================================================
	// SECTION 2: Virtual Runtime Commands
	// ============================================================================
	// These commands demonstrate the built-in virtual shell interpreter.

	// Example 2.1: Simple virtual shell command
	{
		name:        "virtual hello"
		description: "Print greeting using virtual shell interpreter"
		implementations: [
			{
				script: "echo 'Hello from the virtual shell!'"
				target: {
					runtimes: [{name: "virtual"}]
				}
			}
		]
	},

	// Example 2.2: Command with both native and virtual runtime options
	{
		name:        "multi runtime"
		description: "Command that can run in native or virtual runtime"
		implementations: [
			{
				script: "echo 'This can run in native (default) or virtual runtime'"
				target: {
					runtimes: [{name: "native"}, {name: "virtual"}]
				}
			}
		]
	},

	// ============================================================================
	// SECTION 3: Container Runtime Commands
	// ============================================================================
	// These commands demonstrate container-based execution with Docker/Podman.

	// Example 3.1: Simple container command
	{
		name:        "container hello"
		description: "Print greeting from inside a container"
		implementations: [
			{
				script: "echo 'Hello from inside the container!'"
				target: {
					runtimes: [{name: "container", image: "alpine:latest"}]
				}
			}
		]
	},

	// Example 3.2: Container with volume mounts
	{
		name:        "container volumes"
		description: "Demonstrate volume mounts in container"
		implementations: [
			{
				script: """
					echo "=== Volume Mount Demo ==="
					echo ""
					echo "This container has custom volume mounts configured."
					echo "The current directory is mounted at /app-data (read-only)."
					echo ""
					echo "Contents of /app-data:"
					ls /app-data 2>/dev/null | head -10 || echo "(empty or not accessible)"
					echo ""
					echo "File count: $(ls /app-data 2>/dev/null | wc -l) files/directories"
					"""
				target: {
					runtimes: [{
						name:  "container"
						image: "alpine:latest"
						volumes: [
							".:/app-data:ro",
						]
					}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
	},

	// Example 3.3: Container with port mappings
	{
		name:        "container ports"
		description: "Demonstrate port mappings in container (prints info only)"
		implementations: [
			{
				script: """
					echo "=== Port Mapping Demo ==="
					echo "This container has port mappings configured:"
					echo "  - Host 8080 -> Container 80"
					echo "  - Host 3000 -> Container 3000"
					echo ""
					echo "Note: No actual server is started (this is just a demo)"
					"""
				target: {
					runtimes: [{
						name:  "container"
						image: "alpine:latest"
						ports: ["8080:80", "3000:3000"]
					}]
				}
			}
		]
	},

	// Example 3.4: Container with environment variables
	{
		name:        "container env"
		description: "Demonstrate environment variables in container"
		implementations: [
			{
				script: """
					echo "=== Container Environment ==="
					echo "APP_NAME: $APP_NAME"
					echo "APP_ENV: $APP_ENV"
					echo "DEBUG: $DEBUG"
					"""
				target: {
					runtimes: [{name: "container", image: "alpine:latest"}]
				}
			}
		]
		env: {
			APP_NAME: "demo-app"
			APP_ENV:  "development"
			DEBUG:    "true"
		}
	},

	// Example 3.5: Container with host SSH access enabled
	{
		name:        "container ssh"
		description: "Container with SSH access back to host"
		implementations: [
			{
				script: #"""
					echo "=== Host SSH Access Demo ==="
					echo "SSH connection details provided by invowk:"
					echo "  Host:  $INVOWK_SSH_HOST"
					echo "  Port:  $INVOWK_SSH_PORT"
					echo "  User:  $INVOWK_SSH_USER"
					echo "  Token: (hidden for security)"
					echo ""
					echo "To connect to host from container, use:"
					echo "  sshpass -p \$INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no \\"
					echo "    \$INVOWK_SSH_USER@\$INVOWK_SSH_HOST -p \$INVOWK_SSH_PORT 'command'"
					"""#
				target: {
					runtimes: [{
						name:            "container"
						image:           "alpine:latest"
						enable_host_ssh: true
					}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
	},

	// Example 3.6: Container without host SSH (explicit false)
	{
		name:        "container isolated"
		description: "Container without host SSH access (isolated)"
		implementations: [
			{
				script: #"""
					echo "=== Isolated Container ==="
					echo "This container has NO SSH access to the host."
					echo "It runs in complete isolation."
					echo ""
					echo "Checking if SSH env vars are set (they should NOT be):"
					echo "  INVOWK_SSH_HOST: ${INVOWK_SSH_HOST:-<not set>}"
					echo "  INVOWK_SSH_PORT: ${INVOWK_SSH_PORT:-<not set>}"
					"""#
				target: {
					runtimes: [{
						name:            "container"
						image:           "alpine:latest"
						enable_host_ssh: false
					}]
				}
			}
		]
	},

	// ============================================================================
	// SECTION 4: Tool Dependencies
	// ============================================================================
	// These commands demonstrate tool/binary dependency checking.

	// Example 4.1: Single tool dependency (no alternatives)
	{
		name:        "deps tool single"
		description: "Command requiring a single tool (sh)"
		implementations: [
			{
				script: "echo 'sh is available!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["sh"]},
			]
		}
	},

	// Example 4.2: Tool dependency with alternatives (OR semantics)
	{
		name:        "deps tool alternatives"
		description: "Command requiring any of: podman, docker, or nerdctl"
		implementations: [
			{
				script: """
					echo "Container runtime check passed!"
					echo "At least one of podman/docker/nerdctl is available."
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			tools: [
				// Any one of these satisfies the dependency
				{alternatives: ["podman", "docker", "nerdctl"]},
			]
		}
	},

	// Example 4.3: Multiple tool dependencies with mixed alternatives
	{
		name:        "deps tools mixed"
		description: "Command with multiple tool dependencies"
		implementations: [
			{
				script: """
					echo "All tool dependencies satisfied!"
					echo "  - sh is available"
					echo "  - Either cat or type is available"
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["sh"]},           // Required: sh
				{alternatives: ["cat", "type"]},  // Required: cat OR type
			]
		}
	},

	// ============================================================================
	// SECTION 5: Filepath Dependencies
	// ============================================================================
	// These commands demonstrate file/directory dependency checking.

	// Example 5.1: Single filepath dependency
	{
		name:        "deps file single"
		description: "Command requiring a specific file to exist"
		implementations: [
			{
				script: "echo 'README.md exists!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["README.md"]},
			]
		}
	},

	// Example 5.2: Filepath with alternatives
	{
		name:        "deps file alternatives"
		description: "Command requiring any README file"
		implementations: [
			{
				script: "echo 'A README file exists (one of: README.md, README, readme.md)'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["README.md", "README", "readme.md", "README.txt"]},
			]
		}
	},

	// Example 5.3: Filepath with permission checks
	{
		name:        "deps file permissions"
		description: "Command requiring file with specific permissions"
		implementations: [
			{
				script: """
					echo "Permission checks passed!"
					echo "  - Current directory is writable"
					echo "  - README.md is readable"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["."], writable: true},
				{alternatives: ["README.md"], readable: true},
			]
		}
	},

	// ============================================================================
	// SECTION 6: Capability Dependencies
	// ============================================================================
	// These commands demonstrate system capability checking.

	// Example 6.1: Single capability (no alternatives)
	{
		name:        "deps cap single"
		description: "Command requiring internet connectivity"
		implementations: [
			{
				script: "echo 'Internet connectivity confirmed!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["internet"]},
			]
		}
	},

	// Example 6.2: Capability with alternatives
	{
		name:        "deps cap alternatives"
		description: "Command requiring any network connectivity"
		implementations: [
			{
				script: """
					echo "Network connectivity confirmed!"
					echo "Either LAN or Internet is available."
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			capabilities: [
				// Either local network OR internet satisfies this
				{alternatives: ["local-area-network", "internet"]},
			]
		}
	},

	// Example 6.3: Multiple capability requirements
	{
		name:        "deps cap multiple"
		description: "Command requiring multiple capabilities"
		implementations: [
			{
				script: """
					echo "All network capabilities confirmed!"
					echo "  - LAN is available"
					echo "  - Internet is available"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["local-area-network"]},
				{alternatives: ["internet"]},
			]
		}
	},

	// ============================================================================
	// SECTION 7: Custom Check Dependencies
	// ============================================================================
	// These commands demonstrate custom validation scripts.

	// Example 7.1: Single custom check (exit code)
	{
		name:        "deps check exitcode"
		description: "Command with custom check validating exit code"
		implementations: [
			{
				script: "echo 'Custom exit code check passed!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			custom_checks: [
				{
					name:          "true-command"
					check_script:  "true"
					expected_code: 0
				},
			]
		}
	},

	// Example 7.2: Single custom check (output pattern)
	{
		name:        "deps check output"
		description: "Command with custom check validating output pattern"
		implementations: [
			{
				script: "echo 'Custom output check passed!'"
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			custom_checks: [
				{
					name:            "hostname-check"
					check_script:    "hostname"
					expected_code:   0
					expected_output: ".*"  // Any output matches
				},
			]
		}
	},

	// Example 7.3: Custom check with alternatives
	{
		name:        "deps check alternatives"
		description: "Command with alternative custom checks (OR semantics)"
		implementations: [
			{
				script: """
					echo "At least one version check passed!"
					echo "Either bash --version or sh --version returned successfully."
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			custom_checks: [
				// Alternatives: either bash or sh version check passes
				{
					alternatives: [
						{
							name:          "bash-version"
							check_script:  "bash --version"
							expected_code: 0
						},
						{
							name:          "sh-version"
							check_script:  "sh --version 2>&1 || true"
							expected_code: 0
						},
					]
				},
			]
		}
	},

	// ============================================================================
	// SECTION 8: Command Dependencies
	// ============================================================================
	// These commands demonstrate command chaining and dependencies.

	// Example 8.1: Simple command dependency
	{
		name:        "deps cmd simple"
		description: "Command that depends on 'hello' running first"
		implementations: [
			{
				script: "echo 'This runs after examples hello!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			commands: [
				{alternatives: ["examples hello"]},
			]
		}
	},

	// Example 8.2: Command dependency with alternatives
	{
		name:        "deps cmd alternatives"
		description: "Command that depends on any hello command"
		implementations: [
			{
				script: "echo 'This runs after any hello command!'"
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			commands: [
				// Any of these commands satisfies the dependency
				{alternatives: ["examples hello", "examples virtual hello", "examples container hello"]},
			]
		}
	},

	// Example 8.3: Multiple command dependencies
	{
		name:        "deps cmd chain"
		description: "Command with a chain of dependencies"
		implementations: [
			{
				script: """
					echo "=== Dependency Chain Complete ==="
					echo "The following ran in order:"
					echo "  1. examples hello"
					echo "  2. examples hello env"
					echo "  3. This command"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		depends_on: {
			commands: [
				{alternatives: ["examples hello"]},
				{alternatives: ["examples hello env"]},
			]
		}
	},

	// ============================================================================
	// SECTION 9: Implementation-Level Dependencies
	// ============================================================================
	// These commands demonstrate dependencies at the implementation level.

	// Example 9.1: Container with implementation-level tool check
	{
		name:        "impl deps container"
		description: "Container command with implementation-level dependencies"
		implementations: [
			{
				script: """
					echo "=== Implementation-Level Dependencies ==="
					echo "This runs inside a container."
					echo "The 'sh' tool was validated INSIDE the container."
					ls /app-code 2>/dev/null && echo "App code directory mounted successfully."
					"""
				target: {
					runtimes: [{
						name:  "container"
						image: "alpine:latest"
						volumes: [".:/app-code:ro"]
					}]
				}
				// These dependencies are validated INSIDE the container
				depends_on: {
					tools: [
						{alternatives: ["sh"]},
					]
					filepaths: [
						{alternatives: ["/app-code"]},
					]
				}
			}
		]
	},

	// ============================================================================
	// SECTION 10: Complex Combined Examples
	// ============================================================================
	// These commands demonstrate combining multiple features.

	// Example 10.1: Full-featured command with all dependency types
	{
		name:        "full demo"
		description: "Command demonstrating all dependency types"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "     Full Feature Demonstration"
					echo "=========================================="
					echo ""
					echo "All checks passed:"
					echo "  [OK] Tool: sh"
					echo "  [OK] File: README.md (readable)"
					echo "  [OK] Capability: internet"
					echo "  [OK] Custom: hostname check"
					echo "  [OK] Command: examples hello"
					echo ""
					echo "Environment:"
					echo "  DEMO_MODE: $DEMO_MODE"
					echo ""
					echo "=========================================="
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		env: {
			DEMO_MODE: "full"
		}
		depends_on: {
			tools: [
				{alternatives: ["sh"]},
			]
			filepaths: [
				{alternatives: ["README.md"], readable: true},
			]
			capabilities: [
				{alternatives: ["internet"]},
			]
			custom_checks: [
				{
					name:          "hostname-available"
					check_script:  "hostname"
					expected_code: 0
				},
			]
			commands: [
				{alternatives: ["examples hello"]},
			]
		}
	},

	// Example 10.2: Container with full features
	{
		name:        "full container"
		description: "Container command with all container features"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "     Full Container Demonstration"
					echo "=========================================="
					echo ""
					echo "Container Configuration:"
					echo "  Image: alpine:latest"
					echo "  Volume: .:/app-src (read-only)"
					echo "  Ports: 8080:80, 3000:3000"
					echo "  Host SSH: enabled"
					echo ""
					echo "Environment Variables:"
					echo "  APP_NAME: $APP_NAME"
					echo "  APP_VERSION: $APP_VERSION"
					echo ""
					echo "SSH Access to Host:"
					echo "  Host: $INVOWK_SSH_HOST"
					echo "  Port: $INVOWK_SSH_PORT"
					echo "  User: $INVOWK_SSH_USER"
					echo ""
					echo "App Source Contents:"
					ls /app-src 2>/dev/null | head -5
					echo "  ..."
					echo ""
					echo "=========================================="
					"""
				target: {
					runtimes: [{
						name:  "container"
						image: "alpine:latest"
						volumes: [".:/app-src:ro"]
						ports: ["8080:80", "3000:3000"]
						enable_host_ssh: true
					}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
				depends_on: {
					tools: [
						{alternatives: ["sh", "ash"]},
					]
					filepaths: [
						{alternatives: ["/app-src"]},
					]
				}
			}
		]
		env: {
			APP_NAME:    "demo-container-app"
			APP_VERSION: "1.0.0"
		}
	},
]
