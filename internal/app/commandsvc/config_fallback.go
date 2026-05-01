// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/types"
)

// LoadConfigWithFallback loads configuration via the provider. On failure it
// returns defaults with a diagnostic so command execution stays operational.
//
//goplint:ignore -- configPath matches ConfigFallbackFunc and is validated by the config provider.
func LoadConfigWithFallback(ctx context.Context, provider config.Loader, configPath string) (cfg *config.Config, diags []Diagnostic) {
	configFilePath := types.FilesystemPath(configPath) //goplint:ignore -- CLI config path is validated by provider.Load.
	cfg, err := provider.Load(ctx, config.LoadOptions{ConfigFilePath: configFilePath})
	if err == nil {
		return cfg, nil
	}

	if configPath != "" {
		severity := discovery.SeverityError
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, config.ErrConfigFileNotFound) {
			severity = discovery.SeverityWarning
		}
		diag, diagErr := discovery.NewDiagnosticWithCause(
			severity,
			discovery.CodeConfigLoadFailed,
			fmt.Sprintf("failed to load config from %s: %v", configPath, err),
			configFilePath,
			err,
		)
		if diagErr != nil {
			slog.Error("BUG: failed to create config-load diagnostic", "error", diagErr)
			return config.DefaultConfig(), nil
		}
		return config.DefaultConfig(), []Diagnostic{diag}
	}

	severity := discovery.SeverityError
	if errors.Is(err, os.ErrNotExist) {
		severity = discovery.SeverityWarning
	}

	diag, diagErr := discovery.NewDiagnosticWithCause(
		severity,
		discovery.CodeConfigLoadFailed,
		fmt.Sprintf("failed to load config, using defaults: %v", err),
		"",
		err,
	)
	if diagErr != nil {
		slog.Error("BUG: failed to create config-load diagnostic", "error", diagErr)
		return config.DefaultConfig(), nil
	}
	return config.DefaultConfig(), []Diagnostic{diag}
}
