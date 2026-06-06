// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRuntimeConfigWrapsFirstFieldErrorMutation(t *testing.T) {
	t.Parallel()

	err := validateRuntimeConfig(&RuntimeConfig{
		Name:           RuntimeNative,
		EnvInheritMode: "bogus",
	}, "build", 2)
	if !errors.Is(err, ErrInvalidEnvInheritMode) {
		t.Fatalf("validateRuntimeConfig() error = %v, want ErrInvalidEnvInheritMode", err)
	}
	if !strings.Contains(err.Error(), "command 'build' implementation #2") {
		t.Fatalf("validateRuntimeConfig() error = %q, want command implementation context", err)
	}
}
