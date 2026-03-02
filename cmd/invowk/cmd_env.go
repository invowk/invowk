// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"strings"
)

// captureUserEnv captures the current environment as a map.
// This should be called at the start of execution to capture the user's
// actual environment before invowk sets any command-level env vars.
func captureUserEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if key, value, found := strings.Cut(e, "="); found {
			env[key] = value
		}
	}
	return env
}
