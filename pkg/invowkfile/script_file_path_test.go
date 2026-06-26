// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestScriptFilePathValidateRejectsNullByte(t *testing.T) {
	t.Parallel()

	err := ScriptFilePath("scripts/\x00build.sh").Validate()
	if !errors.Is(err, ErrInvalidScriptFilePath) {
		t.Fatalf("ScriptFilePath.Validate() error = %v, want ErrInvalidScriptFilePath", err)
	}
	if !strings.Contains(err.Error(), "must not contain null bytes") {
		t.Fatalf("ScriptFilePath.Validate() error = %q, want null-byte detail", err.Error())
	}
	if !strings.Contains(err.Error(), `scripts/\x00build.sh`) {
		t.Fatalf("ScriptFilePath.Validate() error = %q, want escaped path value", err.Error())
	}
}

func TestInvalidScriptFilePathErrorFormatting(t *testing.T) {
	t.Parallel()

	var nilErr *InvalidScriptFilePathError
	if got, want := nilErr.Error(), "invalid script file path \"\": "; got != want {
		t.Fatalf("nil InvalidScriptFilePathError.Error() = %q, want %q", got, want)
	}

	noValueErr := &InvalidScriptFilePathError{Reason: "must be relative to the module root"}
	if got, want := noValueErr.Error(), "invalid script file path \"\": must be relative to the module root"; got != want {
		t.Fatalf("nil-value InvalidScriptFilePathError.Error() = %q, want %q", got, want)
	}

	value := ScriptFilePath("scripts/build.sh")
	err := &InvalidScriptFilePathError{Value: &value, Reason: "must not contain parent-directory segments"}
	if got, want := err.Error(), "invalid script file path \"scripts/build.sh\": must not contain parent-directory segments"; got != want {
		t.Fatalf("InvalidScriptFilePathError.Error() = %q, want %q", got, want)
	}
}
