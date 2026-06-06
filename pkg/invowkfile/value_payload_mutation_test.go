// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestValueErrorPayloadMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("dotenv file path preserves invalid value", func(t *testing.T) {
		t.Parallel()
		value := DotenvFilePath(" \t ")
		valueErr := requireValuePayloadMutationAs[*InvalidDotenvFilePathError](t, value.Validate())
		if !errors.Is(valueErr, ErrInvalidDotenvFilePath) {
			t.Fatalf("Validate() error = %v, want ErrInvalidDotenvFilePath", valueErr)
		}
		if valueErr.Value != value {
			t.Fatalf("InvalidDotenvFilePathError.Value = %q, want %q", valueErr.Value, value)
		}
	})

	t.Run("env config preserves field errors", func(t *testing.T) {
		t.Parallel()
		valueErr := requireValuePayloadMutationAs[*InvalidEnvConfigError](t, EnvConfig{
			Files: []DotenvFilePath{" \t "},
			Vars:  map[EnvVarName]string{"1BAD": "x"},
		}.Validate())
		if !errors.Is(valueErr, ErrInvalidEnvConfig) {
			t.Fatalf("Validate() error = %v, want ErrInvalidEnvConfig", valueErr)
		}
		requireValuePayloadFieldErrors(t, valueErr.FieldErrors, ErrInvalidDotenvFilePath, ErrInvalidEnvVarName)
	})

	t.Run("port mapping preserves invalid value and reason", func(t *testing.T) {
		t.Parallel()
		value := PortMappingSpec("0:80")
		valueErr := requireValuePayloadMutationAs[*InvalidPortMappingSpecError](t, value.Validate())
		if !errors.Is(valueErr, ErrInvalidPortMappingSpec) {
			t.Fatalf("Validate() error = %v, want ErrInvalidPortMappingSpec", valueErr)
		}
		if valueErr.Value != value {
			t.Fatalf("InvalidPortMappingSpecError.Value = %q, want %q", valueErr.Value, value)
		}
		if valueErr.Reason == "" {
			t.Fatal("InvalidPortMappingSpecError.Reason is empty, want validation detail")
		}
	})

	t.Run("volume mount preserves invalid value and reason", func(t *testing.T) {
		t.Parallel()
		value := VolumeMountSpec("/just-a-path")
		valueErr := requireValuePayloadMutationAs[*InvalidVolumeMountSpecError](t, value.Validate())
		if !errors.Is(valueErr, ErrInvalidVolumeMountSpec) {
			t.Fatalf("Validate() error = %v, want ErrInvalidVolumeMountSpec", valueErr)
		}
		if valueErr.Value != value {
			t.Fatalf("InvalidVolumeMountSpecError.Value = %q, want %q", valueErr.Value, value)
		}
		if valueErr.Reason == "" {
			t.Fatal("InvalidVolumeMountSpecError.Reason is empty, want validation detail")
		}
	})

	t.Run("workdir preserves invalid value", func(t *testing.T) {
		t.Parallel()
		value := WorkDir(" \t ")
		valueErr := requireValuePayloadMutationAs[*InvalidWorkDirError](t, value.Validate())
		if !errors.Is(valueErr, ErrInvalidWorkDir) {
			t.Fatalf("Validate() error = %v, want ErrInvalidWorkDir", valueErr)
		}
		if valueErr.Value != value {
			t.Fatalf("InvalidWorkDirError.Value = %q, want %q", valueErr.Value, value)
		}
	})
}

func requireValuePayloadMutationAs[T error](t *testing.T, err error) T {
	t.Helper()

	got, ok := errors.AsType[T](err)
	if !ok {
		var zero T
		t.Fatalf("error = %v, want %T", err, zero)
	}
	return got
}

func requireValuePayloadFieldErrors(t *testing.T, errs []error, sentinels ...error) {
	t.Helper()

	if len(errs) != len(sentinels) {
		t.Fatalf("FieldErrors count = %d, want %d: %v", len(errs), len(sentinels), errs)
	}
	for _, sentinel := range sentinels {
		if !valuePayloadFieldErrorsContain(errs, sentinel) {
			t.Fatalf("FieldErrors = %v, want sentinel %v", errs, sentinel)
		}
	}
}

func valuePayloadFieldErrorsContain(errs []error, sentinel error) bool {
	for _, err := range errs {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
