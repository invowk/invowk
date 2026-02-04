// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"maps"

	"invowk-cli/pkg/invkfile"
)

type (
	// EnvBuilder builds environment variables for command execution.
	// It applies a 10-level precedence hierarchy (higher number wins):
	//
	//  1. Host environment (filtered by inherit mode)
	//  2. Root-level env.files
	//  3. Command-level env.files
	//  4. Implementation-level env.files
	//  5. Root-level env.vars
	//  6. Command-level env.vars
	//  7. Implementation-level env.vars
	//  8. ExtraEnv (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn, ARGC)
	//  9. RuntimeEnvFiles (--env-file flag)
	//  10. RuntimeEnvVars (--env-var flag) - HIGHEST priority
	//
	// This interface enables:
	//   - Testability: runtimes can be tested with mock env builders
	//   - Flexibility: alternative env building strategies for specific use cases
	//   - Documentation: the precedence hierarchy is explicitly documented
	EnvBuilder interface {
		Build(ctx *ExecutionContext, defaultMode invkfile.EnvInheritMode) (map[string]string, error)
	}

	// DefaultEnvBuilder implements the standard 10-level precedence for environment building.
	DefaultEnvBuilder struct{}

	// MockEnvBuilder is a test helper that returns a fixed environment map.
	// It can be used to test runtimes in isolation without real env building.
	MockEnvBuilder struct {
		// Env is the environment map to return from Build
		Env map[string]string
		// Err is the error to return from Build (if non-nil)
		Err error
	}
)

// NewDefaultEnvBuilder creates a new DefaultEnvBuilder.
func NewDefaultEnvBuilder() *DefaultEnvBuilder {
	return &DefaultEnvBuilder{}
}

// Build constructs the environment map following the 10-level precedence.
// The defaultMode parameter specifies the default inherit mode when not
// overridden by runtime config or CLI flags.
func (b *DefaultEnvBuilder) Build(ctx *ExecutionContext, defaultMode invkfile.EnvInheritMode) (map[string]string, error) {
	cfg := resolveEnvInheritConfig(ctx, defaultMode)
	env := buildHostEnv(cfg)

	// Determine the base path for resolving env files
	basePath := ctx.Invkfile.GetScriptBasePath()

	// 2. Root-level env.files
	for _, path := range ctx.Invkfile.Env.GetFiles() {
		if err := LoadEnvFile(env, path, basePath); err != nil {
			return nil, err
		}
	}

	// 3. Command-level env.files
	for _, path := range ctx.Command.Env.GetFiles() {
		if err := LoadEnvFile(env, path, basePath); err != nil {
			return nil, err
		}
	}

	// 4. Implementation-level env.files
	for _, path := range ctx.SelectedImpl.Env.GetFiles() {
		if err := LoadEnvFile(env, path, basePath); err != nil {
			return nil, err
		}
	}

	// 5. Root-level env.vars
	maps.Copy(env, ctx.Invkfile.Env.GetVars())

	// 6. Command-level env.vars
	maps.Copy(env, ctx.Command.Env.GetVars())

	// 7. Implementation-level env.vars
	maps.Copy(env, ctx.SelectedImpl.Env.GetVars())

	// 8. Extra env from context (flags, args)
	maps.Copy(env, ctx.Env.ExtraEnv)

	// 9. Runtime --env-file flag files
	for _, path := range ctx.Env.RuntimeEnvFiles {
		if err := LoadEnvFileFromCwd(env, path); err != nil {
			return nil, err
		}
	}

	// 10. Runtime --env-var flag values (highest priority)
	maps.Copy(env, ctx.Env.RuntimeEnvVars)

	return env, nil
}

// Build returns the mock environment or error.
func (m *MockEnvBuilder) Build(_ *ExecutionContext, _ invkfile.EnvInheritMode) (map[string]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Env == nil {
		return make(map[string]string), nil
	}
	// Return a copy to prevent mutations
	result := make(map[string]string, len(m.Env))
	maps.Copy(result, m.Env)
	return result, nil
}
