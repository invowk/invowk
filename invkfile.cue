// Invkfile - Example command definitions for invowk
// See https://github.com/invowk/invowk for documentation
//
// This file contains example commands that demonstrate all invowk features.
// All commands are idempotent and do not cause side effects on the host.
//
// Note: Module metadata (module, version, description, requires) belongs in invkmod.cue.
// This file only contains command definitions (cmds) and shared configuration.

// Global environment configuration - applied to all commands
// Root-level env has the lowest priority from invkfile configuration.
// Command-level and implementation-level env can override these values.
env: {
	vars: {
		APP_ENV:     "development"
		LOG_LEVEL:   "info"
	}
}

// Global dependency checks - validated before ANY command runs
// Root-level depends_on provides shared prerequisites for all commands.
// These checks run first, followed by command-level, then implementation-level.
// Merge order: Root (lowest priority) -> Command -> Implementation (highest priority)
depends_on: {
	// Ensure basic shell is available for all commands
	tools: [
		{alternatives: ["sh", "bash"]},
	]
}

cmds: [
	// ============================================================================
	// SECTION 1: Simple Commands (Native Runtime)
	// ============================================================================
	// These commands demonstrate basic native shell execution with minimal config.

	// Example 1.1: Simplest possible command - native runtime with virtual fallback
	{
		name:        "hello"
		description: "Print a simple greeting (native runtime)"
		implementations: [
			{
				script: "echo 'Hello from invowk!'"
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
	},

	// Example 1.5: Environment variable hierarchy demonstration
	// This command shows how env vars are merged across three levels:
	// Root-level (lowest priority) -> Command-level -> Implementation-level (highest priority from invkfile)
	// Note: --env-file and --env-var CLI flags have even higher priority.
	{
		name:        "env hierarchy"
		description: "Demonstrate environment variable precedence across levels"
		implementations: [
			{
				script: """
					echo "=== Environment Variable Hierarchy ==="
					echo "APP_ENV: $APP_ENV (from root-level, not overridden)"
					echo "LOG_LEVEL: $LOG_LEVEL (from command-level, overrides root)"
					echo "RUNTIME: $RUNTIME (from implementation-level, overrides command)"
					"""
				runtimes: [{name: "native"}, {name: "virtual"}]
				env: {
					vars: {
						RUNTIME: "native"
					}
				}
			}
		]
		env: {
			vars: {
				LOG_LEVEL: "debug"
				RUNTIME:   "unknown"
			}
		}
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
				runtimes: [{name: "virtual"}]
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
	},

	// ============================================================================
	// SECTION 3: Interpreter Support
	// ============================================================================
	// These commands demonstrate the interpreter field for running scripts
	// with non-shell interpreters (Python, Ruby, Node.js, etc.).
	// The interpreter can be auto-detected from shebang or explicitly specified.

	// Example 3.1: Python script with shebang (auto-detection)
	{
		name:        "interpreter python shebang"
		description: "Python script with automatic shebang detection"
		implementations: [
			{
				script: """
					#!/usr/bin/env python3
					import sys
					print(f"Hello from Python {sys.version_info.major}.{sys.version_info.minor}!")
					print("Interpreter was auto-detected from the shebang line.")
					"""
				// Default interpreter is "auto" - shebang is parsed automatically
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["python3"]}]
		}
	},

	// Example 3.2: Python script with explicit interpreter
	{
		name:        "interpreter python explicit"
		description: "Python script with explicitly specified interpreter"
		implementations: [
			{
				script: """
					import sys
					print(f"Hello from Python {sys.version_info.major}.{sys.version_info.minor}!")
					print("Interpreter was explicitly set to 'python3'.")
					print("No shebang needed when interpreter is explicit.")
					"""
				// Explicit interpreter - no shebang needed
				runtimes:  [{name: "native", interpreter: "python3"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["python3"]}]
		}
	},

	// Example 3.3: Python with interpreter arguments (-u for unbuffered output)
	{
		name:        "interpreter python args"
		description: "Python script with interpreter arguments in shebang"
		implementations: [
			{
				script: """
					#!/usr/bin/env -S python3 -u
					import sys
					print("This output is unbuffered (-u flag)")
					print(f"Arguments: {sys.argv[1:]}", file=sys.stderr)
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["python3"]}]
		}
	},

	// Example 3.4: Python script file reference
	{
		name:        "interpreter python file"
		description: "Python script loaded from external file"
		implementations: [
			{
				// When script is a file path, shebang is read from the file
				script: "./scripts/example.py"
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools:     [{alternatives: ["python3"]}]
			filepaths: [{alternatives: ["./scripts/example.py"]}]
		}
	},

	// Example 3.5: Node.js script with shebang
	{
		name:        "interpreter node"
		description: "Node.js script with shebang detection"
		implementations: [
			{
				script: """
					#!/usr/bin/env node
					console.log("Hello from Node.js!");
					console.log(`Node version: ${process.version}`);
					console.log(`Arguments: ${process.argv.slice(2).join(', ')}`);
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["node"]}]
		}
	},

	// Example 3.6: Ruby script with explicit interpreter
	{
		name:        "interpreter ruby"
		description: "Ruby script with explicit interpreter"
		implementations: [
			{
				script: """
					puts "Hello from Ruby!"
					puts "Ruby version: #{RUBY_VERSION}"
					puts "Arguments: #{ARGV.join(', ')}"
					"""
				runtimes:  [{name: "native", interpreter: "ruby"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["ruby"]}]
		}
	},

	// Example 3.7: Perl script with shebang and arguments
	{
		name:        "interpreter perl"
		description: "Perl script with shebang including -w flag"
		implementations: [
			{
				script: """
					#!/usr/bin/perl -w
					use strict;
					print "Hello from Perl!\\n";
					print "Perl version: $]\\n";
					print "Arguments: @ARGV\\n";
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [{alternatives: ["perl"]}]
		}
	},

	// Example 3.8: Interpreter with positional arguments
	{
		name:        "interpreter python positional"
		description: "Python script receiving positional arguments"
		implementations: [
			{
				script: """
					#!/usr/bin/env python3
					import sys
					
					name = sys.argv[1] if len(sys.argv) > 1 else "World"
					count = int(sys.argv[2]) if len(sys.argv) > 2 else 1
					
					for i in range(count):
					    print(f"Hello, {name}! (iteration {i+1})")
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		args: [
			{name: "name", description: "Name to greet", default_value: "World"},
			{name: "count", description: "Number of greetings", type: "int", default_value: "3"},
		]
		depends_on: {
			tools: [{alternatives: ["python3"]}]
		}
	},

	// Example 3.9: Container with interpreter
	{
		name:        "interpreter container python"
		description: "Python script running inside a container"
		implementations: [
			{
				script: """
					#!/usr/bin/env python3
					import os
					import sys
					
					print("Hello from Python inside a container!")
					print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")
					print(f"Working directory: {os.getcwd()}")
					print(f"Arguments: {sys.argv[1:]}")
					"""
				runtimes: [{
					name:  "container"
					image: "python:3-slim"
					// interpreter auto-detected from shebang
				}]
			}
		]
	},

	// Example 3.10: Container with explicit interpreter (no shebang)
	{
		name:        "interpreter container explicit"
		description: "Container script with explicit interpreter specified"
		implementations: [
			{
				script: """
					import os
					import sys
					
					print("Script without shebang, interpreter explicitly set")
					print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")
					print(f"Container hostname: {os.uname().nodename}")
					"""
				runtimes: [{
					name:        "container"
					image:       "python:3-slim"
					interpreter: "python3"
				}]
			}
		]
	},

	// Example 3.11: Fallback to shell when no interpreter/shebang
	{
		name:        "interpreter fallback shell"
		description: "Script without shebang falls back to shell execution"
		implementations: [
			{
				script: """
					echo "No shebang and no explicit interpreter"
					echo "This script runs via the default shell (/bin/sh)"
					echo "Working directory: $(pwd)"
					echo "User: $(whoami)"
					"""
				// No interpreter specified, no shebang in script
				// -> falls back to shell execution (default behavior)
				runtimes: [{name: "native"}]
			}
		]
	},

	// ============================================================================
	// SECTION 4: Container Runtime Commands
	// ============================================================================
	// These commands demonstrate container-based execution with Docker/Podman.

	// Example 4.1: Simple container command
	{
		name:        "container hello"
		description: "Print greeting from inside a container"
		implementations: [
			{
				script: "echo 'Hello from inside the container!'"
				runtimes: [{name: "container", image: "debian:stable-slim"}]
			}
		]
	},

	// Example 4.2: Container with volume mounts
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
				runtimes: [{
					name:  "container"
					image: "debian:stable-slim"
					volumes: [
						".:/app-data:ro",
					]
				}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
	},

	// Example 4.3: Container with port mappings
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
				runtimes: [{
					name:  "container"
					image: "debian:stable-slim"
					ports: ["8080:80", "3000:3000"]
				}]
			}
		]
	},

	// Example 4.4: Container with environment variables
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
					echo "TERM (host allowlist): ${TERM:-<unset>}"
					echo "LANG (host allowlist): ${LANG:-<unset>}"
					"""
				runtimes: [{
					name:              "container"
					image:             "debian:stable-slim"
					env_inherit_mode:  "allow"
					env_inherit_allow: ["TERM", "LANG"]
				}]
			}
		]
		env: {
			vars: {
				APP_NAME: "demo-app"
				APP_ENV:  "development"
				DEBUG:    "true"
			}
		}
	},

	// Example 4.5: Container with host SSH access enabled
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
				runtimes: [{
					name:            "container"
					image:           "debian:stable-slim"
					enable_host_ssh: true
				}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
	},

	// Example 4.6: Container without host SSH (explicit false)
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
				runtimes: [{
					name:            "container"
					image:           "debian:stable-slim"
					enable_host_ssh: false
				}]
			}
		]
	},

	// ============================================================================
	// SECTION 5: Tool Dependencies
	// ============================================================================
	// These commands demonstrate tool/binary dependency checking.

	// Example 5.1: Single tool dependency (no alternatives)
	{
		name:        "deps tool single"
		description: "Command requiring a single tool (sh)"
		implementations: [
			{
				script: "echo 'sh is available!'"
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["sh"]},
			]
		}
	},

	// Example 5.2: Tool dependency with alternatives (OR semantics)
	{
		name:        "deps tool alternatives"
		description: "Command requiring any of: podman, docker, or nerdctl"
		implementations: [
			{
				script: """
					echo "Container runtime check passed!"
					echo "At least one of podman/docker/nerdctl is available."
					"""
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		depends_on: {
			tools: [
				// Any one of these satisfies the dependency
				{alternatives: ["podman", "docker", "nerdctl"]},
			]
		}
	},

	// Example 5.3: Multiple tool dependencies with mixed alternatives
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
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
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
	// SECTION 6: Filepath Dependencies
	// ============================================================================
	// These commands demonstrate file/directory dependency checking.

	// Example 6.1: Single filepath dependency
	{
		name:        "deps file single"
		description: "Command requiring a specific file to exist"
		implementations: [
			{
				script: "echo 'README.md exists!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["README.md"]},
			]
		}
	},

	// Example 6.2: Filepath with alternatives
	{
		name:        "deps file alternatives"
		description: "Command requiring any README file"
		implementations: [
			{
				script: "echo 'A README file exists (one of: README.md, README, readme.md)'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["README.md", "README", "readme.md", "README.txt"]},
			]
		}
	},

	// Example 6.3: Filepath with permission checks
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
				runtimes: [{name: "native"}]
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
	// SECTION 7: Capability Dependencies
	// ============================================================================
	// These commands demonstrate system capability checking.

	// Example 7.1: Single capability (no alternatives)
	{
		name:        "deps cap single"
		description: "Command requiring internet connectivity"
		implementations: [
			{
				script: "echo 'Internet connectivity confirmed!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["internet"]},
			]
		}
	},

	// Example 7.2: Capability with alternatives
	{
		name:        "deps cap alternatives"
		description: "Command requiring any network connectivity"
		implementations: [
			{
				script: """
					echo "Network connectivity confirmed!"
					echo "Either LAN or Internet is available."
					"""
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			capabilities: [
				// Either local network OR internet satisfies this
				{alternatives: ["local-area-network", "internet"]},
			]
		}
	},

	// Example 7.3: Multiple capability requirements
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
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["local-area-network"]},
				{alternatives: ["internet"]},
			]
		}
	},

	// Example 7.4: Container engine availability
	{
		name:        "deps cap containers"
		description: "Command requiring container engine availability"
		implementations: [
			{
				script: "echo 'Container engine is available!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["containers"]},
			]
		}
	},

	// Example 7.5: Interactive TTY requirement
	{
		name:        "deps cap tty"
		description: "Command requiring interactive TTY"
		implementations: [
			{
				script: "echo 'Interactive TTY detected!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["tty"]},
			]
		}
	},

	// ============================================================================
	// SECTION 8: Custom Check Dependencies
	// ============================================================================
	// These commands demonstrate custom validation scripts.

	// Example 8.1: Single custom check (exit code)
	{
		name:        "deps check exitcode"
		description: "Command with custom check validating exit code"
		implementations: [
			{
				script: "echo 'Custom exit code check passed!'"
				runtimes: [{name: "native"}, {name: "virtual"}]
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

	// Example 8.2: Single custom check (output pattern)
	{
		name:        "deps check output"
		description: "Command with custom check validating output pattern"
		implementations: [
			{
				script: "echo 'Custom output check passed!'"
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
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

	// Example 8.3: Custom check with alternatives
	{
		name:        "deps check alternatives"
		description: "Command with alternative custom checks (OR semantics)"
		implementations: [
			{
				script: """
					echo "At least one version check passed!"
					echo "Either bash --version or sh --version returned successfully."
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
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
	// SECTION 9: Command Dependencies
	// ============================================================================
	// These commands demonstrate command chaining and dependencies.

	// Example 9.1: Simple command dependency
	{
		name:        "deps cmd simple"
		description: "Command that depends on 'hello' running first"
		implementations: [
			{
				script: "echo 'This runs after examples hello!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			cmds: [
				{alternatives: ["examples hello"]},
			]
		}
	},

	// Example 9.2: Command dependency with alternatives
	{
		name:        "deps cmd alternatives"
		description: "Command that depends on any hello command"
		implementations: [
			{
				script: "echo 'This runs after any hello command!'"
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			cmds: [
				// Any of these commands satisfies the dependency
				{alternatives: ["examples hello", "examples virtual hello", "examples container hello"]},
			]
		}
	},

	// Example 9.3: Multiple command dependencies
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
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			cmds: [
				{alternatives: ["examples hello"]},
				{alternatives: ["examples hello env"]},
			]
		}
	},

	// ============================================================================
	// SECTION 10: Implementation-Level Dependencies
	// ============================================================================
	// These commands demonstrate dependencies at the implementation level.

	// Example 10.1: Container with implementation-level tool check
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
				runtimes: [{
					name:  "container"
					image: "debian:stable-slim"
					volumes: [".:/app-code:ro"]
				}]
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
	// SECTION 11: Command Flags
	// ============================================================================
	// These commands demonstrate the use of command-line flags.

	// Example 11.1: Simple command with flags
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output"},
			{name: "output", description: "Output file path"},
		]
	},

	// Example 11.2: Flags with default values
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		flags: [
			{name: "env", description: "Target environment", default_value: "development"},
			{name: "retry-count", description: "Number of retries", default_value: "3"},
			{name: "dry-run", description: "Perform a dry run without making changes", default_value: "false"},
		]
	},

	// Example 11.3: Typed flags (bool, int, float, string)
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"},
			{name: "count", description: "Number of iterations", type: "int", default_value: "1"},
			{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.75"},
			{name: "message", description: "Message to display", type: "string", default_value: "Hello"},
		]
	},

	// Example 11.4: Required flags
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
				runtimes: [{name: "native"}]
			}
		]
		flags: [
			{name: "target", description: "Target to deploy to (required)", required: true},
			{name: "confirm", description: "Skip confirmation prompt", type: "bool", default_value: "false"},
		]
	},

	// Example 11.5: Short aliases for flags
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", short: "v", default_value: "false"},
			{name: "output", description: "Output file path", short: "o"},
			{name: "force", description: "Force overwrite", type: "bool", short: "f", default_value: "false"},
		]
	},

	// Example 11.6: Flags with validation regex
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		flags: [
			{name: "env", description: "Environment (dev, staging, or prod)", validation: "^(dev|staging|prod)$", default_value: "dev"},
			{name: "version", description: "Version number (semver format)", validation: #"^[0-9]+\.[0-9]+\.[0-9]+$"#, default_value: "1.0.0"},
		]
	},

	// Example 11.7: Full-featured flags combining all options
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
					echo "    --target-env / -x = '$INVOWK_FLAG_TARGET_ENV' (validated: dev|staging|prod)"
					echo ""
					echo "  Typed flags with defaults:"
					echo "    --replicas / -n (int) = '$INVOWK_FLAG_REPLICAS'"
					echo "    --dry-run / -d (bool) = '$INVOWK_FLAG_DRY_RUN'"
					echo "    --tag / -t (string) = '$INVOWK_FLAG_TAG' (validated: semver)"
					echo ""
					echo "Try: invowk cmd examples flags full -x=prod -n=3 -d -t=2.0.0"
					"""
				runtimes: [{name: "native"}]
			}
		]
		flags: [
			{
				name:        "target-env"
				description: "Target environment"
				type:        "string"
				required:    true
				short:       "x"
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
	// SECTION 12: Complex Combined Examples
	// ============================================================================
	// These commands demonstrate combining multiple features.

	// Example 12.1: Full-featured command with all dependency types
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
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		env: {
			vars: {
				DEMO_MODE: "full"
			}
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
			cmds: [
				{alternatives: ["examples hello"]},
			]
		}
	},

	// Example 12.2: Container with full features and command flags
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
					echo "  Image: debian:stable-slim"
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
				runtimes: [{
					name:  "container"
					image: "debian:stable-slim"
					volumes: [".:/app-src:ro"]
					ports: ["8080:80", "3000:3000"]
					enable_host_ssh: true
				}]
				platforms: [{name: "linux"}, {name: "macos"}]
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
			vars: {
				APP_NAME:    "demo-container-app"
				APP_VERSION: "1.0.0"
			}
		}
	},

	// ============================================================================
	// SECTION 13: Positional Arguments
	// ============================================================================
	// These commands demonstrate positional command-line arguments.

	// Example 13.1: Simple command with required argument
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
		]
	},

	// Example 13.2: Command with required and optional arguments
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
			{name: "greeting", description: "The greeting to use", default_value: "Hello"},
		]
	},

	// Example 13.3: Command with typed arguments (int, float)
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		args: [
			{name: "width", description: "Width in pixels", required: true, type: "int"},
			{name: "height", description: "Height in pixels", required: true, type: "int"},
			{name: "scale", description: "Scale factor", type: "float", default_value: "1.0"},
		]
	},

	// Example 13.4: Command with validated arguments (regex)
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
				runtimes: [{name: "native"}, {name: "virtual"}]
			}
		]
		args: [
			{name: "env", description: "Target environment (dev, staging, or prod)", required: true, validation: "^(dev|staging|prod)$"},
			{name: "version", description: "Version to deploy (semver format)", required: true, validation: #"^[0-9]+\.[0-9]+\.[0-9]+$"#},
		]
	},

	// Example 13.5: Command with variadic arguments
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
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		args: [
			{name: "destination", description: "Destination directory", required: true},
			{name: "files", description: "Source files to copy", required: true, variadic: true},
		]
	},

	// Example 13.6: Command with all argument features combined
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
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
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

	// Example 13.7: Command with both flags and arguments
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
				runtimes: [{name: "native"}]
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

	// Example 13.8: Shell-native positional parameters ($1, $2, $@, $#)
	{
		name:        "args shell positional"
		description: "Access arguments via shell positional parameters ($1, $2, etc.)"
		implementations: [
			{
				script: #"""
					echo "=== Shell Positional Parameters Demo ==="
					echo ""
					echo "Arguments can be accessed using traditional shell syntax:"
					echo "  \$1 (first arg)  = '$1'"
					echo "  \$2 (second arg) = '$2'"
					echo "  \$@ (all args)   = '$@'"
					echo "  \$# (arg count)  = '$#'"
					echo ""
					echo "This is the traditional POSIX shell way to access arguments."
					echo "It works in native, virtual, and container runtimes."
					echo ""
					echo "Try: invowk cmd examples args shell positional hello world"
					"""#
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "first", description: "First argument"},
			{name: "second", description: "Second argument"},
		]
	},

	// Example 13.9: Positional parameters with variadic arguments
	{
		name:        "args shell variadic"
		description: "Process variadic arguments using shell positional parameters"
		implementations: [
			{
				script: #"""
					echo "=== Variadic Args via Positional Parameters ==="
					echo ""
					echo "Processing $# file(s)..."
					echo ""
					i=1
					for file in "$@"; do
					    echo "  [$i] $file"
					    i=$((i + 1))
					done
					echo ""
					echo "Using \$@ in a for loop is the idiomatic way to process"
					echo "all arguments in shell scripts."
					echo ""
					echo "Try: invowk cmd examples args shell variadic file1.txt file2.txt file3.txt"
					"""#
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		args: [
			{name: "files", description: "Files to process", variadic: true},
		]
	},

	// Example 13.10: Container with positional parameters
	{
		name:        "args container shell"
		description: "Access positional parameters inside a container"
		implementations: [
			{
				script: #"""
					echo "=== Container Positional Parameters Demo ==="
					echo ""
					echo "Arguments passed to container script:"
					echo "  \$1 = '$1'"
					echo "  \$2 = '$2'"
					echo "  \$# = '$#'"
					echo "  \$@ = '$@'"
					echo ""
					echo "Positional parameters work the same way inside containers"
					echo "as they do in native shell execution."
					echo ""
					echo "Try: invowk cmd examples args container shell hello container"
					"""#
				runtimes: [{name: "container", image: "debian:stable-slim"}]
			}
		]
		args: [
			{name: "arg1", description: "First argument"},
			{name: "arg2", description: "Second argument"},
		]
	},

	// ============================================================================
	// SECTION 14: Environment Variable Isolation
	// ============================================================================
	// These commands demonstrate that INVOWK_ARG_* and INVOWK_FLAG_* environment
	// variables are isolated between nested command invocations. When a command
	// calls another invowk command, the child does not inherit the parent's
	// flag/arg environment variables, preventing unexpected behavior.

	// Example 14.1: Child command for isolation demo (displays its own env vars)
	{
		name:        "isolation child"
		description: "Child command that displays its own flag and arg values"
		implementations: [
			{
				script: #"""
					echo "  [CHILD] INVOWK_ARG_MESSAGE = '${INVOWK_ARG_MESSAGE:-<not set>}'"
					echo "  [CHILD] INVOWK_FLAG_CHILD_FLAG = '${INVOWK_FLAG_CHILD_FLAG:-<not set>}'"
					echo "  [CHILD] INVOWK_ARG_PARENT_ONLY = '${INVOWK_ARG_PARENT_ONLY:-<not set>}' (should be <not set>)"
					echo "  [CHILD] INVOWK_FLAG_PARENT_FLAG = '${INVOWK_FLAG_PARENT_FLAG:-<not set>}' (should be <not set>)"
					"""#
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "message", description: "A message to display", default_value: "hello from child"},
		]
		flags: [
			{name: "child-flag", description: "A flag specific to child", default_value: "child-default"},
		]
	},

	// Example 14.2: Parent command that demonstrates environment isolation
	{
		name:        "isolation parent"
		description: "Parent command demonstrating env var isolation when calling child"
		implementations: [
			{
				script: #"""
					echo "=========================================="
					echo "  Environment Variable Isolation Demo"
					echo "=========================================="
					echo ""
					echo "BEFORE calling child command:"
					echo "  [PARENT] INVOWK_ARG_PARENT_ONLY = '$INVOWK_ARG_PARENT_ONLY'"
					echo "  [PARENT] INVOWK_FLAG_PARENT_FLAG = '$INVOWK_FLAG_PARENT_FLAG'"
					echo ""
					echo "Calling child command with its own args/flags..."
					echo "  invowk cmd examples isolation child 'message from parent' --child-flag=from-parent"
					echo ""
					echo "--- Child command output ---"
					invowk cmd examples isolation child "message from parent" --child-flag=from-parent
					echo "--- End child output ---"
					echo ""
					echo "AFTER child command returns:"
					echo "  [PARENT] INVOWK_ARG_PARENT_ONLY = '$INVOWK_ARG_PARENT_ONLY' (unchanged)"
					echo "  [PARENT] INVOWK_FLAG_PARENT_FLAG = '$INVOWK_FLAG_PARENT_FLAG' (unchanged)"
					echo ""
					echo "Key observations:"
					echo "  1. Child did NOT see parent's INVOWK_ARG_PARENT_ONLY or INVOWK_FLAG_PARENT_FLAG"
					echo "  2. Child received its own INVOWK_ARG_MESSAGE and INVOWK_FLAG_CHILD_FLAG"
					echo "  3. Parent's env vars remain intact after child returns"
					echo ""
					echo "This isolation prevents accidental leakage of flags/args between commands."
					echo "=========================================="
					"""#
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		args: [
			{name: "parent-only", description: "An argument only the parent has", default_value: "parent-secret-value"},
		]
		flags: [
			{name: "parent-flag", description: "A flag only the parent has", default_value: "parent-flag-value"},
		]
		depends_on: {
			tools: [
				{alternatives: ["invowk"]},
			]
		}
	},

	// Example 14.3: Demonstrate isolation with virtual runtime
	{
		name:        "isolation virtual"
		description: "Environment isolation demo using virtual shell runtime"
		implementations: [
			{
				script: #"""
					echo "=== Virtual Runtime Isolation Demo ==="
					echo ""
					echo "Parent values (virtual runtime):"
					echo "  INVOWK_ARG_DATA = '$INVOWK_ARG_DATA'"
					echo "  INVOWK_FLAG_MODE = '$INVOWK_FLAG_MODE'"
					echo ""
					echo "The isolation mechanism works identically in virtual runtime."
					echo "INVOWK_ARG_* and INVOWK_FLAG_* variables are filtered from the"
					echo "inherited environment before each command execution."
					echo ""
					echo "This ensures consistent behavior across native, virtual, and"
					echo "container runtimes."
					"""#
				runtimes: [{name: "virtual"}]
			}
		]
		args: [
			{name: "data", description: "Some data value", default_value: "virtual-data"},
		]
		flags: [
			{name: "mode", description: "Operation mode", default_value: "virtual-mode"},
		]
	},

	// ============================================================================
	// SECTION 15: Environment Variable Dependencies
	// ============================================================================
	// These commands demonstrate the env_vars dependency type, which validates
	// that required environment variables exist in the user's environment BEFORE
	// invowk sets any command-level env vars. This is useful for commands that
	// require credentials, API keys, or other configuration from the environment.

	// Example 15.1: Single required environment variable
	{
		name:        "deps env single"
		description: "Command requiring a single environment variable"
		implementations: [
			{
				script: """
					echo "=== Single Env Var Dependency Demo ==="
					echo ""
					echo "Required environment variable is set:"
					echo "  HOME = '$HOME'"
					echo ""
					echo "The env_vars dependency validated that HOME exists in your"
					echo "environment before the command started."
					"""
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			env_vars: [
				{alternatives: [{name: "HOME"}]},
			]
		}
	},

	// Example 15.2: Environment variable with alternatives (OR semantics)
	{
		name:        "deps env alternatives"
		description: "Command requiring any of several environment variables"
		implementations: [
			{
				script: #"""
					echo "=== Env Var Alternatives Demo ==="
					echo ""
					echo "At least one of these variables is set:"
					echo "  EDITOR = '${EDITOR:-<not set>}'"
					echo "  VISUAL = '${VISUAL:-<not set>}'"
					echo "  PAGER = '${PAGER:-<not set>}'"
					echo ""
					echo "The env_vars dependency uses OR semantics - if ANY of the"
					echo "alternatives is set, the dependency is satisfied."
					"""#
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			env_vars: [
				// Any one of these satisfies the dependency
				{alternatives: [{name: "EDITOR"}, {name: "VISUAL"}, {name: "PAGER"}]},
			]
		}
	},

	// Example 15.3: Environment variable with regex validation
	{
		name:        "deps env validated"
		description: "Command with regex-validated environment variable"
		implementations: [
			{
				script: """
					echo "=== Validated Env Var Demo ==="
					echo ""
					echo "Required environment variable with validation:"
					echo "  USER = '$USER'"
					echo ""
					echo "The USER variable must:"
					echo "  1. Exist in your environment"
					echo "  2. Match the pattern: ^[a-zA-Z][a-zA-Z0-9_-]*$"
					echo "     (start with letter, contain only alphanumerics, underscore, hyphen)"
					echo ""
					echo "This is useful for validating format of credentials, API keys, etc."
					"""
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			env_vars: [
				// USER must exist and match the pattern (username format)
				{alternatives: [{name: "USER", validation: "^[a-zA-Z][a-zA-Z0-9_-]*$"}]},
			]
		}
	},

	// Example 15.4: Multiple environment variable dependencies
	{
		name:        "deps env multiple"
		description: "Command requiring multiple environment variables"
		implementations: [
			{
				script: """
					echo "=== Multiple Env Var Dependencies Demo ==="
					echo ""
					echo "All required environment variables are set:"
					echo "  HOME = '$HOME'"
					echo "  USER = '$USER'"
					echo "  PATH = '$PATH' (truncated)"
					echo ""
					echo "Each entry in env_vars is validated independently."
					echo "All entries must be satisfied for the command to run."
					"""
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			env_vars: [
				{alternatives: [{name: "HOME"}]},
				{alternatives: [{name: "USER"}]},
				{alternatives: [{name: "PATH"}]},
			]
		}
	},

	// Example 15.5: Combining env_vars with other dependency types
	{
		name:        "deps env combined"
		description: "Command combining env_vars with other dependency checks"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Combined Dependencies Demo"
					echo "=========================================="
					echo ""
					echo "All dependency checks passed:"
					echo ""
					echo "  [OK] Environment Variables:"
					echo "       HOME = '$HOME'"
					echo ""
					echo "  [OK] Tools:"
					echo "       sh is available"
					echo ""
					echo "  [OK] Files:"
					echo "       README.md exists and is readable"
					echo ""
					echo "The env_vars check runs FIRST, before all other"
					echo "dependency checks. This ensures validation against"
					echo "the user's actual environment, not variables that"
					echo "might be set by invowk's 'env' construct."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			}
		]
		depends_on: {
			// env_vars are checked FIRST (before tools, filepaths, etc.)
			env_vars: [
				{alternatives: [{name: "HOME"}]},
			]
			tools: [
				{alternatives: ["sh"]},
			]
			filepaths: [
				{alternatives: ["README.md"], readable: true},
			]
		}
	},

	// Example 15.6: Implementation-level env_vars dependency (container example)
	{
		name:        "deps env container"
		description: "Container command with env_vars at implementation level"
		implementations: [
			{
				script: """
					echo "=== Container Env Var Dependencies Demo ==="
					echo ""
					echo "Environment variables checked INSIDE the container:"
					echo "  PATH = '$PATH'"
					echo "  HOME = '$HOME'"
					echo ""
					echo "Implementation-level env_vars dependencies are validated"
					echo "inside the container environment, not on the host."
					echo ""
					echo "This is useful for ensuring the container image has"
					echo "required environment variables configured."
					"""
				runtimes: [{name: "container", image: "debian:stable-slim"}]
				// These env_vars are validated INSIDE the container
				depends_on: {
					env_vars: [
						{alternatives: [{name: "PATH"}]},
						{alternatives: [{name: "HOME"}]},
					]
				}
			}
		]
	},

	// ============================================================================
	// SECTION 16: Environment Configuration (env block)
	// ============================================================================
	// These commands demonstrate the env block feature, which provides
	// environment configuration through files and vars. This is useful for:
	// - Loading configuration from .env files
	// - Separating sensitive values from invkfile definitions
	// - Supporting different environments (dev, staging, prod)
	//
	// Key features:
	// - env.files: list of dotenv files to load (later files override earlier)
	// - env.vars: inline key-value pairs (override values from files)
	// - Paths are relative to the invkfile location
	// - Optional files: suffix with ? (e.g., ".env.local?")
	// - Use forward slashes for cross-platform compatibility
	//
	// Precedence (lowest to highest):
	// 1. Command-level env.files
	// 2. Implementation-level env.files
	// 3. Command-level env.vars
	// 4. Implementation-level env.vars
	// 5. ExtraEnv (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn, ARGC)
	// 6. --env-file flag (runtime)
	// 7. --env-var flag (runtime, highest priority)

	// Example 16.1: Basic env.files at command level
	{
		name:        "env files basic"
		description: "Load environment from .env file at command level"
		env: {
			files: ["examples/.env"]
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Basic env.files Demo"
					echo "=========================================="
					echo ""
					echo "Variables loaded from examples/.env:"
					echo "  APP_NAME    = '$APP_NAME'"
					echo "  APP_VERSION = '$APP_VERSION'"
					echo "  APP_ENV     = '$APP_ENV'"
					echo "  ENABLE_DEBUG= '$ENABLE_DEBUG'"
					echo "  LOG_LEVEL   = '$LOG_LEVEL'"
					echo ""
					echo "The env.files field loads dotenv files before"
					echo "command execution. Paths are relative to the"
					echo "invkfile location."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}, {name: "virtual"}]
			},
		]
	},

	// Example 16.2: Optional env files with ? suffix
	{
		name:        "env files optional"
		description: "Load optional .env.local file (may not exist)"
		env: {
			files: ["examples/.env", "examples/.env.local?", "examples/.env.missing?"]
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Optional env.files Demo"
					echo "=========================================="
					echo ""
					echo "Variables loaded (with optional overrides):"
					echo "  APP_ENV       = '$APP_ENV'"
					echo "  LOG_LEVEL     = '$LOG_LEVEL'"
					echo "  LOCAL_ONLY_VAR= '${LOCAL_ONLY_VAR:-<not set>}'"
					echo ""
					echo "Files loaded:"
					echo "  [required] examples/.env"
					echo "  [optional] examples/.env.local? (loaded if exists)"
					echo "  [optional] examples/.env.missing? (skipped, doesn't exist)"
					echo ""
					echo "The ? suffix makes a file optional. Missing optional"
					echo "files are silently skipped without error."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			},
		]
	},

	// Example 16.3: env.files with subdirectory paths
	{
		name:        "env files subdirectory"
		description: "Load env files from subdirectories"
		env: {
			files: ["examples/.env", "examples/config/database.env"]
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Subdirectory env.files Demo"
					echo "=========================================="
					echo ""
					echo "App variables (from examples/.env):"
					echo "  APP_NAME = '$APP_NAME'"
					echo "  APP_ENV  = '$APP_ENV'"
					echo ""
					echo "Database variables (from examples/config/database.env):"
					echo "  DB_HOST = '$DB_HOST'"
					echo "  DB_PORT = '$DB_PORT'"
					echo "  DB_NAME = '$DB_NAME'"
					echo "  DB_USER = '$DB_USER'"
					echo ""
					echo "Use forward slashes in paths for cross-platform"
					echo "compatibility. Paths are relative to invkfile location."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			},
		]
	},

	// Example 16.4: Implementation-level env
	{
		name:        "env impl level"
		description: "Different env per implementation"
		env: {
			files: ["examples/.env"]
		}
		implementations: [
			{
				// Implementation adds database config on top of command-level env
				env: {
					files: ["examples/config/database.env"]
				}
				script: """
					echo "=========================================="
					echo "  Implementation-level env Demo"
					echo "=========================================="
					echo ""
					echo "Command-level env.files: examples/.env"
					echo "  APP_NAME = '$APP_NAME'"
					echo "  APP_ENV  = '$APP_ENV'"
					echo ""
					echo "Implementation-level env.files: examples/config/database.env"
					echo "  DB_HOST = '$DB_HOST'"
					echo "  DB_NAME = '$DB_NAME'"
					echo ""
					echo "Implementation-level env.files are loaded AFTER"
					echo "command-level, so they can override command-level values."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			},
		]
	},

	// Example 16.5: env.vars overriding env.files
	{
		name:        "env vars override"
		description: "Inline env.vars override env.files values"
		env: {
			files: ["examples/.env"]
			vars: {
				APP_ENV:   "production"
				LOG_LEVEL: "warn"
			}
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  env.files + env.vars Demo"
					echo "=========================================="
					echo ""
					echo "From env.files (examples/.env):"
					echo "  APP_NAME    = '$APP_NAME' (from file)"
					echo "  APP_VERSION = '$APP_VERSION' (from file)"
					echo ""
					echo "Overridden by env.vars:"
					echo "  APP_ENV   = '$APP_ENV' (vars overrides 'development')"
					echo "  LOG_LEVEL = '$LOG_LEVEL' (vars overrides 'info')"
					echo ""
					echo "Precedence: env.files < env.vars"
					echo "Inline vars always take priority over files."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}, {name: "virtual"}]
			},
		]
	},

	// Example 16.6: env in container runtime
	{
		name:        "env container"
		description: "Use env with container execution"
		env: {
			files: ["examples/.env"]
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Container env Demo"
					echo "=========================================="
					echo ""
					echo "Environment variables inside container:"
					echo "  APP_NAME    = '$APP_NAME'"
					echo "  APP_VERSION = '$APP_VERSION'"
					echo "  APP_ENV     = '$APP_ENV'"
					echo ""
					echo "env works with all runtimes including containers."
					echo "Variables are loaded from host filesystem and passed"
					echo "to the container environment."
					echo "=========================================="
					"""
				runtimes: [{name: "container", image: "debian:stable-slim"}]
			},
		]
	},

	// Example 16.7: Runtime --env-file flag demonstration
	{
		name:        "env runtime flag"
		description: "Use --env-file flag to load additional files at runtime"
		env: {
			files: ["examples/.env"]
		}
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Runtime --env-file Flag Demo"
					echo "=========================================="
					echo ""
					echo "Current values:"
					echo "  APP_NAME  = '$APP_NAME'"
					echo "  APP_ENV   = '$APP_ENV'"
					echo "  LOG_LEVEL = '$LOG_LEVEL'"
					echo ""
					echo "To override at runtime, use the --env-file flag:"
					echo ""
					echo "  invowk run 'env runtime flag' --env-file .env.prod"
					echo "  invowk run 'env runtime flag' -e .env.prod"
					echo ""
					echo "Multiple --env-file flags can be specified:"
					echo ""
					echo "  invowk run 'env runtime flag' -e .env.prod -e secrets.env"
					echo ""
					echo "Runtime --env-file has highest precedence, overriding"
					echo "all other env.files and env.vars definitions."
					echo ""
					echo "Paths for --env-file are relative to current directory,"
					echo "not the invkfile location."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			},
		]
	},

	// ============================================================================
	// SECTION 17: Working Directory
	// ============================================================================
	// These commands demonstrate the workdir feature at different levels.
	// The workdir defines where commands execute, with hierarchical override:
	//   CLI flag (--workdir) > Implementation > Command > Root > Default (invkfile dir)
	//
	// Paths should use forward slashes (/) for cross-platform compatibility.
	// Relative paths are resolved from the invkfile directory.

	// Example 17.1: Command-level workdir
	// The workdir is set at the command level, affecting all implementations
	{
		name:        "workdir command level"
		description: "Execute in a specific directory (command-level workdir)"
		workdir:     "/tmp"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Command-Level Working Directory"
					echo "=========================================="
					echo ""
					echo "Configured workdir: /tmp"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "This command runs in /tmp because workdir"
					echo "is set at the command level."
					echo "=========================================="
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},

	// Example 17.2: Implementation-level workdir (overrides command-level)
	// Each implementation can specify its own workdir, overriding the command level
	{
		name:        "workdir impl level"
		description: "Implementation-level workdir overrides command-level"
		workdir:     "/tmp"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Implementation-Level Working Directory"
					echo "=========================================="
					echo ""
					echo "Command workdir: /tmp"
					echo "Implementation workdir: /var"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "Implementation-level workdir takes precedence"
					echo "over command-level workdir."
					echo "=========================================="
					"""
				workdir: "/var"
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},

	// Example 17.3: Relative workdir paths
	// Relative paths are resolved from the invkfile directory
	{
		name:        "workdir relative"
		description: "Relative workdir resolved from invkfile location"
		workdir:     "examples"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Relative Working Directory"
					echo "=========================================="
					echo ""
					echo "Configured workdir: examples"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "Relative paths are resolved from the"
					echo "invkfile directory location."
					echo ""
					echo "Contents of current directory:"
					ls -la 2>/dev/null || echo "(directory may not exist)"
					echo "=========================================="
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},

	// Example 17.4: Cross-platform workdir with forward slashes
	// Always use forward slashes for cross-platform compatibility
	{
		name:        "workdir cross platform"
		description: "Cross-platform workdir using forward slashes"
		workdir:     "examples/nested/path"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Cross-Platform Working Directory"
					echo "=========================================="
					echo ""
					echo "Configured workdir: examples/nested/path"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "Use forward slashes (/) for paths - invowk"
					echo "converts them to the platform's separator."
					echo "=========================================="
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
			{
				script: """
					echo ==========================================
					echo   Cross-Platform Working Directory
					echo ==========================================
					echo.
					echo Configured workdir: examples/nested/path
					echo Actual pwd: %CD%
					echo.
					echo Use forward slashes (/) for paths - invowk
					echo converts them to the platform's separator.
					echo ==========================================
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "windows"}]
			},
		]
	},

	// Example 17.5: CLI --workdir flag override demonstration
	// The --workdir flag has the highest precedence, overriding all levels
	{
		name:        "workdir cli override"
		description: "Use --workdir flag to override at runtime"
		workdir:     "/tmp"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  CLI --workdir Flag Override"
					echo "=========================================="
					echo ""
					echo "Command workdir: /tmp"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "To override at runtime, use the --workdir flag:"
					echo ""
					echo "  invowk run 'workdir cli override' --workdir /var"
					echo "  invowk run 'workdir cli override' -w /home"
					echo ""
					echo "The CLI flag has highest precedence:"
					echo ""
					echo "  1. CLI flag (--workdir / -w)     <- Highest"
					echo "  2. Implementation-level workdir"
					echo "  3. Command-level workdir"
					echo "  4. Root-level workdir"
					echo "  5. Default (invkfile directory)  <- Lowest"
					echo "=========================================="
					"""
				workdir: "/var"
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},

	// Example 17.6: Workdir with container runtime
	// Container workdir is mapped to /workspace/<path> inside the container
	{
		name:        "workdir container"
		description: "Working directory in container runtime"
		workdir:     "examples"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Container Working Directory"
					echo "=========================================="
					echo ""
					echo "Configured workdir: examples"
					echo "Actual pwd: $(pwd)"
					echo ""
					echo "In container runtime, the workdir path is"
					echo "mapped to /workspace/<path> inside the container."
					echo ""
					echo "Host 'examples' -> Container '/workspace/examples'"
					echo ""
					echo "Contents of current directory:"
					ls -la
					echo "=========================================="
					"""
				runtimes: [{
					name:  "container"
					image: "debian:stable-slim"
				}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},

	// ============================================================================
	// SECTION 18: Root-Level Dependencies (depends_on at file level)
	// ============================================================================
	// These commands demonstrate the root-level depends_on feature.
	// Root-level dependencies are validated BEFORE any command runs and apply
	// to ALL commands in the invkfile. This is useful for:
	// - Shared prerequisites (tools, capabilities) needed by all commands
	// - Global environment requirements (API keys, credentials)
	// - Common file dependencies
	//
	// Merge order (lowest to highest priority):
	// 1. Root-level depends_on     <- validated first, lowest priority
	// 2. Command-level depends_on  <- can add to or override root
	// 3. Implementation-level      <- highest priority, most specific
	//
	// Note: This invkfile has root-level depends_on: tools: [{alternatives: ["sh", "bash"]}]
	// which ensures a shell is available for all commands.

	// Example 18.1: Command inheriting root-level dependencies
	{
		name:        "root deps inherited"
		description: "Command that inherits root-level tool dependency (sh/bash)"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Root-Level Dependencies Demo"
					echo "=========================================="
					echo ""
					echo "This command has NO explicit depends_on, but still"
					echo "benefits from the root-level dependency check."
					echo ""
					echo "Root-level depends_on ensures 'sh' or 'bash' is"
					echo "available before ANY command in this file runs."
					echo ""
					echo "This is useful for shared prerequisites that apply"
					echo "to all commands in the invkfile."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			}
		]
		// No depends_on here - inherits from root level
	},

	// Example 18.2: Command adding to root-level dependencies
	{
		name:        "root deps extended"
		description: "Command that extends root-level dependencies"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Extended Root Dependencies Demo"
					echo "=========================================="
					echo ""
					echo "Dependency checks performed (in order):"
					echo ""
					echo "  1. Root-level: sh or bash available"
					echo "  2. Command-level: README.md exists and readable"
					echo ""
					echo "This command ADDS its own dependency on top of"
					echo "the root-level dependencies."
					echo ""
					echo "Root + Command dependencies are merged:"
					echo "  - tools: [sh/bash] (from root)"
					echo "  - filepaths: [README.md] (from command)"
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			}
		]
		// This ADDS to root-level dependencies
		depends_on: {
			filepaths: [
				{alternatives: ["README.md"], readable: true},
			]
		}
	},

	// Example 18.3: Full hierarchy - Root + Command + Implementation
	{
		name:        "root deps full hierarchy"
		description: "Demonstrate full three-level dependency hierarchy"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Full Dependency Hierarchy Demo"
					echo "=========================================="
					echo ""
					echo "This command demonstrates all three levels of"
					echo "depends_on being merged together:"
					echo ""
					echo "  1. ROOT LEVEL (from invkfile):"
					echo "     - tools: [sh, bash]"
					echo ""
					echo "  2. COMMAND LEVEL:"
					echo "     - capabilities: [internet]"
					echo ""
					echo "  3. IMPLEMENTATION LEVEL:"
					echo "     - filepaths: [README.md]"
					echo ""
					echo "All three levels are validated before execution."
					echo "Later levels can add new checks or override earlier ones."
					echo "=========================================="
					"""
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
				// Implementation-level dependencies (highest priority)
				depends_on: {
					filepaths: [
						{alternatives: ["README.md"]},
					]
				}
			}
		]
		// Command-level dependencies (middle priority)
		depends_on: {
			capabilities: [
				{alternatives: ["internet"]},
			]
		}
	},

	// Example 18.4: Root-level env_vars dependency scenario
	// This demonstrates how root-level env_vars could be used for API keys
	{
		name:        "root deps env vars scenario"
		description: "Scenario: root-level env var checks for API credentials"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Root-Level Env Vars Scenario"
					echo "=========================================="
					echo ""
					echo "Imagine an invkfile with root-level depends_on:"
					echo ""
					echo "  depends_on: {"
					echo "    env_vars: ["
					echo "      {alternatives: [{name: \"API_KEY\"}]},"
					echo "      {alternatives: [{name: \"API_SECRET\"}]},"
					echo "    ]"
					echo "  }"
					echo ""
					echo "Every command would automatically require these"
					echo "environment variables to be set before execution."
					echo ""
					echo "Current invkfile only requires sh/bash at root level."
					echo "This example shows how the feature could be extended."
					echo "=========================================="
					"""
				runtimes: [{name: "native"}]
			}
		]
	},

	// Example 18.5: Container command with root-level + impl-level deps
	{
		name:        "root deps container"
		description: "Container command with root and implementation dependencies"
		implementations: [
			{
				script: """
					echo "=========================================="
					echo "  Container with Root Dependencies"
					echo "=========================================="
					echo ""
					echo "Root-level dependencies are checked on HOST:"
					echo "  - tools: [sh, bash] (verified on host)"
					echo ""
					echo "Implementation-level dependencies checked in CONTAINER:"
					echo "  - tools: [sh, ash] (verified inside container)"
					echo "  - filepaths: [/etc/os-release] (inside container)"
					echo ""
					echo "Container OS:"
					cat /etc/os-release | head -2
					echo ""
					echo "Root deps ensure host prerequisites are met."
					echo "Implementation deps validate container environment."
					echo "=========================================="
					"""
				runtimes: [{name: "container", image: "debian:stable-slim"}]
				// These are validated INSIDE the container
				depends_on: {
					tools: [
						{alternatives: ["sh", "ash"]},
					]
					filepaths: [
						{alternatives: ["/etc/os-release"]},
					]
				}
			}
		]
		// This is validated on the HOST (merged with root-level)
		depends_on: {
			capabilities: [
				{alternatives: ["internet"]},  // Needed to pull container image
			]
		}
	},

	// ============================================================================
	// SECTION 19: Interactive Mode with TUI Components
	// ============================================================================
	// These commands demonstrate the interactive mode (-i flag) which enables
	// nested invowk tui commands to delegate their rendering to the parent process.
	// When running with -i, the parent starts an HTTP server and child processes
	// communicate via INVOWK_TUI_ADDR and INVOWK_TUI_TOKEN environment variables.

	// Example 19.1: Interactive demo showcasing multiple TUI components
	{
		name:        "interactive demo"
		description: "Demonstrate interactive mode with nested TUI components (run with -i flag)"
		implementations: [
			{
				script: """
					#!/bin/sh
					echo "=========================================="
					echo "  Interactive Mode Demo"
					echo "=========================================="
					echo ""
					echo "This demo shows how nested 'invowk tui' commands"
					echo "delegate their rendering to the parent process"
					echo "when running in interactive mode (-i flag)."
					echo ""

					# Check if running in interactive mode
					if [ -n "$INVOWK_TUI_ADDR" ]; then
					    echo "Running in interactive mode"
					    echo "  Server: $INVOWK_TUI_ADDR"
					    echo ""
					else
					    echo "Not running in interactive mode"
					    echo "  Run this command with: invowk cmd -i examples interactive demo"
					    exit 1
					fi

					# Demo 1: Text input
					echo "--- Demo 1: Text Input ---"
					name=$(invowk tui input --title "What is your name?" --placeholder "Enter your name...")
					echo "Hello, $name!"
					echo ""

					# Demo 2: Confirmation
					echo "--- Demo 2: Confirmation ---"
					if invowk tui confirm --title "Do you want to continue with more demos?"; then
					    echo "Great! Continuing with more demos..."
					else
					    echo "Okay, stopping here. Goodbye!"
					    exit 0
					fi
					echo ""

					# Demo 3: Single selection
					echo "--- Demo 3: Single Selection ---"
					color=$(invowk tui choose --title "Pick your favorite color" red green blue yellow purple)
					echo "You chose: $color"
					echo ""

					# Demo 4: Multiple selection
					echo "--- Demo 4: Multiple Selection ---"
					echo "Select your favorite programming languages:"
					languages=$(invowk tui choose --title "Select languages (space to toggle, enter to confirm)" --no-limit go python rust javascript typescript ruby)
					echo "You selected: $languages"
					echo ""

					# Demo 5: Filter selection
					echo "--- Demo 5: Filter Selection ---"
					fruit=$(invowk tui filter --title "Search for a fruit" apple banana cherry date elderberry fig grape honeydew kiwi lemon mango)
					echo "You filtered and selected: $fruit"
					echo ""

					# Demo 6: Spinner for long operation
					echo "--- Demo 6: Spinner ---"
					invowk tui spin --title "Processing your selections..." -- sleep 2
					echo "Processing complete!"
					echo ""

					# Summary
					echo "=========================================="
					echo "  Demo Complete!"
					echo "=========================================="
					echo ""
					echo "Summary:"
					echo "  Name: $name"
					echo "  Favorite color: $color"
					echo "  Languages: $languages"
					echo "  Fruit: $fruit"
					echo ""
					echo "All TUI components were rendered by the parent"
					echo "process via HTTP delegation. This allows scripts"
					echo "running in PTY mode to display rich TUI interfaces"
					echo "without terminal ownership conflicts."
					echo "=========================================="
					"""
				runtimes: [{name: "native"},{name: "virtual"},{
					name: "container"
					image: "python:3-slim"
					}]
			}
		]
	},

	// Example 19.2: Interactive file browser demo
	{
		name:        "interactive file"
		description: "Demonstrate interactive file picker (run with -i flag)"
		implementations: [
			{
				script: """
					#!/bin/sh
					echo "=========================================="
					echo "  Interactive File Picker Demo"
					echo "=========================================="
					echo ""

					if [ -z "$INVOWK_TUI_ADDR" ]; then
					    echo "Run with: invowk cmd -i examples interactive file"
					    exit 1
					fi

					# Pick a file
					file=$(invowk tui file --title "Select a file to view")
					if [ -z "$file" ]; then
					    echo "No file selected."
					    exit 0
					fi

					echo "Selected: $file"
					echo ""

					# Show file info
					echo "File info:"
					ls -la "$file"
					echo ""

					# Ask if user wants to view contents
					if invowk tui confirm --title "View file contents?"; then
					    echo "--- File Contents ---"
					    head -50 "$file"
					    echo ""
					    echo "--- End of preview (first 50 lines) ---"
					fi
					"""
				runtimes: [{name: "native"}]
			}
		]
	},

	// Example 19.3: Interactive table display
	{
		name:        "interactive table"
		description: "Demonstrate interactive table display (run with -i flag)"
		implementations: [
			{
				script: """
					#!/bin/sh
					echo "=========================================="
					echo "  Interactive Table Demo"
					echo "=========================================="
					echo ""

					if [ -z "$INVOWK_TUI_ADDR" ]; then
					    echo "Run with: invowk cmd -i examples interactive table"
					    exit 1
					fi

					# Display a table of data
					echo "Displaying process information in a table..."
					echo ""

					invowk tui table --title "Top Processes" --columns "PID,USER,MEM,CPU,COMMAND" --row "1,root,5.2,0.1,systemd" --row "1234,user,3.8,2.5,firefox" --row "5678,user,2.1,15.3,node"

					echo ""
					echo "Table displayed successfully!"
					"""
				runtimes: [{name: "native"}]
			}
		]
	},

	// Example 19.4: Interactive pager demo
	{
		name:        "interactive pager"
		description: "Demonstrate interactive pager for long content (run with -i flag)"
		implementations: [
			{
				script: """
					#!/bin/sh
					echo "=========================================="
					echo "  Interactive Pager Demo"
					echo "=========================================="
					echo ""

					if [ -z "$INVOWK_TUI_ADDR" ]; then
					    echo "Run with: invowk cmd -i examples interactive pager"
					    exit 1
					fi

					# Generate some content to page through
					echo "Generating content for the pager..."

					# Use echo to generate content instead of heredoc
					{
					echo "========================================"
					echo "    Welcome to the Invowk Pager Demo"
					echo "========================================"
					echo ""
					echo "This is a demonstration of the interactive pager component."
					echo "The pager allows you to scroll through long content using"
					echo "keyboard navigation."
					echo ""
					echo "NAVIGATION:"
					echo "  - j/down  : Scroll down"
					echo "  - k/up    : Scroll up"
					echo "  - g/Home  : Go to top"
					echo "  - G/End   : Go to bottom"
					echo "  - q/Esc   : Exit pager"
					echo ""
					echo "FEATURES:"
					echo "  - Smooth scrolling"
					echo "  - Line number display"
					echo "  - Search functionality (coming soon)"
					echo ""
					echo "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
					echo "Sed do eiusmod tempor incididunt ut labore et dolore magna"
					echo "aliqua. Ut enim ad minim veniam, quis nostrud exercitation"
					echo "ullamco laboris nisi ut aliquip ex ea commodo consequat."
					echo ""
					echo "Duis aute irure dolor in reprehenderit in voluptate velit"
					echo "esse cillum dolore eu fugiat nulla pariatur. Excepteur sint"
					echo "occaecat cupidatat non proident, sunt in culpa qui officia"
					echo "deserunt mollit anim id est laborum."
					echo ""
					echo "========================================"
					echo "         End of Demo Content"
					echo "========================================"
					} | invowk tui pager --title "Demo Content"

					echo ""
					echo "Pager closed successfully!"
					"""
				runtimes: [{name: "native"}]
			}
		]
	},

	// Example 19.5: Interactive multi-line text input
	{
		name:        "interactive write"
		description: "Demonstrate interactive multi-line text editor (run with -i flag)"
		implementations: [
			{
				script: """
					#!/bin/sh
					echo "=========================================="
					echo "  Interactive Text Editor Demo"
					echo "=========================================="
					echo ""

					if [ -z "$INVOWK_TUI_ADDR" ]; then
					    echo "Run with: invowk cmd -i examples interactive write"
					    exit 1
					fi

					echo "Opening text editor..."
					echo "Write a short note or message, then press Ctrl+D to submit."
					echo ""

					# Open multi-line text editor
					note=$(invowk tui write --title "Write your note" --placeholder "Type your message here...")

					if [ -z "$note" ]; then
					    echo "No content entered."
					    exit 0
					fi

					echo "--- Your Note ---"
					echo "$note"
					echo "--- End of Note ---"
					echo ""

					# Ask what to do with it
					action=$(invowk tui choose --title "What would you like to do with this note?" --mode single "Save to file" "Copy to clipboard" "Discard")

					case "$action" in
					    "Save to file")
					        filename=$(invowk tui input --title "Enter filename" --placeholder "note.txt")
					        echo "$note" > "/tmp/$filename"
					        echo "Saved to /tmp/$filename"
					        ;;
					    "Copy to clipboard")
					        echo "(Clipboard copy would happen here)"
					        echo "Note content ready for copying."
					        ;;
					    "Discard")
					        echo "Note discarded."
					        ;;
					esac
					"""
				runtimes: [{name: "native"}]
			}
		]
	},
]
