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
	// SECTION 10: Command Flags
	// ============================================================================
	// These commands demonstrate the use of command-line flags.

	// Example 10.1: Simple command with flags
	{
		name:        "flags simple"
		description: "Command with simple flags"
		implementations: [
			{
				script: """
					echo "=== Simple Flags Demo ==="
					echo ""
					echo "Flags passed to this command are available as environment variables:"
					echo "  INVOWK_FLAG_VERBOSE = '${INVOWK_FLAG_VERBOSE}'"
					echo "  INVOWK_FLAG_OUTPUT = '${INVOWK_FLAG_OUTPUT}'"
					echo ""
					if [ -n "$INVOWK_FLAG_VERBOSE" ] && [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
					    echo "[VERBOSE] Extra verbose information would appear here"
					fi
					if [ -n "$INVOWK_FLAG_OUTPUT" ]; then
					    echo "[OUTPUT] Would write to: $INVOWK_FLAG_OUTPUT"
					fi
					echo ""
					echo "Try: invowk cmd examples flags simple --verbose=true --output=/tmp/out.txt"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output"},
			{name: "output", description: "Output file path"},
		]
	},

	// Example 10.2: Flags with default values
	{
		name:        "flags defaults"
		description: "Command with flags that have default values"
		implementations: [
			{
				script: """
					echo "=== Flags with Defaults Demo ==="
					echo ""
					echo "These flags have default values if not specified:"
					echo "  INVOWK_FLAG_ENV = '${INVOWK_FLAG_ENV}' (default: development)"
					echo "  INVOWK_FLAG_RETRY_COUNT = '${INVOWK_FLAG_RETRY_COUNT}' (default: 3)"
					echo "  INVOWK_FLAG_DRY_RUN = '${INVOWK_FLAG_DRY_RUN}' (default: false)"
					echo ""
					if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
					    echo "[DRY-RUN MODE] Would deploy to '$INVOWK_FLAG_ENV' with $INVOWK_FLAG_RETRY_COUNT retries"
					else
					    echo "Deploying to '$INVOWK_FLAG_ENV' with $INVOWK_FLAG_RETRY_COUNT retries..."
					fi
					echo ""
					echo "Try: invowk cmd examples flags defaults --env=production --dry-run=true"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "env", description: "Target environment", default_value: "development"},
			{name: "retry-count", description: "Number of retries", default_value: "3"},
			{name: "dry-run", description: "Perform a dry run without making changes", default_value: "false"},
		]
	},

	// Example 10.3: Typed flags (bool, int, float, string)
	{
		name:        "flags typed"
		description: "Command with typed flags (bool, int, float, string)"
		implementations: [
			{
				script: """
					echo "=== Typed Flags Demo ==="
					echo ""
					echo "These flags have explicit types:"
					echo "  INVOWK_FLAG_VERBOSE (bool) = '$INVOWK_FLAG_VERBOSE'"
					echo "  INVOWK_FLAG_COUNT (int) = '$INVOWK_FLAG_COUNT'"
					echo "  INVOWK_FLAG_THRESHOLD (float) = '$INVOWK_FLAG_THRESHOLD'"
					echo "  INVOWK_FLAG_MESSAGE (string) = '$INVOWK_FLAG_MESSAGE'"
					echo ""
					echo "Typed flags are validated:"
					echo "  - bool: only 'true' or 'false' accepted"
					echo "  - int: only valid integers accepted"
					echo "  - float: only valid floating-point numbers accepted"
					echo "  - string: any value accepted (default)"
					echo ""
					echo "Try: invowk cmd examples flags typed --verbose --count=5 --threshold=0.95 --message='Hello World'"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"},
			{name: "count", description: "Number of iterations", type: "int", default_value: "1"},
			{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.75"},
			{name: "message", description: "Message to display", type: "string", default_value: "Hello"},
		]
	},

	// Example 10.4: Required flags
	{
		name:        "flags required"
		description: "Command with required flags that must be provided"
		implementations: [
			{
				script: """
					echo "=== Required Flags Demo ==="
					echo ""
					echo "This command requires the --target flag to be provided."
					echo "  INVOWK_FLAG_TARGET = '$INVOWK_FLAG_TARGET'"
					echo ""
					echo "Required flags cannot have default values."
					echo ""
					echo "Try: invowk cmd examples flags required --target=production"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "target", description: "Target to deploy to (required)", required: true},
			{name: "confirm", description: "Skip confirmation prompt", type: "bool", default_value: "false"},
		]
	},

	// Example 10.5: Short aliases for flags
	{
		name:        "flags short"
		description: "Command with short flag aliases"
		implementations: [
			{
				script: """
					echo "=== Short Flag Aliases Demo ==="
					echo ""
					echo "These flags have short aliases:"
					echo "  -v / --verbose = '$INVOWK_FLAG_VERBOSE'"
					echo "  -o / --output = '$INVOWK_FLAG_OUTPUT'"
					echo "  -f / --force = '$INVOWK_FLAG_FORCE'"
					echo ""
					echo "Short aliases make command-line usage more convenient."
					echo ""
					echo "Try: invowk cmd examples flags short -v -o=/tmp/out.txt -f"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", short: "v", default_value: "false"},
			{name: "output", description: "Output file path", short: "o"},
			{name: "force", description: "Force overwrite", type: "bool", short: "f", default_value: "false"},
		]
	},

	// Example 10.6: Flags with validation regex
	{
		name:        "flags validation"
		description: "Command with flags validated by regex patterns"
		implementations: [
			{
				script: """
					echo "=== Flag Validation Demo ==="
					echo ""
					echo "These flags are validated against regex patterns:"
					echo "  --env (dev|staging|prod) = '$INVOWK_FLAG_ENV'"
					echo "  --version (semver) = '$INVOWK_FLAG_VERSION'"
					echo ""
					echo "Invalid values will be rejected before the command runs."
					echo ""
					echo "Try: invowk cmd examples flags validation --env=staging --version=1.2.3"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{name: "env", description: "Environment (dev, staging, or prod)", validation: "^(dev|staging|prod)$", default_value: "dev"},
			{name: "version", description: "Version number (semver format)", validation: #"^[0-9]+\.[0-9]+\.[0-9]+$"#, default_value: "1.0.0"},
		]
	},

	// Example 10.7: Full-featured flags combining all options
	{
		name:        "flags full"
		description: "Command demonstrating all flag features combined"
		implementations: [
			{
				script: """
					echo "=== Full Flags Feature Demo ==="
					echo ""
					echo "This command demonstrates all flag features:"
					echo ""
					echo "  Required flag:"
					echo "    --env / -e = '$INVOWK_FLAG_ENV' (validated: dev|staging|prod)"
					echo ""
					echo "  Typed flags with defaults:"
					echo "    --replicas / -n (int) = '$INVOWK_FLAG_REPLICAS'"
					echo "    --dry-run / -d (bool) = '$INVOWK_FLAG_DRY_RUN'"
					echo "    --tag / -t (string) = '$INVOWK_FLAG_TAG' (validated: semver)"
					echo ""
					echo "Try: invowk cmd examples flags full -e=prod -n=3 -d -t=2.0.0"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		flags: [
			{
				name:        "env"
				description: "Target environment"
				type:        "string"
				required:    true
				short:       "e"
				validation:  "^(dev|staging|prod)$"
			},
			{
				name:          "replicas"
				description:   "Number of replicas to deploy"
				type:          "int"
				short:         "n"
				default_value: "1"
			},
			{
				name:          "dry-run"
				description:   "Perform a dry run without making changes"
				type:          "bool"
				short:         "d"
				default_value: "false"
			},
			{
				name:          "tag"
				description:   "Version tag (semver format)"
				type:          "string"
				short:         "t"
				default_value: "1.0.0"
				validation:    #"^[0-9]+\.[0-9]+\.[0-9]+$"#
			},
		]
	},

	// ============================================================================
	// SECTION 11: Complex Combined Examples
	// ============================================================================
	// These commands demonstrate combining multiple features.

	// Example 11.1: Full-featured command with all dependency types
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

	// Example 11.2: Container with full features and command flags
	{
		name:        "full container"
		description: "Container command with all container features"
		flags: [
			{name: "env", description: "Target environment (e.g., staging, production)"},
			{name: "dry-run", description: "Perform a dry run without making changes", default_value: "false"},
			{name: "verbose", description: "Enable verbose output"},
		]
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

	// ============================================================================
	// SECTION 12: Positional Arguments
	// ============================================================================
	// These commands demonstrate positional command-line arguments.

	// Example 12.1: Simple command with required argument
	{
		name:        "args simple"
		description: "Command with a single required argument"
		implementations: [
			{
				script: """
					echo "=== Simple Positional Argument Demo ==="
					echo ""
					echo "You provided the following argument:"
					echo "  INVOWK_ARG_NAME = '$INVOWK_ARG_NAME'"
					echo ""
					echo "Hello, $INVOWK_ARG_NAME!"
					echo ""
					echo "Try: invowk cmd examples args simple Alice"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
		]
	},

	// Example 12.2: Command with required and optional arguments
	{
		name:        "args optional"
		description: "Command with required and optional arguments"
		implementations: [
			{
				script: """
					echo "=== Required + Optional Arguments Demo ==="
					echo ""
					echo "Arguments received:"
					echo "  INVOWK_ARG_NAME = '$INVOWK_ARG_NAME' (required)"
					echo "  INVOWK_ARG_GREETING = '$INVOWK_ARG_GREETING' (optional, default: Hello)"
					echo ""
					echo "$INVOWK_ARG_GREETING, $INVOWK_ARG_NAME!"
					echo ""
					echo "Try: invowk cmd examples args optional Alice"
					echo "Try: invowk cmd examples args optional Alice 'Good morning'"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
			{name: "greeting", description: "The greeting to use", default_value: "Hello"},
		]
	},

	// Example 12.3: Command with typed arguments (int, float)
	{
		name:        "args typed"
		description: "Command with typed positional arguments"
		implementations: [
			{
				script: """
					echo "=== Typed Arguments Demo ==="
					echo ""
					echo "Arguments received:"
					echo "  INVOWK_ARG_WIDTH (int) = '$INVOWK_ARG_WIDTH'"
					echo "  INVOWK_ARG_HEIGHT (int) = '$INVOWK_ARG_HEIGHT'"
					echo "  INVOWK_ARG_SCALE (float) = '$INVOWK_ARG_SCALE'"
					echo ""
					echo "Typed arguments are validated at runtime:"
					echo "  - int: only valid integers accepted"
					echo "  - float: only valid floating-point numbers accepted"
					echo ""
					echo "Try: invowk cmd examples args typed 800 600 1.5"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		args: [
			{name: "width", description: "Width in pixels", required: true, type: "int"},
			{name: "height", description: "Height in pixels", required: true, type: "int"},
			{name: "scale", description: "Scale factor", type: "float", default_value: "1.0"},
		]
	},

	// Example 12.4: Command with validated arguments (regex)
	{
		name:        "args validated"
		description: "Command with regex-validated arguments"
		implementations: [
			{
				script: """
					echo "=== Validated Arguments Demo ==="
					echo ""
					echo "Arguments received:"
					echo "  INVOWK_ARG_ENV = '$INVOWK_ARG_ENV' (must be dev|staging|prod)"
					echo "  INVOWK_ARG_VERSION = '$INVOWK_ARG_VERSION' (must be semver format)"
					echo ""
					echo "Deploying version $INVOWK_ARG_VERSION to $INVOWK_ARG_ENV..."
					echo ""
					echo "Try: invowk cmd examples args validated staging 2.1.0"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		args: [
			{name: "env", description: "Target environment (dev, staging, or prod)", required: true, validation: "^(dev|staging|prod)$"},
			{name: "version", description: "Version to deploy (semver format)", required: true, validation: #"^[0-9]+\.[0-9]+\.[0-9]+$"#},
		]
	},

	// Example 12.5: Command with variadic arguments
	{
		name:        "args variadic"
		description: "Command with variadic arguments (accepts multiple values)"
		implementations: [
			{
				script: #"""
					echo "=== Variadic Arguments Demo ==="
					echo ""
					echo "Arguments received:"
					echo "  INVOWK_ARG_DESTINATION = '$INVOWK_ARG_DESTINATION'"
					echo "  INVOWK_ARG_FILES = '$INVOWK_ARG_FILES' (space-joined)"
					echo "  INVOWK_ARG_FILES_COUNT = '$INVOWK_ARG_FILES_COUNT'"
					echo ""
					echo "Individual file arguments:"
					i=1
					while [ $i -le ${INVOWK_ARG_FILES_COUNT:-0} ]; do
					    eval "file=\$INVOWK_ARG_FILES_$i"
					    echo "  INVOWK_ARG_FILES_$i = '$file'"
					    i=$((i + 1))
					done
					echo ""
					echo "Variadic arguments collect all remaining positional values."
					echo "Only the last argument can be variadic."
					echo ""
					echo "Try: invowk cmd examples args variadic /tmp file1.txt file2.txt file3.txt"
					"""#
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		args: [
			{name: "destination", description: "Destination directory", required: true},
			{name: "files", description: "Source files to copy", required: true, variadic: true},
		]
	},

	// Example 12.6: Command with all argument features combined
	{
		name:        "args full"
		description: "Command demonstrating all argument features"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "     Full Arguments Feature Demo"
					echo "=========================================="
					echo ""
					echo "  Required string arg (validated):"
					echo "    INVOWK_ARG_ENV = '$INVOWK_ARG_ENV'"
					echo ""
					echo "  Optional typed args with defaults:"
					echo "    INVOWK_ARG_REPLICAS (int) = '$INVOWK_ARG_REPLICAS'"
					echo "    INVOWK_ARG_TIMEOUT (float) = '$INVOWK_ARG_TIMEOUT'"
					echo ""
					echo "  Variadic arg (collects remaining args):"
					echo "    INVOWK_ARG_SERVICES = '$INVOWK_ARG_SERVICES'"
					echo "    INVOWK_ARG_SERVICES_COUNT = '$INVOWK_ARG_SERVICES_COUNT'"
					echo ""
					echo "Try: invowk cmd examples args full prod 3 30.0 api web worker"
					echo "=========================================="
					"""
				target: {
					runtimes:  [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		args: [
			{
				name:        "env"
				description: "Target environment"
				required:    true
				validation:  "^(dev|staging|prod)$"
			},
			{
				name:          "replicas"
				description:   "Number of replicas"
				type:          "int"
				default_value: "1"
			},
			{
				name:          "timeout"
				description:   "Request timeout in seconds"
				type:          "float"
				default_value: "30.0"
			},
			{
				name:        "services"
				description: "Services to deploy"
				variadic:    true
			},
		]
	},

	// Example 12.7: Command with both flags and arguments
	{
		name:        "args with flags"
		description: "Command combining positional arguments and flags"
		implementations: [
			{
				script: """
					echo "=== Arguments + Flags Combined Demo ==="
					echo ""
					echo "Positional Arguments:"
					echo "  INVOWK_ARG_SOURCE = '$INVOWK_ARG_SOURCE'"
					echo "  INVOWK_ARG_DESTINATION = '$INVOWK_ARG_DESTINATION'"
					echo ""
					echo "Flags:"
					echo "  INVOWK_FLAG_VERBOSE = '$INVOWK_FLAG_VERBOSE'"
					echo "  INVOWK_FLAG_FORCE = '$INVOWK_FLAG_FORCE'"
					echo "  INVOWK_FLAG_BACKUP = '$INVOWK_FLAG_BACKUP'"
					echo ""
					echo "Both positional args and flags can be used together."
					echo "Flags are prefixed with INVOWK_FLAG_, args with INVOWK_ARG_."
					echo ""
					echo "Try: invowk cmd examples args with flags file.txt /tmp --verbose --backup"
					"""
				target: {
					runtimes: [{name: "native"}]
				}
			}
		]
		args: [
			{name: "source", description: "Source file", required: true},
			{name: "destination", description: "Destination directory", required: true},
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", short: "v", default_value: "false"},
			{name: "force", description: "Force overwrite", type: "bool", short: "f", default_value: "false"},
			{name: "backup", description: "Create backup before overwriting", type: "bool", short: "b", default_value: "false"},
		]
	},
]
