// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/deps"
)

func TestRenderDependencyError(t *testing.T) {
	t.Parallel()

	err := &deps.DependencyError{
		CommandName: "complex-deploy",
		MissingTools: []deps.DependencyMessage{
			"  - kubectl - not found in PATH",
		},
		MissingCommands: []deps.DependencyMessage{
			"  - build - command not found",
		},
		MissingFilepaths: []deps.DependencyMessage{
			"  - config.yaml - file not found",
		},
		MissingCapabilities: []deps.DependencyMessage{
			"  - capability \"internet\" not available: no connection",
		},
		MissingEnvVars: []deps.DependencyMessage{
			"  - AWS_ACCESS_KEY_ID - not set in environment",
		},
		FailedCustomChecks: []deps.DependencyMessage{
			"  - docker-version: exit code 127 (expected 0)",
		},
	}

	output := RenderDependencyError(err)

	for _, want := range []string{
		"Dependencies not satisfied",
		"'complex-deploy'",
		"Missing Tools",
		"Missing Commands",
		"Missing or Inaccessible Files",
		"Missing Capabilities",
		"Missing or Invalid Environment Variables",
		"Failed Custom Checks",
		"kubectl",
		"AWS_ACCESS_KEY_ID",
		"docker-version",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("RenderDependencyError() missing %q in:\n%s", want, output)
		}
	}
}

func TestRenderHostNotSupportedError(t *testing.T) {
	t.Parallel()

	output := RenderHostNotSupportedError("clean", "windows", "linux, mac")

	for _, want := range []string{"Host not supported", "'clean'", "windows", "linux, mac"} {
		if !strings.Contains(output, want) {
			t.Fatalf("RenderHostNotSupportedError() missing %q in:\n%s", want, output)
		}
	}
}
