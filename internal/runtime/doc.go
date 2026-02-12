// SPDX-License-Identifier: MPL-2.0

// Package runtime provides command execution runtimes for Invowk.
//
// Three runtime implementations are available:
//   - native: executes commands using the host shell (bash/sh/PowerShell)
//   - virtual: executes commands using an embedded shell interpreter (mvdan/sh)
//   - container: executes commands inside a container (Docker/Podman)
//
// All runtimes implement the Runtime interface with Name(), Execute(), Available(), and Validate().
// Runtimes supporting output capture implement CapturingRuntime, and those supporting interactive
// mode with PTY attachment implement InteractiveRuntime.
//
// ExecutionContext is the primary data structure for command execution, using composition of
// IOContext (I/O streams), EnvContext (environment configuration), and TUIContext (TUI server).
//
// Environment variable building follows a 10-level precedence hierarchy managed by EnvBuilder.
// See env_builder.go for the full precedence order, from host environment (lowest) to
// --invk-env-var CLI flags (highest).
package runtime
