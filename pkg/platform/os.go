// SPDX-License-Identifier: MPL-2.0

package platform

// OS name constants for runtime.GOOS comparisons.
// Centralizes the string literals to avoid scattered magic strings.
//
// Metadata environment variable names (EnvVar*) are injected into command
// execution contexts and filtered by shouldFilterEnvVar in internal/runtime
// to prevent leakage between nested invowk invocations.
const (
	// Windows is the GOOS value for Windows.
	Windows = "windows"
	// Darwin is the GOOS value for macOS.
	Darwin = "darwin"
	// Linux is the GOOS value for Linux.
	Linux = "linux"
	// EnvVarCmdName is the env var injected with the command name being executed.
	EnvVarCmdName = "INVOWK_CMD_NAME"
	// EnvVarRuntime is the env var injected with the selected runtime type.
	EnvVarRuntime = "INVOWK_RUNTIME"
	// EnvVarSource is the env var injected with the source identifier (invowkfile path or module ID).
	EnvVarSource = "INVOWK_SOURCE"
	// EnvVarPlatform is the env var injected with the resolved platform (linux, macos, windows).
	EnvVarPlatform = "INVOWK_PLATFORM"
)
