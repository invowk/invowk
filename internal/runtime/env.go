// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"os"

	"invowk-cli/pkg/invkfile"
)

type envInheritConfig struct {
	mode  invkfile.EnvInheritMode
	allow []string
	deny  []string
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

	if ctx.Env.InheritModeOverride != "" {
		cfg.mode = ctx.Env.InheritModeOverride
	}
	if ctx.Env.InheritAllowOverride != nil {
		cfg.allow = append([]string{}, ctx.Env.InheritAllowOverride...)
	}
	if ctx.Env.InheritDenyOverride != nil {
		cfg.deny = append([]string{}, ctx.Env.InheritDenyOverride...)
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
