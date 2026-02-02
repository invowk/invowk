// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type OutputFormat string

const (
	OutputFormatHuman OutputFormat = "human"
	OutputFormatJSON  OutputFormat = "json"
)

type Options struct {
	RepoRoot              string
	OutputPath            string
	OutputFormat          OutputFormat
	DocsSources           []string
	CanonicalExamplesPath string
	ExcludePkgAPIs        bool
	RootCmd               *cobra.Command
}

func (o *Options) Normalize() error {
	if o.RepoRoot == "" {
		repoRoot, err := FindRepoRoot("")
		if err != nil {
			return err
		}
		o.RepoRoot = repoRoot
	}
	if o.RepoRoot != "" {
		absRoot, err := filepath.Abs(o.RepoRoot)
		if err != nil {
			return err
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
