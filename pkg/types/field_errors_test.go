// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestFormatFieldErrors(t *testing.T) {
	t.Parallel()

	t.Run("zero errors", func(t *testing.T) {
		t.Parallel()
		got := FormatFieldErrors("request", nil)
		want := "invalid request: 0 field error(s)"
		if got != want {
			t.Errorf("FormatFieldErrors() = %q, want %q", got, want)
		}
	})

	t.Run("single error", func(t *testing.T) {
		t.Parallel()
		errs := []error{errors.New("bad field")}
		got := FormatFieldErrors("result", errs)
		want := "invalid result: 1 field error(s)"
		if got != want {
			t.Errorf("FormatFieldErrors() = %q, want %q", got, want)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		t.Parallel()
		errs := []error{errors.New("a"), errors.New("b"), errors.New("c")}
		got := FormatFieldErrors("execution context", errs)
		want := "invalid execution context: 3 field error(s)"
		if got != want {
			t.Errorf("FormatFieldErrors() = %q, want %q", got, want)
		}
	})
}
