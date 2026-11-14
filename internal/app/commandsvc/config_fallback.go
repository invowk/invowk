// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

// LoadConfigWithFallback loads configuration via the provider.
//
// Explicit sources fail closed: when --ivk-config or INVOWK_CONFIG selected
// the file (the provider reports config.ErrExplicitConfig), it returns
// (nil, nil, err) and callers must stop before discovery or execution. A
// non-empty configPath is always the --ivk-config value, so a provider error
// for it is treated as explicit even when the provider did not wrap it.
//
// Only a failing user config file (no explicit source) keeps the historical
// behaviour: built-in defaults plus a config-load diagnostic, so command
// execution stays operational with fewer capabilities rather than more.
//
//goplint:ignore -- configPath matches ConfigFallbackFunc and is validated by the config provider.
func LoadConfigWithFallback(ctx context.Context, provider config.Loader, configPath string) (cfg *config.Config, diags []Diagnostic, err error) {
	configFilePath := types.FilesystemPath(configPath) //goplint:ignore -- CLI config path is validated by provider.Load.
	cfg, loadErr := provider.Load(ctx, config.LoadOptions{ConfigFilePath: configFilePath})
	if loadErr == nil {
		return cfg, nil, nil
	}

	if errors.Is(loadErr, config.ErrExplicitConfig) {
		return nil, nil, loadErr
	}
	if configPath != "" {
		return nil, nil, &config.ExplicitConfigError{Source: config.ConfigSourceFlag, Path: configFilePath, Err: loadErr}
	}

	severity := DiagnosticSeverityError
	if errors.Is(loadErr, os.ErrNotExist) {
		severity = DiagnosticSeverityWarning
	}

	diag, diagErr := NewDiagnosticWithCause(
		severity,
		DiagnosticCodeConfigLoadFailed,
		fmt.Sprintf("failed to load config, using defaults: %v", loadErr),
		"",
		loadErr,
	)
	if diagErr != nil {
		slog.Error("BUG: failed to create config-load diagnostic", "error", diagErr)
		return config.DefaultConfig(), nil, nil
	}
	return config.DefaultConfig(), []Diagnostic{diag}, nil
}
