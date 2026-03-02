// SPDX-License-Identifier: MPL-2.0

// Package deps provides dependency validation logic for invowk commands.
//
// It validates six dependency categories defined in invowkfile depends_on blocks:
// tools (host PATH and container), filepaths (host filesystem and container),
// environment variables, capabilities (network, TTY, etc.), custom check scripts,
// and command discoverability. Validation proceeds in two phases:
//
//   - Phase 1 (Host): Root, command, and implementation-level depends_on are merged
//     and always validated against the HOST system, regardless of the selected runtime.
//   - Phase 2 (Runtime): If the selected runtime is container, the runtime config's
//     depends_on is validated inside the container environment.
//
// The package also provides input validation for flag values and positional arguments.
//
// File organization:
//   - types.go: Exported types, sentinels, constants, CommandSetProvider interface
//   - deps.go: Top-level ValidateDependencies, ValidateHostDependencies,
//     ValidateRuntimeDependencies, CheckCommandDependenciesExist
//   - tools.go: Tool validation (host PATH and container)
//   - filepaths.go: Filepath validation (host filesystem and container)
//   - checks.go: Custom check scripts, env vars, capabilities
//   - helpers.go: Shared helpers (EvaluateAlternatives, NewContainerValidationContext, etc.)
//   - input.go: Flag and argument validation (ValidateFlagValues, ValidateArguments)
package deps
