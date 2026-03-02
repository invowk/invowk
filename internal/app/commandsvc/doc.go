// SPDX-License-Identifier: MPL-2.0

// Package commandsvc provides the command execution orchestration service.
//
// It implements the full execution pipeline for invowk commands:
//  1. Config loading and command discovery
//  2. Input validation (flags, arguments, platform compatibility)
//  3. Runtime resolution (CLI override → config default → per-command default)
//  4. SSH server lifecycle management for container host access
//  5. Execution context construction with env var projection
//  6. Dependency validation (tools, cmds, filepaths, capabilities, custom checks, env vars)
//  7. Execution dispatch (timeout → deps → runtime)
//
// The service returns raw typed errors instead of styled ServiceErrors. The CLI
// adapter in cmd/ wraps errors with rendering (lipgloss styles, issue catalog
// rendering). This keeps the service free of presentation concerns and avoids
// a circular dependency on the cmd package.
//
// Dry-run mode returns a [DryRunData] struct in the [Result] instead of writing
// styled output directly. The CLI adapter handles dry-run rendering.
//
// File organization:
//   - doc.go: Package documentation
//   - types.go: Request, Result, DryRunData, local interfaces, ClassifiedError
//   - service.go: Service struct, New(), Execute(), discoverCommand(), resolveDefinitions(), loadConfig()
//   - inputs.go: validateInputs(), resolveRuntime(), buildExecContext()
//   - dispatch.go: dispatchExecution(), executeInteractive(), createRuntimeRegistry(), bridgeTUIRequests()
//   - ssh.go: sshServerController lifecycle management
//   - errors.go: classifyExecutionError() — plain text error classification
package commandsvc
