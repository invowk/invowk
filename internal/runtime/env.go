// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"os"

	"invowk-cli/pkg/invkfile"
)

// envInheritConfig is the resolved environment inheritance configuration after
// applying the 3-level precedence chain: default mode → runtime config overrides
// → CLI flag overrides. Each level can independently override mode, allow list,
// and deny list.
type envInheritConfig struct {
	mode  invkfile.EnvInheritMode
	allow []string
	deny  []string
}

// resolveEnvInheritConfig applies the 3-level precedence chain to produce a final
// inheritance config. Level 1: the caller-supplied defaultMode (runtime-specific default).
// Level 2: the implementation's runtime config block (invkfile per-runtime overrides).
// Level 3: CLI flag overrides from the execution context (--invk-env-inherit-mode, etc.).
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

// buildHostEnv filters the host environment according to the resolved inheritance config.
// Mode "none" returns an empty map. Mode "allow" returns only allowlisted vars.
// Mode "all" returns everything except denylisted vars. In all modes, INVOWK_* vars
// are excluded via FilterInvowkEnvVars to prevent leaking internal state into commands.
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
