// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"fmt"
	"invowk-cli/internal/vhsnorm"
	"os"

	"github.com/spf13/cobra"
)

// internalNormalizeCmd normalizes VHS test output for deterministic comparison.
// This is an internal command used by the VHS test infrastructure.
var internalNormalizeCmd = &cobra.Command{
	Use:    "normalize <input-file>",
	Short:  "Normalize VHS test output (internal use only)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE:   runInternalNormalize,
}

func init() {
	internalNormalizeCmd.Flags().StringP("config", "c", "", "path to normalize.cue config file")
	internalNormalizeCmd.Flags().StringP("output", "o", "", "output file (default: stdout)")

	internalVHSCmd.AddCommand(internalNormalizeCmd)
}

// runInternalNormalize executes the normalization.
func runInternalNormalize(cmd *cobra.Command, args []string) (err error) {
	inputFile := args[0]
	configPath, _ := cmd.Flags().GetString("config")
	outputPath, _ := cmd.Flags().GetString("output")

	// Load configuration
	var cfg *vhsnorm.Config
	if configPath != "" {
		cfg, err = vhsnorm.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg = vhsnorm.DefaultConfig()
	}

	// Create normalizer
	normalizer, err := vhsnorm.NewNormalizer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create normalizer: %w", err)
	}

	// Open input file
	input, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Determine output destination
	var output *os.File
	if outputPath != "" {
		output, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			if closeErr := output.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()
	} else {
		output = os.Stdout
	}

	// Normalize
	if err := normalizer.Normalize(input, output); err != nil {
		return fmt.Errorf("normalization failed: %w", err)
	}

	return nil
}
