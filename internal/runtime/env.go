// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"invowk-cli/pkg/invkfile"
	"maps"
	"os"
)

type envInheritConfig struct {
	mode  invkfile.EnvInheritMode
	allow []string
	deny  []string
}

// buildRuntimeEnv builds the environment for the command with proper precedence:
// 1. Host environment (filtered and optionally inherited)
// 2. Root-level env.files (loaded in array order)
// 3. Command-level env.files (loaded in array order)
// 4. Implementation-level env.files (loaded in array order)
// 5. Root-level env.vars (inline static variables)
// 6. Command-level env.vars (inline static variables)
// 7. Implementation-level env.vars (inline static variables)
// 8. ExtraEnv: INVOWK_FLAG_*, INVOWK_ARG_*, ARGn, ARGC
// 9. --env-file flag files (loaded in flag order)
// 10. --env-var flag values (KEY=VALUE pairs) - HIGHEST priority
func buildRuntimeEnv(ctx *ExecutionContext, defaultMode invkfile.EnvInheritMode) (map[string]string, error) {
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
	maps.Copy(env, ctx.ExtraEnv)

	// 9. Runtime --env-file flag files
	for _, path := range ctx.RuntimeEnvFiles {
		if err := LoadEnvFileFromCwd(env, path); err != nil {
			return nil, err
		}
	}

	// 10. Runtime --env-var flag values (highest priority)
	maps.Copy(env, ctx.RuntimeEnvVars)

	return env, nil
}

func resolveEnvInheritConfig(ctx *ExecutionContext, defaultMode invkfile.EnvInheritMode) envInheritConfig {
	cfg := envInheritConfig{
		mode: defaultMode,
	}

	if rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime); rtConfig != nil {
		if rtConfig.EnvInheritMode != "" {
			cfg.mode = rtConfig.EnvInheritMode
		}
		if len(rtConfig.EnvInheritAllow) > 0 {
			cfg.allow = append([]string{}, rtConfig.EnvInheritAllow...)
		}
		if len(rtConfig.EnvInheritDeny) > 0 {
			cfg.deny = append([]string{}, rtConfig.EnvInheritDeny...)
		}
	}

	if ctx.EnvInheritModeOverride != "" {
		cfg.mode = ctx.EnvInheritModeOverride
	}
	if ctx.EnvInheritAllowOverride != nil {
		cfg.allow = append([]string{}, ctx.EnvInheritAllowOverride...)
	}
	if ctx.EnvInheritDenyOverride != nil {
		cfg.deny = append([]string{}, ctx.EnvInheritDenyOverride...)
	}

	return cfg
}

func buildHostEnv(cfg envInheritConfig) map[string]string {
	env := make(map[string]string)
	if cfg.mode == invkfile.EnvInheritNone {
		return env
	}

	allowSet := make(map[string]struct{})
	if cfg.mode == invkfile.EnvInheritAllow {
		for _, name := range cfg.allow {
			allowSet[name] = struct{}{}
		}
	}

	denySet := make(map[string]struct{})
	for _, name := range cfg.deny {
		denySet[name] = struct{}{}
	}

	for _, entry := range FilterInvowkEnvVars(os.Environ()) {
		idx := findEnvSeparator(entry)
		if idx == -1 {
			continue
		}
		name := entry[:idx]
		value := entry[idx+1:]

		if cfg.mode == invkfile.EnvInheritAllow {
			if _, ok := allowSet[name]; !ok {
				continue
			}
		}
		if _, denied := denySet[name]; denied {
			continue
		}

		env[name] = value
	}

	return env
}

// validateWorkDir validates that a working directory exists and is accessible.
// This provides a better error message than letting exec fail with a cryptic error.
func validateWorkDir(dir string) error {
	if dir == "" {
		return nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %s", dir)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	return nil
}
