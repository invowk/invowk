// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Output format options.
const (
	OutputFormatHuman OutputFormat = "human"
	OutputFormatJSON  OutputFormat = "json"
)

type (
	// OutputFormat defines the output format for summaries.
	OutputFormat string

	// Options configures the documentation audit execution.
	Options struct {
		RepoRoot              string
		OutputPath            string
		OutputFormat          OutputFormat
		DocsSources           []string
		CanonicalExamplesPath string
		ExcludePkgAPIs        bool
		RootCmd               *cobra.Command
	}
)

// Normalize fills defaults and validates the options.
func (o *Options) Normalize() error {
	if o.RepoRoot == "" {
		repoRoot, rootErr := findRepoRoot("")
		if rootErr != nil {
			return rootErr
		}
		o.RepoRoot = repoRoot
	}
	if o.RepoRoot != "" {
		absRoot, absErr := filepath.Abs(o.RepoRoot)
		if absErr != nil {
			return fmt.Errorf("resolve repo root: %w", absErr)
		}
		o.RepoRoot = absRoot
	}
	if o.OutputFormat == "" {
		o.OutputFormat = OutputFormatHuman
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join(o.RepoRoot, "docs-audit.md")
	}
	if !filepath.IsAbs(o.OutputPath) {
		o.OutputPath = filepath.Join(o.RepoRoot, o.OutputPath)
	}
	if o.CanonicalExamplesPath == "" {
		o.CanonicalExamplesPath = filepath.Join(o.RepoRoot, "examples")
	}
	if !filepath.IsAbs(o.CanonicalExamplesPath) {
		o.CanonicalExamplesPath = filepath.Join(o.RepoRoot, o.CanonicalExamplesPath)
	}
	if !o.ExcludePkgAPIs {
		o.ExcludePkgAPIs = true
	}
	return o.Validate()
}

// Validate checks required fields and values for options.
func (o Options) Validate() error {
	if o.RepoRoot == "" {
		return fmt.Errorf("repo root is required")
	}
	info, err := os.Stat(o.RepoRoot)
	if err != nil {
		return fmt.Errorf("repo root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo root is not a directory: %s", o.RepoRoot)
	}
	if o.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	switch o.OutputFormat {
	case OutputFormatHuman, OutputFormatJSON:
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", o.OutputFormat)
	}
}
